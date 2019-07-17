package config

import (
	"strings"

	"github.com/techdroplabs/rdscheck/utils"
)

var (
	S3Bucket          = utils.GetEnvString("S3_BUCKET", "")
	S3Key             = utils.GetEnvString("S3_KEY", "")
	AWSRegion         = utils.GetEnvString("AWS_REGION", "us-west-2")
	DestinationRegion = utils.GetEnvString("AWS_REGION_DESTINATION", AWSRegion)
	SnapshotRetention = utils.GetEnvInt("SNAPSHOT_RETENTION", 1)
	SecurityGroupIds  = strings.Split(utils.GetEnvString("AWS_SG_IDS", ""), ",")
	SubnetIds         = strings.Split(utils.GetEnvString("AWS_SUBNETS_IDS", ""), ",")
	DDApiKey          = utils.GetEnvString("DD_API_KEY", "")
	DDAplicationKey   = utils.GetEnvString("DD_APP_KEY", "")
)
