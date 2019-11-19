package main

import (
	"os"

	"github.com/aws/aws-lambda-go/lambda"
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
	lambda.Start(run)
}

func run() {
	source := checks.New()
	destination := checks.New()

	doc, err := getDoc(source)
	if err != nil {
		log.WithError(err).Error("Could not get the doc")
		os.Exit(1)
	}

	err = validate(destination, doc)
	if err != nil {
		log.WithError(err).Error("Could not validate the snapshots")
	}
}

func getDoc(source checks.DefaultChecks) (checks.Doc, error) {
	source.SetSessions(config.AWSRegionSource)

	doc := checks.Doc{}

	yaml, err := source.GetYamlFileFromS3(config.S3Bucket, config.S3Key)
	if err != nil {
		log.WithError(err).Error("Could not get the yaml file from s3")
		return doc, err
	}

	doc, err = source.UnmarshalYamlFile(yaml)
	if err != nil {
		log.WithError(err).Error("Could not unmarshal yaml file")
		return doc, err
	}
	return doc, nil
}

func validate(destination checks.DefaultChecks, doc checks.Doc) error {
	for _, instance := range doc.Instances {
		destination.SetSessions(instance.Destination)
		snapshots, err := destination.GetSnapshots(instance.Name)
		if err != nil {
			log.WithFields(log.Fields{
				"RDS Instance": instance.Name,
			}).Errorf("Could not get snapshots: %s", err)
			return err
		}
		for _, snapshot := range snapshots {
			if destination.CheckTag(*snapshot.DBSnapshotArn, "CreatedBy", "rdscheck") {
				status := destination.GetTagValue(*snapshot.DBSnapshotArn, "Status")
				switch status {
				case Ready:
					err := destination.PostDatadogChecks(snapshot, "rdscheck.status", "ok")
					if err != nil {
						log.WithError(err).Error("Could not update datadog status")
					}

					err = destination.CreateDatabaseSubnetGroup(snapshot, config.SubnetIds)
					if err != nil {
						log.WithFields(log.Fields{
							"RDS Instance": *snapshot.DBInstanceIdentifier,
						}).Errorf("Could not create Database Subnet Group: %s", err)
						err = destination.UpdateTag(snapshot, "Status", "alarm")
						if err != nil {
							log.Error(err)
						}
					}

					err = destination.UpdateTag(snapshot, "Status", "restore")
					if err != nil {
						log.Error(err)
					}

				case Restore:
					err := destination.CreateDBFromSnapshot(snapshot, instance.Type, config.SecurityGroupIds)
					if err != nil {
						log.WithFields(log.Fields{
							"Snapshot":     *snapshot.DBSnapshotIdentifier,
							"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
						}).Errorf("Could not create rds instance from snapshot: %s", err)
						errors := destination.UpdateTag(snapshot, "Status", "alarm")
						if errors != nil {
							log.Error(errors)
						}
						return err
					}

					err = destination.UpdateTag(snapshot, "Status", "modify")
					if err != nil {
						log.Error(err)
					}

				case Modify:
					if destination.GetDBInstanceStatus(snapshot) != "available" {
						break
					}

					dbInfo, err := destination.GetDBInstanceInfo(snapshot)
					if err != nil {
						log.WithFields(log.Fields{
							"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
						}).Info("Could not get RDS instance Info")
						errors := destination.UpdateTag(snapshot, "Status", "alarm")
						if errors != nil {
							log.Error(errors)
						}
						return err
					}

					err = destination.ChangeDBpassword(snapshot, *dbInfo.DBInstanceArn, instance.Password)
					if err != nil {
						log.WithFields(log.Fields{
							"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
						}).Info("Could not update db password")
						errors := destination.UpdateTag(snapshot, "Status", "alarm")
						if errors != nil {
							log.Error(errors)
						}
						return err
					}

					err = destination.UpdateTag(snapshot, "Status", "verify")
					if err != nil {
						log.Error(err)
					}

				case Verify:
					if destination.GetDBInstanceStatus(snapshot) != "available" {
						break
					}

					dbInfo, err := destination.GetDBInstanceInfo(snapshot)
					if err != nil {
						log.WithFields(log.Fields{
							"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
						}).Info("Could not get RDS instance Info")
						errors := destination.UpdateTag(snapshot, "Status", "alarm")
						if errors != nil {
							log.Error(errors)
						}
						return err
					}

					destination.InitDb(*dbInfo.Endpoint, *dbInfo.MasterUsername, instance.Password, instance.Database)

					if instance.Name == *dbInfo.DBName {
						for _, query := range instance.Queries {
							if destination.CheckRegexAgainstRow(query.Query, query.Regex) {
								err := destination.UpdateTag(snapshot, "Status", "clean")
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
								errors := destination.UpdateTag(snapshot, "Status", "alarm")
								if errors != nil {
									log.Error(errors)
								}
								return err
							}
						}
					}

				case Alarm:
					err := destination.PostDatadogChecks(snapshot, "rdscheck.status", "critical")
					if err != nil {
						log.WithError(err).Error("Could not update datadog status")
					}

					err = destination.UpdateTag(snapshot, "ChecksFailed", "yes")
					if err != nil {
						log.Error(err)
					}

					err = destination.UpdateTag(snapshot, "Status", "clean")
					if err != nil {
						log.Error(err)
					}

				case Clean:
					err := destination.DeleteDB(snapshot)
					if err != nil {
						log.WithFields(log.Fields{
							"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
						}).Errorf("Could not delete the rds instance: %s", err)
						return err
					}

					err = destination.UpdateTag(snapshot, "Status", "tested")
					if err != nil {
						log.Error(err)
					}

				case Tested:
					if destination.GetDBInstanceStatus(snapshot) != "" {
						break
					}

					if !destination.CheckIfDatabaseSubnetGroupExist(snapshot) {
						break
					}

					err := destination.DeleteDatabaseSubnetGroup(snapshot)
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
