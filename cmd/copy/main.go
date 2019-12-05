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

	doc, err := getDoc(source)
	if err != nil {
		log.WithError(err).Error("getDoc returned:")
		os.Exit(1)
	}

	err = copy(source, destination, doc)
	if err != nil {
		log.WithError(err).Error("copy returned:")
		os.Exit(1)
	}

	err = clean(destination, doc)
	if err != nil {
		log.WithError(err).Error("clean returned:")
		os.Exit(1)
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

func copy(source checks.DefaultChecks, destination checks.DefaultChecks, doc checks.Doc) error {
	source.SetSessions(config.AWSRegionSource)

	for _, instance := range doc.Instances {
		destination.SetSessions(instance.Destination)

		snapshots, err := source.GetSnapshots(instance.Name)
		if err != nil {
			log.WithFields(log.Fields{
				"RDS Instance": instance.Name,
				"AWS Region":   config.AWSRegionSource,
			}).WithError(err).Error("Could not get snapshots")
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
	}
	return nil
}

func clean(destination checks.DefaultChecks, doc checks.Doc) error {
	for _, instance := range doc.Instances {
		destination.SetSessions(instance.Destination)

		snapshots, err := destination.GetSnapshots(instance.Name)
		if err != nil {
			log.WithFields(log.Fields{
				"RDS Instance": instance.Name,
				"AWS Region":   instance.Destination,
			}).WithError(err).Error("Could not get snapshots")
			return err
		}

		oldSnapshots, err := destination.GetOldSnapshots(snapshots, instance.Retention)
		if err != nil {
			log.WithError(err).Error("Could not get old snapshots")
			return err
		}

		for _, snapshot := range oldSnapshots {
			if destination.CheckTag(*snapshot.DBSnapshotArn, "CreatedBy", "rdscheck") {

				err := destination.PostDatadogChecks(snapshot, "rdscheck.status", "ok", "copy")
				if err != nil {
					log.WithError(err).Error("Could not update datadog status")
					return err
				}

				err = destination.DeleteOldSnapshot(snapshot)
				if err != nil {
					log.WithError(err).Error("Could not delete old snapshots")

					err := destination.PostDatadogChecks(snapshot, "rdscheck.status", "critical", "copy")
					if err != nil {
						log.WithError(err).Error("Could not update datadog status")
						return err
					}
					return err
				}
			}
		}
	}
	return nil
}
