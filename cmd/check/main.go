package main

import (
	"os"

	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/s3"
	log "github.com/sirupsen/logrus"
	"github.com/techdroplabs/rdscheck/checks"
	"github.com/techdroplabs/rdscheck/common"
	"github.com/techdroplabs/rdscheck/config"
	"github.com/techdroplabs/rdscheck/dbinstance"
)

func main() {
	instance := dbinstance.NewDBInstance()
	err := run(instance)
	if err != nil {
		log.WithError(err).Error("Run returned:")
		os.Exit(1)
	}
}

func run(i *common.DBInstance) error {
	dd := common.DataDogSession(config.DDApiKey, config.DDAplicationKey)
	destinationRDS := rds.New(common.AWSSessions(config.SnapshotDestinationRegion))
	s3Session := s3.New(common.AWSSessions(config.SnapshotDestinationRegion))

	yaml, err := common.GetYamlFileFromS3(s3Session, config.S3Bucket, config.S3Key)
	if err != nil {
		log.WithError(err).Error("Could not get the yaml file from s3")
		return err
	}

	doc, err := common.UnmarshalYamlFile(yaml)
	if err != nil {
		log.WithError(err).Error("Could not unmarshal yaml file")
		return err
	}

	for _, doc := range doc.Instances {
		snapshots, err := dbinstance.GetSnapshots(destinationRDS, doc.Name)
		if err != nil {
			log.WithFields(log.Fields{
				"RDS Instance": doc.Name,
			}).Errorf("Could not get snapshots: %s", err)
			return err
		}

		for _, snapshot := range snapshots {
			if !dbinstance.CheckTag(destinationRDS, *snapshot.DBSnapshotArn, "Status", "tested") {
				if dbinstance.CheckTag(destinationRDS, *snapshot.DBSnapshotArn, "CreatedBy", "rdscheck") {
					if !dbinstance.CheckIfDatabaseSubnetGroupExist(destinationRDS, snapshot) {
						err := dbinstance.CreateDatabaseSubnetGroup(dd, destinationRDS, snapshot, config.SubnetIds)
						if err != nil {
							log.WithFields(log.Fields{
								"RDS Instance": *snapshot.DBInstanceIdentifier,
							}).Errorf("Could not create Database Subnet Group: %s", err)
							return err
						}
					}

					if !dbinstance.CheckIfRDSInstanceExist(destinationRDS, snapshot) {
						if !dbinstance.CheckTag(destinationRDS, *snapshot.DBSnapshotArn, "Status", "testing") {
							err = dbinstance.CreateDBFromSnapshot(dd, destinationRDS, snapshot, doc.Database, config.SecurityGroupIds)
							if err != nil {
								log.WithFields(log.Fields{
									"Snapshot":     *snapshot.DBSnapshotIdentifier,
									"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
								}).Errorf("Could not create rds instance from snapshot: %s", err)
								return err
							}
						}
					}

					dbInfo, err := dbinstance.GetDBInstanceInfo(dd, destinationRDS, snapshot)
					if err != nil {
						log.WithFields(log.Fields{
							"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
						}).Info("Could not get RDS instance Info")
						return err
					}

					if dbinstance.CheckTag(destinationRDS, *dbInfo.DBInstanceArn, "Status", "ready") {
						if dbinstance.GetDBInstanceStatus(destinationRDS, snapshot) == "available" {
							err = dbinstance.ChangeDBpassword(dd, destinationRDS, snapshot, *dbInfo.DBInstanceArn, doc.Password)
							if err != nil {
								log.WithError(err).Error("Could not update db password")
								return err
							}
						}
					}

					if dbinstance.CheckTag(destinationRDS, *dbInfo.DBInstanceArn, "Status", "testing") {
						if dbinstance.GetDBInstanceStatus(destinationRDS, snapshot) == "available" {
							if doc.Name == *dbInfo.DBName {
								for _, query := range doc.Queries {
									if checks.CheckSQLQueries(dd, snapshot, *dbInfo.Endpoint, *dbInfo.MasterUsername, doc.Password, *dbInfo.DBName, query.Query, query.Regex) {
										common.PostDatadogChecks(dd, "rdscheck.status", "ok", snapshot)
									} else {
										log.WithFields(log.Fields{
											"RDS Instance": string(*snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier),
											"DB Name":      *dbInfo.DBName,
											"Query":        query.Query,
											"Regex":        query.Regex,
										}).Errorf("Query matched failed: %s", err)
										common.PostDatadogChecks(dd, "rdscheck.status", "critical", snapshot)
										return err
									}
								}
							}
							err = dbinstance.UpdateStatusTag(dd, snapshot, destinationRDS, *dbInfo.DBInstanceArn, "tested")
							if err != nil {
								log.WithError(err).Error("Could not update snapshot status")
								return err
							}
						}
					}

					if dbinstance.CheckTag(destinationRDS, *dbInfo.DBInstanceArn, "Status", "tested") {
						if dbinstance.CheckIfRDSInstanceExist(destinationRDS, snapshot) {
							if dbinstance.GetDBInstanceStatus(destinationRDS, snapshot) == "available" {
								err = dbinstance.DeleteDB(dd, destinationRDS, snapshot)
								if err != nil {
									log.WithFields(log.Fields{
										"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
									}).Errorf("Could not delete the rds instance: %s", err)
									return err
								}
							}
						}
					}
				} else {
					log.WithFields(log.Fields{
						"Snapshot": *snapshot.DBSnapshotIdentifier,
					}).Info("Snapshot not created by rdscheck")
				}
			} else {
				if !dbinstance.CheckIfRDSInstanceExist(destinationRDS, snapshot) {
					if dbinstance.CheckIfDatabaseSubnetGroupExist(destinationRDS, snapshot) {
						err := dbinstance.DeleteDatabaseSubnetGroup(dd, destinationRDS, snapshot)
						if err != nil {
							log.WithFields(log.Fields{
								"RDS Instance": *snapshot.DBInstanceIdentifier,
							}).Errorf("Could not create Database Subnet Group: %s", err)
							return err
						}
					}
				}
				log.WithFields(log.Fields{
					"Snapshot": *snapshot.DBSnapshotIdentifier,
				}).Info("Snapshot already tested")
			}
		}
	}
	return nil
}
