package main

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/techdroplabs/rdscheck/checks"
	"github.com/techdroplabs/rdscheck/config"
)

const (
	Ready   = "ready"
	Restore = "restore"
	Modify  = "modify"
	Verify  = "verify"
	Clean   = "clean"
	Tested  = "tested"
	Alarm   = "alarm"
)

func main() {
	checks := checks.New()

	doc, err := getDoc(checks)
	if err != nil {
		log.WithError(err).Error("Could not get the doc")
		os.Exit(1)
	}

	err = validate(checks, doc)
	if err != nil {
		log.WithError(err).Error("Could not validate the snapshots")
	}
}

func getDoc(client checks.DefaultChecks) (checks.Doc, error) {

	client.SetSessions(config.DestinationRegion)

	doc := checks.Doc{}

	yaml, err := client.GetYamlFileFromS3(config.S3Bucket, config.S3Key)
	if err != nil {
		log.WithError(err).Error("Could not get the yaml file from s3")
		return doc, err
	}

	doc, err = client.UnmarshalYamlFile(yaml)
	if err != nil {
		log.WithError(err).Error("Could not unmarshal yaml file")
		return doc, err
	}
	return doc, nil
}

func validate(client checks.DefaultChecks, doc checks.Doc) error {
	for _, instance := range doc.Instances {
		snapshots, err := client.GetSnapshots(instance.Name)
		if err != nil {
			log.WithFields(log.Fields{
				"RDS Instance": instance.Name,
			}).Errorf("Could not get snapshots: %s", err)
			return err
		}
		for _, snapshot := range snapshots {
			if client.CheckTag(*snapshot.DBSnapshotArn, "CreatedBy", "rdscheck") {
				status := client.GetTagValue(*snapshot.DBSnapshotArn, "Status")
				switch status {
				case Ready:
					err := client.PostDatadogChecks(snapshot, "rdscheck.status", "ok")
					if err != nil {
						log.WithError(err).Error("Could not update datadog status")
					}

					err = client.CreateDatabaseSubnetGroup(snapshot, config.SubnetIds)
					if err != nil {
						log.WithFields(log.Fields{
							"RDS Instance": *snapshot.DBInstanceIdentifier,
						}).Errorf("Could not create Database Subnet Group: %s", err)
						err = client.UpdateTag(snapshot, "Status", "alarm")
						if err != nil {
							log.Error(err)
						}
					}

					err = client.UpdateTag(snapshot, "Status", "restore")
					if err != nil {
						log.Error(err)
					}

				case Restore:
					err := client.CreateDBFromSnapshot(snapshot, instance.Database, instance.Type, config.SecurityGroupIds)
					if err != nil {
						log.WithFields(log.Fields{
							"Snapshot":     *snapshot.DBSnapshotIdentifier,
							"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
						}).Errorf("Could not create rds instance from snapshot: %s", err)
						errors := client.UpdateTag(snapshot, "Status", "alarm")
						if errors != nil {
							log.Error(errors)
						}
						return err
					}

					err = client.UpdateTag(snapshot, "Status", "modify")
					if err != nil {
						log.Error(err)
					}

				case Modify:
					if client.GetDBInstanceStatus(snapshot) != "available" {
						break
					}

					dbInfo, err := client.GetDBInstanceInfo(snapshot)
					if err != nil {
						log.WithFields(log.Fields{
							"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
						}).Info("Could not get RDS instance Info")
						errors := client.UpdateTag(snapshot, "Status", "alarm")
						if errors != nil {
							log.Error(errors)
						}
						return err
					}

					err = client.ChangeDBpassword(snapshot, *dbInfo.DBInstanceArn, instance.Password)
					if err != nil {
						log.WithFields(log.Fields{
							"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
						}).Info("Could not update db password")
						errors := client.UpdateTag(snapshot, "Status", "alarm")
						if errors != nil {
							log.Error(errors)
						}
						return err
					}

					err = client.UpdateTag(snapshot, "Status", "verify")
					if err != nil {
						log.Error(err)
					}

				case Verify:
					if client.GetDBInstanceStatus(snapshot) != "available" {
						break
					}

					dbInfo, err := client.GetDBInstanceInfo(snapshot)
					if err != nil {
						log.WithFields(log.Fields{
							"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
						}).Info("Could not get RDS instance Info")
						errors := client.UpdateTag(snapshot, "Status", "alarm")
						if errors != nil {
							log.Error(errors)
						}
						return err
					}

					if instance.Name == *dbInfo.DBName {
						for _, query := range instance.Queries {
							client.InitDb(*dbInfo.Endpoint, *dbInfo.MasterUsername, instance.Password, *dbInfo.DBName)

							if client.CheckSQLQueries(query.Query, query.Regex) {
								err := client.UpdateTag(snapshot, "Status", "clean")
								if err != nil {
									log.Error(err)
								}
							} else {
								log.WithFields(log.Fields{
									"RDS Instance": string(*snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier),
									"DB Name":      *dbInfo.DBName,
									"Query":        query.Query,
									"Regex":        query.Regex,
								}).Errorf("Query matched failed: %s", err)
								errors := client.UpdateTag(snapshot, "Status", "alarm")
								if errors != nil {
									log.Error(errors)
								}
								return err
							}
						}
					}

				case Alarm:
					err := client.PostDatadogChecks(snapshot, "rdscheck.status", "critical")
					if err != nil {
						log.WithError(err).Error("Could not update datadog status")
					}

					err = client.UpdateTag(snapshot, "ChecksFailed", "yes")
					if err != nil {
						log.Error(err)
					}

					err = client.UpdateTag(snapshot, "Status", "clean")
					if err != nil {
						log.Error(err)
					}

				case Clean:
					err := client.DeleteDB(snapshot)
					if err != nil {
						log.WithFields(log.Fields{
							"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
						}).Errorf("Could not delete the rds instance: %s", err)
						return err
					}

					err = client.UpdateTag(snapshot, "Status", "tested")
					if err != nil {
						log.Error(err)
					}

				case Tested:
					if client.GetDBInstanceStatus(snapshot) != "" {
						break
					}

					if !client.CheckIfDatabaseSubnetGroupExist(snapshot) {
						break
					}

					err := client.DeleteDatabaseSubnetGroup(snapshot)
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
