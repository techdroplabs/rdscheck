package checks

import (
	"errors"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
	"github.com/techdroplabs/rdscheck/config"
)

// GetSnapshots gets the latest snapshots of a RDS instance
func (c *Client) GetSnapshots(DBInstanceIdentifier string) ([]*rds.DBSnapshot, error) {
	input := &rds.DescribeDBSnapshotsInput{
		DBInstanceIdentifier: aws.String(DBInstanceIdentifier),
	}

	r, err := c.RDS.DescribeDBSnapshots(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			return nil, aerr
		}
	}

	sorted := r.DBSnapshots[:0]
	for _, snapshot := range r.DBSnapshots {
		if *snapshot.Status == "available" {
			sorted = append(sorted, snapshot)
		}
	}

	sort.Slice(sorted, func(i, j int) bool {
		return (*sorted[i].SnapshotCreateTime).Before(*sorted[j].SnapshotCreateTime)
	})

	return sorted, nil
}

// CopySnapshots copies the snapshots either to the same region as the original
// or to a new region
func (c *Client) CopySnapshots(snapshot *rds.DBSnapshot, destination, kmsid, preSignedUrl, cleanArn string) error {

	input := &rds.CopyDBSnapshotInput{
		SourceRegion:               aws.String(config.AWSRegionSource),
		DestinationRegion:          aws.String(destination),
		SourceDBSnapshotIdentifier: aws.String(*snapshot.DBSnapshotArn),
		TargetDBSnapshotIdentifier: aws.String(cleanArn),
		Tags: []*rds.Tag{
			{
				Key:   aws.String("CreatedBy"),
				Value: aws.String("rdscheck"),
			},
			{
				Key:   aws.String("RDS Instance"),
				Value: aws.String(*snapshot.DBSnapshotIdentifier),
			},
			{
				Key:   aws.String("Status"),
				Value: aws.String("ready"),
			},
			{
				Key:   aws.String("ChecksFailed"),
				Value: aws.String("no"),
			},
		},
	}

	if *snapshot.Encrypted {
		input.PreSignedUrl = aws.String(preSignedUrl)
		input.KmsKeyId = aws.String(kmsid)
	}

	_, err := c.RDS.CopyDBSnapshot(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case rds.ErrCodeDBSnapshotAlreadyExistsFault:
				log.WithFields(log.Fields{
					"Snapshot": *snapshot.DBSnapshotIdentifier,
				}).Info("Snapshot already exist")
				return nil
			default:
				return err
			}
		} else {
			return err
		}
	} else {
		log.WithFields(log.Fields{
			"Snapshot":    *snapshot.DBSnapshotIdentifier,
			"From":        config.AWSRegionSource,
			"Destination": destination,
		}).Info("Snapshot copied")
	}
	return nil
}

// PreSignUrl presigned an aws url so that we can copy an encrypted snapshot from a region to another
func (c *Client) PreSignUrl(destinationRegion, snapshotArn, kmsid, cleanArn string) (string, error) {
	input := &rds.CopyDBSnapshotInput{
		SourceRegion:               aws.String(config.AWSRegionSource),
		DestinationRegion:          aws.String(destinationRegion),
		SourceDBSnapshotIdentifier: aws.String(snapshotArn),
		KmsKeyId:                   aws.String(kmsid),
		TargetDBSnapshotIdentifier: aws.String(cleanArn),
	}
	req, _ := c.RDS.CopyDBSnapshotRequest(input)
	url, err := req.Presign(time.Duration(5) * time.Minute)
	if err != nil {
		return "", err
	}
	return url, nil
}

// GetOldSnapshots gets old snapshots based on the retention policy
// retentionDays is a integer of the number of days we want to keep the snapshots.
func (c *Client) GetOldSnapshots(snapshots []*rds.DBSnapshot, retentionDays int) ([]*rds.DBSnapshot, error) {
	var oldSnapshots []*rds.DBSnapshot
	oldDate := time.Now().AddDate(0, 0, -retentionDays)
	for _, s := range snapshots {
		if *s.Status != "available" {
			continue
		}

		if s.SnapshotCreateTime.After(oldDate) {
			break
		}

		oldSnapshots = append(oldSnapshots, s)
	}
	return oldSnapshots, nil
}

