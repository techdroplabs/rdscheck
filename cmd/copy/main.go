package main

import (
	"os"

	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/s3"
	log "github.com/sirupsen/logrus"
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
	sourceRDS := rds.New(common.AWSSessions(config.AWSRegion))
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
	for _, instance := range doc.Instances {
		snapshots, err := dbinstance.GetSnapshots(sourceRDS, instance.Name)
		if err != nil {
			log.WithFields(log.Fields{
				"RDS Instance": instance.Name,
				"AWS Region":   config.AWSRegion,
			}).Error("Could not get snapshots")
			return err
		}

		destinationRDS := rds.New(common.AWSSessions(config.DestinationRegion))
		for _, s := range snapshots {
			err := dbinstance.CopySnapshots(destinationRDS, s)
			if err != nil {
				log.WithFields(log.Fields{
					"Snapshot": *s.DBSnapshotIdentifier,
				}).Errorf("Could not copy snapshot: %s", err)
			}
		}

		snapshots, err = dbinstance.GetSnapshots(destinationRDS, instance.Name)
		if err != nil {
			log.WithFields(log.Fields{
				"RDS Instance": instance.Name,
				"AWS Region":   config.DestinationRegion,
			}).Errorf("Could not get snapshots: %s", err)
			return err
		}

		oldSnapshots, err := dbinstance.GetOldSnapshots(destinationRDS, snapshots)
		if err != nil {
			log.WithError(err).Error("Could not get old snapshots")
			return err
		}

		err = dbinstance.DeleteOldSnapshots(destinationRDS, oldSnapshots)
		if err != nil {
			log.WithError(err).Error("Could not delete old snapshots")
			return err
		}
	}
	return nil
}
