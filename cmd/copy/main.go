package main

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/techdroplabs/rdscheck/checks"
	"github.com/techdroplabs/rdscheck/config"
)

func main() {
	source := checks.New()
	destination := checks.New()

	err := run(source, destination)
	if err != nil {
		log.WithError(err).Error("Run returned:")
		os.Exit(1)
	}
}

func run(source checks.DefaultChecks, destination checks.DefaultChecks) error {
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
			err := destination.CopySnapshots(snapshot, instance.Destination)
			if err != nil {
				log.WithFields(log.Fields{
					"Snapshot": *snapshot.DBSnapshotIdentifier,
				}).Errorf("Could not copy snapshot: %s", err)
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
	return nil
}