//  DeleteOldSnapshots deletes snapshots returned by GetOldSnapshots
func (c *Client) DeleteOldSnapshots(snapshots []*rds.DBSnapshot) error {
	for _, s := range snapshots {
		if s.DBSnapshotIdentifier == nil {
			log.Info("No old snapshots to delete")
			break
		}

		input := &rds.DeleteDBSnapshotInput{
			DBSnapshotIdentifier: aws.String(*s.DBSnapshotIdentifier),
		}

		_, err := c.RDS.DeleteDBSnapshot(input)
		if err != nil {
			return err
		} else {
			log.WithFields(log.Fields{
				"Snapshot": *s.DBSnapshotIdentifier,
			}).Info("Snapshot deleted")
			return nil
		}
	}
	return nil
}

// CheckIfDatabaseSubnetGroupExist return true if the Subnet Group already exist
func (c *Client) CheckIfDatabaseSubnetGroupExist(snapshot *rds.DBSnapshot) bool {
	input := &rds.DescribeDBSubnetGroupsInput{
		DBSubnetGroupName: aws.String(*snapshot.DBSnapshotIdentifier),
	}
	_, err := c.RDS.DescribeDBSubnetGroups(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case rds.ErrCodeDBSubnetGroupAlreadyExistsFault:
				return true
			default:
				return false
			}
		} else {
			return false
		}
	}
	return true
}

// CreateDatabaseSubnetGroup creates the Subnet Group if it doesnt already exist
func (c *Client) CreateDatabaseSubnetGroup(snapshot *rds.DBSnapshot, subnetids []string) error {

	input := &rds.CreateDBSubnetGroupInput{
		DBSubnetGroupDescription: aws.String(*snapshot.DBSnapshotIdentifier),
		DBSubnetGroupName:        aws.String(*snapshot.DBSnapshotIdentifier),
		SubnetIds:                aws.StringSlice(subnetids),
		Tags: []*rds.Tag{
			{
				Key:   aws.String("CreatedBy"),
				Value: aws.String("rdscheck"),
			},
			{
				Key:   aws.String("Snapshot"),
				Value: aws.String(*snapshot.DBSnapshotIdentifier),
			},
		},
	}

	_, err := c.RDS.CreateDBSubnetGroup(input)
	if err != nil {
		return err
	}

	return nil
}

// CreateDBFromSnapshot creates the RDS instance from a snapshot
func (c *Client) CreateDBFromSnapshot(snapshot *rds.DBSnapshot, instancetype string, vpcsecuritygroupids []string) error {

	input := &rds.RestoreDBInstanceFromDBSnapshotInput{
		AutoMinorVersionUpgrade: aws.Bool(false),
		DBInstanceClass:         aws.String(instancetype),
		DBInstanceIdentifier:    aws.String(*snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier),
		DBSnapshotIdentifier:    aws.String(*snapshot.DBSnapshotIdentifier),
		DBSubnetGroupName:       aws.String(*snapshot.DBSnapshotIdentifier),
		DeletionProtection:      aws.Bool(false),
		Engine:                  aws.String(*snapshot.Engine),
		MultiAZ:                 aws.Bool(false),
		Port:                    aws.Int64(*snapshot.Port),
		PubliclyAccessible:      aws.Bool(false),
		Tags: []*rds.Tag{
			{
				Key:   aws.String("CreatedBy"),
				Value: aws.String("rdscheck"),
			},
			{
				Key:   aws.String("Snapshot"),
				Value: aws.String(*snapshot.DBSnapshotIdentifier),
			},
			{
				Key:   aws.String("Status"),
				Value: aws.String("testing"),
			},
		},
		VpcSecurityGroupIds: aws.StringSlice(vpcsecuritygroupids),
	}

	_, err := c.RDS.RestoreDBInstanceFromDBSnapshot(input)
	if err != nil {
		return err
	}

	return nil
}

// Delete the RDS instance created from a specific snapshot
func (c *Client) DeleteDB(snapshot *rds.DBSnapshot) error {
	input := &rds.DeleteDBInstanceInput{
		DBInstanceIdentifier:   aws.String(*snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier),
		DeleteAutomatedBackups: aws.Bool(true),
		SkipFinalSnapshot:      aws.Bool(true),
	}

	_, err := c.RDS.DeleteDBInstance(input)
	if err != nil {
		return err
	}

	return nil
}

