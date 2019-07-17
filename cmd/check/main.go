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
	datadog := common.DataDogSession(config.DDApiKey, config.DDAplicationKey)
	destinationRDS := rds.New(common.AWSSessions(config.DestinationRegion))
	s3Session := s3.New(common.AWSSessions(config.DestinationRegion))

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
			if dbinstance.CheckTag(destinationRDS, *snapshot.DBSnapshotArn, "CreatedBy", "rdscheck") {
				status := dbinstance.GetTagValue(destinationRDS, *snapshot.DBSnapshotArn, "Status")
				switch status {
				case "ready":
					err := common.PostDatadogChecks(datadog, "rdscheck.status", "ok", snapshot)
					if err != nil {
						log.WithError(err).Error("Could not update datadog status")
					}

					err = dbinstance.CreateDatabaseSubnetGroup(destinationRDS, snapshot, config.SubnetIds)
					if err != nil {
						log.WithFields(log.Fields{
							"RDS Instance": *snapshot.DBInstanceIdentifier,
						}).Errorf("Could not create Database Subnet Group: %s", err)
						dbinstance.UpdateStatusTag(snapshot, destinationRDS, "alarm")
						return err
					}

					dbinstance.UpdateStatusTag(snapshot, destinationRDS, "restore")

				case "restore":
					err = dbinstance.CreateDBFromSnapshot(destinationRDS, snapshot, doc.Database, config.SecurityGroupIds)
					if err != nil {
						log.WithFields(log.Fields{
							"Snapshot":     *snapshot.DBSnapshotIdentifier,
							"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
						}).Errorf("Could not create rds instance from snapshot: %s", err)
						dbinstance.UpdateStatusTag(snapshot, destinationRDS, "alarm")
						return err
					}

					dbinstance.UpdateStatusTag(snapshot, destinationRDS, "modify")

				case "modify":
					if dbinstance.GetDBInstanceStatus(destinationRDS, snapshot) != "available" {
						break
					}

					dbInfo, err := dbinstance.GetDBInstanceInfo(destinationRDS, snapshot)
					if err != nil {
						log.WithFields(log.Fields{
							"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
						}).Info("Could not get RDS instance Info")
						dbinstance.UpdateStatusTag(snapshot, destinationRDS, "alarm")
						return err
					}

					err = dbinstance.ChangeDBpassword(destinationRDS, snapshot, *dbInfo.DBInstanceArn, doc.Password)
					if err != nil {
						log.WithFields(log.Fields{
							"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
						}).Info("Could not update db password")
						dbinstance.UpdateStatusTag(snapshot, destinationRDS, "alarm")
						return err
					}

					dbinstance.UpdateStatusTag(snapshot, destinationRDS, "verify")

				case "verify":
					if dbinstance.GetDBInstanceStatus(destinationRDS, snapshot) != "available" {
						break
					}

					dbInfo, err := dbinstance.GetDBInstanceInfo(destinationRDS, snapshot)
					if err != nil {
						log.WithFields(log.Fields{
							"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
						}).Info("Could not get RDS instance Info")
						dbinstance.UpdateStatusTag(snapshot, destinationRDS, "alarm")
						return err
					}

					if doc.Name == *dbInfo.DBName {
						for _, query := range doc.Queries {
							if checks.CheckSQLQueries(destinationRDS, snapshot, *dbInfo.Endpoint, *dbInfo.MasterUsername, doc.Password, *dbInfo.DBName, query.Query, query.Regex) {
								dbinstance.UpdateStatusTag(snapshot, destinationRDS, "clean")
							} else {
								log.WithFields(log.Fields{
									"RDS Instance": string(*snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier),
									"DB Name":      *dbInfo.DBName,
									"Query":        query.Query,
									"Regex":        query.Regex,
								}).Errorf("Query matched failed: %s", err)
								dbinstance.UpdateStatusTag(snapshot, destinationRDS, "alarm")
								return err
							}
						}
					}

				case "alarm":
					err := common.PostDatadogChecks(datadog, "rdscheck.status", "critical", snapshot)
					if err != nil {
						log.WithError(err).Error("Could not update datadog status")
					}

					dbinstance.UpdateStatusTag(snapshot, destinationRDS, "clean")

				case "clean":
					err = dbinstance.DeleteDB(destinationRDS, snapshot)
					if err != nil {
						log.WithFields(log.Fields{
							"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
						}).Errorf("Could not delete the rds instance: %s", err)
						dbinstance.UpdateStatusTag(snapshot, destinationRDS, "alarm")
						return err
					}

					dbinstance.UpdateStatusTag(snapshot, destinationRDS, "tested")

				case "tested":
					if dbinstance.GetDBInstanceStatus(destinationRDS, snapshot) != "" {
						break
					}

					if !dbinstance.CheckIfDatabaseSubnetGroupExist(destinationRDS, snapshot) {
						break
					}

					err := dbinstance.DeleteDatabaseSubnetGroup(destinationRDS, snapshot)
					if err != nil {
						log.WithFields(log.Fields{
							"RDS Instance": *snapshot.DBInstanceIdentifier,
						}).Errorf("Could not delete database subnet group: %s", err)
						return err
					}
				}
			}
		}
	}
	return nil
}
