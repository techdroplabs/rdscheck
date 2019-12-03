package main

import (
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/service/rds"
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
			}).WithError(err).Error("Could not get snapshots")
			return err
		}
		for _, snapshot := range snapshots {
			if destination.CheckTag(*snapshot.DBSnapshotArn, "CreatedBy", "rdscheck") {
				status := destination.GetTagValue(*snapshot.DBSnapshotArn, "Status")
				err := process(destination, snapshot, &instance, status)
				if err != nil {
					log.WithFields(log.Fields{
						"RDS Instance": instance.Name,
						"Snapshot":     *snapshot.DBSnapshotIdentifier,
					}).WithError(err).Error("Could not get snapshots")
					return err
				}
			}
		}
	}
	return nil
}

func process(destination checks.DefaultChecks, snapshot *rds.DBSnapshot, instance *checks.Instances, status string) error {
	switch status {
	case Ready:
		return caseReady(destination, snapshot)
	case Restore:
		return caseRestore(destination, snapshot, instance)
	case Modify:
		return caseModify(destination, snapshot, instance)
	case Verify:
		return caseVerify(destination, snapshot, instance)
	case Alarm:
		return caseAlarm(destination, snapshot)
	case Clean:
		return caseClean(destination, snapshot)
	case Tested:
		return caseTested(destination, snapshot)
	default:
		return nil
	}
}

func caseReady(destination checks.DefaultChecks, snapshot *rds.DBSnapshot) error {
	err := destination.PostDatadogChecks(snapshot, "rdscheck.status", "ok", "check")
	if err != nil {
		log.WithError(err).Error("Could not update datadog status")
		return err
	}

	err = destination.CreateDatabaseSubnetGroup(snapshot, config.SubnetIds)
	if err != nil {
		log.WithFields(log.Fields{
			"RDS Instance": *snapshot.DBInstanceIdentifier,
		}).WithError(err).Error("Could not create Database Subnet Group")
		err = destination.UpdateTag(snapshot, "Status", "alarm")
		if err != nil {
			return err
		}
	}

	err = destination.UpdateTag(snapshot, "Status", "restore")
	if err != nil {
		return err
	}
	return nil
}

func caseRestore(destination checks.DefaultChecks, snapshot *rds.DBSnapshot, instance *checks.Instances) error {
	err := destination.CreateDBFromSnapshot(snapshot, instance.Type, config.SecurityGroupIds)
	if err != nil {
		log.WithFields(log.Fields{
			"Snapshot":     *snapshot.DBSnapshotIdentifier,
			"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
		}).WithError(err).Error("Could not create rds instance from snapshot")
		errors := destination.UpdateTag(snapshot, "Status", "alarm")
		if errors != nil {
			return err
		}
		return err
	}

	err = destination.UpdateTag(snapshot, "Status", "modify")
	if err != nil {
		return err
	}
	return nil
}

func caseModify(destination checks.DefaultChecks, snapshot *rds.DBSnapshot, instance *checks.Instances) error {
	if destination.GetDBInstanceStatus(snapshot) != "available" {
		return nil
	}

	dbInfo, err := destination.GetDBInstanceInfo(snapshot)
	if err != nil {
		log.WithFields(log.Fields{
			"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
		}).Info("Could not get RDS instance Info")
		errors := destination.UpdateTag(snapshot, "Status", "alarm")
		if errors != nil {
			return err
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
			return err
		}
		return err
	}

	err = destination.UpdateTag(snapshot, "Status", "verify")
	if err != nil {
		return err
	}
	return nil
}

func caseVerify(destination checks.DefaultChecks, snapshot *rds.DBSnapshot, instance *checks.Instances) error {
	if destination.GetDBInstanceStatus(snapshot) != "available" {
		return nil
	}

	dbInfo, err := destination.GetDBInstanceInfo(snapshot)
	if err != nil {
		log.WithFields(log.Fields{
			"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
		}).Info("Could not get RDS instance Info")
		errors := destination.UpdateTag(snapshot, "Status", "alarm")
		if errors != nil {
			return err
		}
		return err
	}

	err = destination.InitDb(dbInfo, instance.Password, instance.Database)
	if err != nil {
		errors := destination.UpdateTag(snapshot, "Status", "alarm")
		if errors != nil {
			return err
		}
		return err
	}

	for _, query := range instance.Queries {
		if destination.CheckRegexAgainstRow(query.Query, query.Regex) {
			err := destination.UpdateTag(snapshot, "Status", "clean")
			if err != nil {
				return err
			}
		} else {
			log.WithFields(log.Fields{
				"RDS Instance": string(*snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier),
				"DB Name":      *dbInfo.DBName,
				"Query":        query.Query,
				"Regex":        query.Regex,
			}).WithError(err).Error("Query matched failed")
			errors := destination.UpdateTag(snapshot, "Status", "alarm")
			if errors != nil {
				return err
			}
			return err
		}
	}
	return nil
}

func caseAlarm(destination checks.DefaultChecks, snapshot *rds.DBSnapshot) error {
	err := destination.PostDatadogChecks(snapshot, "rdscheck.status", "critical", "check")
	if err != nil {
		log.WithError(err).Error("Could not update datadog status")
		return err
	}

	err = destination.UpdateTag(snapshot, "ChecksFailed", "yes")
	if err != nil {
		return err
	}

	err = destination.UpdateTag(snapshot, "Status", "clean")
	if err != nil {
		return err
	}
	return nil
}

func caseClean(destination checks.DefaultChecks, snapshot *rds.DBSnapshot) error {
	err := destination.DeleteDB(snapshot)
	if err != nil {
		log.WithFields(log.Fields{
			"RDS Instance": *snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier,
		}).WithError(err).Error("Could not delete the rds instance")
		return err
	}

	err = destination.UpdateTag(snapshot, "Status", "tested")
	if err != nil {
		return err
	}
	return nil
}

func caseTested(destination checks.DefaultChecks, snapshot *rds.DBSnapshot) error {
	if destination.GetDBInstanceStatus(snapshot) != "" {
		return nil
	}

	if !destination.CheckIfDatabaseSubnetGroupExist(snapshot) {
		return nil
	}

	err := destination.DeleteDatabaseSubnetGroup(snapshot)
	if err != nil {
		log.WithFields(log.Fields{
			"RDS Instance": *snapshot.DBInstanceIdentifier,
		}).WithError(err).Error("Could not delete database subnet group")
		return err
	}
	return nil
}
