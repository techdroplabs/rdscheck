package main

import (
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	log "github.com/sirupsen/logrus"
	"github.com/techdroplabs/rdscheck/checks"
	"github.com/techdroplabs/rdscheck/config"
)

func main() {
	lambda.Start(run)
}

func run() {
	source := checks.New()
	destination := checks.New()

	err := copy(source, destination)
	if err != nil {
		log.WithError(err).Error("Run returned:")
		os.Exit(1)
	}
}

func copy(source checks.DefaultChecks, destination checks.DefaultChecks) error {
	source.SetSessions(config.AWSRegionSource)

	yaml, err := source.GetYamlFileFromS3(config.S3Bucket, config.S3Key)
	if err != nil {
		log.WithError(err).Error("Could not get the yaml file from s3")
		return err
	}

	doc, err := source.UnmarshalYamlFile(yaml)
	if err != nil {
		log.WithError(err).Error("Could not unmarshal yaml file")
		return err
	}

	for _, instance := range doc.Instances {
		destination.SetSessions(instance.Destination)
		snapshots, err := source.GetSnapshots(instance.Name)
		if err != nil {
			log.WithFields(log.Fields{
				"RDS Instance": instance.Name,
				"AWS Region":   config.AWSRegionSource,
			}).Errorf("Could not get snapshots: %s", err)
			return err
		}
		for _, snapshot := range snapshots {
			if *snapshot.SnapshotType == "automated" {
				err := destination.PostDatadogChecks(snapshot, "rdscheck.status", "ok", "copy")
				if err != nil {
					log.WithError(err).Error("Could not update datadog status")
					return err
				}
				var preSignedUrl string
				cleanArn := destination.CleanArn(snapshot)
				if *snapshot.Encrypted {
					preSignedUrl, err = source.PreSignUrl(instance.Destination, *snapshot.DBSnapshotArn, instance.KmsID, cleanArn)
					if err != nil {
						log.WithFields(log.Fields{
							"snapshot": *snapshot.DBSnapshotIdentifier,
						}).WithError(err).Error("Could not presigned the url")
						err := destination.PostDatadogChecks(snapshot, "rdscheck.status", "critical", "copy")
						if err != nil {
							log.WithError(err).Error("Could not update datadog status")
							return err
						}
						return err
					}
				}
				err = destination.CopySnapshots(snapshot, instance.Destination, instance.KmsID, preSignedUrl, cleanArn)
				if err != nil {
					log.WithFields(log.Fields{
						"Snapshot": *snapshot.DBSnapshotIdentifier,
					}).WithError(err).Error("Could not copy snapshot")
					err := destination.PostDatadogChecks(snapshot, "rdscheck.status", "critical", "copy")
					if err != nil {
						log.WithError(err).Error("Could not update datadog status")
						return err
					}
					return err
				}
			}
		}

		snapshots, err = destination.GetSnapshots(instance.Name)
		if err != nil {
			log.WithFields(log.Fields{
				"RDS Instance": instance.Name,
				"AWS Region":   instance.Destination,
			}).Errorf("Could not get snapshots: %s", err)
			return err
		}
		for _, snapshot := range snapshots {
			if destination.CheckTag(*snapshot.DBSnapshotArn, "CreatedBy", "rdscheck") {
				oldSnapshots, err := destination.GetOldSnapshots(snapshots, instance.Retention)
				if err != nil {
					log.WithError(err).Error("Could not get old snapshots")
					return err
				}

				err = destination.DeleteOldSnapshots(oldSnapshots)
				if err != nil {
					log.WithError(err).Error("Could not delete old snapshots")
					return err
				}
			}
		}
	}
	return nil
}