// UpdateTag updates a tag value on a snapshot
func (c *Client) UpdateTag(snapshot *rds.DBSnapshot, key, value string) error {
	inputRemove := &rds.RemoveTagsFromResourceInput{
		ResourceName: aws.String(*snapshot.DBSnapshotArn),
		TagKeys: []*string{
			aws.String(key),
		},
	}
	_, err := c.RDS.RemoveTagsFromResource(inputRemove)
	if err != nil {
		return errors.New("Could not remove tag")
	}

	inputAdd := &rds.AddTagsToResourceInput{
		ResourceName: aws.String(*snapshot.DBSnapshotArn),
		Tags: []*rds.Tag{
			{
				Key:   aws.String(key),
				Value: aws.String(value),
			},
		},
	}
	_, err = c.RDS.AddTagsToResource(inputAdd)
	if err != nil {
		return errors.New("Could not update tag")
	}
	return nil
}

// CheckTag checks the value of a specific tag (key) on a AWS resource
func (c *Client) CheckTag(arn, key, value string) bool {
	input := &rds.ListTagsForResourceInput{
		ResourceName: aws.String(arn),
	}
	o, err := c.RDS.ListTagsForResource(input)
	if err != nil {
		return false
	}
	for _, t := range o.TagList {
		if *t.Key == key && *t.Value == value {
			return true
		}
	}
	return false
}

// GetDBInstanceInfo returns informations about a rds instance
func (c *Client) GetDBInstanceInfo(snapshot *rds.DBSnapshot) (*rds.DBInstance, error) {
	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(*snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier),
	}
	o, err := c.RDS.DescribeDBInstances(input)
	if err != nil {
		return nil, err
	}
	for _, db := range o.DBInstances {
		return db, nil
	}
	return nil, nil
}

// DeleteDatabaseSubnetGroup deletes the subnet group created for a rds instance
func (c *Client) DeleteDatabaseSubnetGroup(snapshot *rds.DBSnapshot) error {
	input := &rds.DeleteDBSubnetGroupInput{
		DBSubnetGroupName: aws.String(*snapshot.DBSnapshotIdentifier),
	}
	_, err := c.RDS.DeleteDBSubnetGroup(input)
	if err != nil {
		return err
	}

	return nil
}

// ChangeDBpassword changes the database password of a rds instance
func (c *Client) ChangeDBpassword(snapshot *rds.DBSnapshot, DBArn, password string) error {
	input := &rds.ModifyDBInstanceInput{
		DBInstanceIdentifier: aws.String(*snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier),
		MasterUserPassword:   aws.String(password),
	}
	_, err := c.RDS.ModifyDBInstance(input)
	if err != nil {
		return err
	}

	statusOk := false

	for !statusOk {
		time.Sleep(2 * time.Second)
		db, err := c.GetDBInstanceInfo(snapshot)
		if err != nil {
			return err
		}
		if *db.DBInstanceStatus == "resetting-master-credentials" {
			statusOk = true
		}
	}

	return nil
}

// GetDBInstanceStatus returns the status of a rds instance
func (c *Client) GetDBInstanceStatus(snapshot *rds.DBSnapshot) string {
	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(*snapshot.DBInstanceIdentifier + "-" + *snapshot.DBSnapshotIdentifier),
	}
	o, err := c.RDS.DescribeDBInstances(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case rds.ErrCodeDBInstanceNotFoundFault:
				return ""
			default:
				return ""
			}
		} else {
			log.WithError(err).Error("GetDBInstanceStatus failed to DescribeDBInstances")
			return ""
		}
	}
	for _, db := range o.DBInstances {
		return *db.DBInstanceStatus
	}
	return ""
}

// GetTagValue returns the value of a specific tag on a AWS resource
func (c *Client) GetTagValue(arn, key string) string {
	input := &rds.ListTagsForResourceInput{
		ResourceName: aws.String(arn),
	}
	o, err := c.RDS.ListTagsForResource(input)
	if err != nil {
		return ""
	}
	for _, t := range o.TagList {
		if *t.Key == key {
			return *t.Value
		}
	}
	return ""
}
