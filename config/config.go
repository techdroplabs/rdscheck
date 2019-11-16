package config

import (
	"strings"

	"github.com/techdroplabs/rdscheck/utils"
)

var (
	// We dont define the var bellow from the yaml configuration file
	// because we assume it will be populated via terraform when we create the lambda function
	S3Bucket         = utils.GetEnvString("S3_BUCKET", "")
	S3Key            = utils.GetEnvString("S3_KEY", "")
	AWSRegionSource  = utils.GetEnvString("AWS_REGION_SOURCE", "us-west-2")
	SecurityGroupIds = strings.Split(utils.GetEnvString("AWS_SG_IDS", ""), ",")
	SubnetIds        = strings.Split(utils.GetEnvString("AWS_SUBNETS_IDS", ""), ",")
	DDApiKey         = utils.GetEnvString("DD_API_KEY", "")
	DDAplicationKey  = utils.GetEnvString("DD_APP_KEY", "")
)
