package dbinstance

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
	"github.com/techdroplabs/rdscheck/common"
	"github.com/techdroplabs/rdscheck/config"
)

func NewDBInstance() *common.DBInstance {
	return &common.DBInstance{}
}

// GetSnapshots gets the latest snapshots of a RDS instance
func GetSnapshots(i *rds.RDS, DBInstanceIdentifier string) ([]*rds.DBSnapshot, error) {
	input := &rds.DescribeDBSnapshotsInput{
		DBInstanceIdentifier: aws.String(DBInstanceIdentifier),
	}

	r, err := i.DescribeDBSnapshots(input)
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
func CopySnapshots(i *rds.RDS, snap *rds.DBSnapshot) error {

	arn := strings.SplitN(*snap.DBSnapshotArn, ":", 8)
	cleanArn := arn[len(arn)-1]

	input := &rds.CopyDBSnapshotInput{
		SourceRegion:               aws.String(config.AWSRegion),
		DestinationRegion:          aws.String(config.DestinationRegion),
		SourceDBSnapshotIdentifier: aws.String(*snap.DBSnapshotArn),
		TargetDBSnapshotIdentifier: aws.String(cleanArn),
		Tags: []*rds.Tag{
			{
				Key:   aws.String("CreatedBy"),
				Value: aws.String("rdscheck"),
			},
			{
				Key:   aws.String("RDS Instance"),
				Value: aws.String(*snap.DBSnapshotIdentifier),
			},
			{
				Key:   aws.String("Status"),
				Value: aws.String("ready"),
			},
		},
	}
	_, err := i.CopyDBSnapshot(input)
	if err != nil {
		return err
	}
	return nil
}

// GetOldSnapshots gets old snapshots based on the retention policy
func GetOldSnapshots(i *rds.RDS, snap []*rds.DBSnapshot) ([]*rds.DBSnapshot, error) {
	var oldSnapshots []*rds.DBSnapshot
	oldDate := time.Now().AddDate(0, 0, -config.SnapshotRetention)
	for _, s := range snap {
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
func DeleteOldSnapshots(i *rds.RDS, snap []*rds.DBSnapshot) error {
	for _, s := range snap {
		if s.DBSnapshotIdentifier == nil {
			fmt.Sprintln("Nothing to delete")
			break
		}

		input := &rds.DeleteDBSnapshotInput{
			DBSnapshotIdentifier: aws.String(*s.DBSnapshotIdentifier),
		}

		_, err := i.DeleteDBSnapshot(input)
		if err != nil {
			return err
		}
	}
	return nil
}

// CheckIfDatabaseSubnetGroupExist return true if the Subnet Group already exist
func CheckIfDatabaseSubnetGroupExist(i *rds.RDS, snap *rds.DBSnapshot) bool {
	input := &rds.DescribeDBSubnetGroupsInput{
		DBSubnetGroupName: aws.String(*snap.DBSnapshotIdentifier),
	}
	_, err := i.DescribeDBSubnetGroups(input)
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
func CreateDatabaseSubnetGroup(i *rds.RDS, snap *rds.DBSnapshot, subnetids []string) error {

	input := &rds.CreateDBSubnetGroupInput{
		DBSubnetGroupDescription: aws.String(*snap.DBSnapshotIdentifier),
		DBSubnetGroupName:        aws.String(*snap.DBSnapshotIdentifier),
		SubnetIds:                aws.StringSlice(subnetids),
		Tags: []*rds.Tag{
			{
				Key:   aws.String("CreatedBy"),
				Value: aws.String("rdscheck"),
			},
			{
				Key:   aws.String("Snapshot"),
				Value: aws.String(*snap.DBSnapshotIdentifier),
			},
		},
	}

	_, err := i.CreateDBSubnetGroup(input)
	if err != nil {
		return err
	}

	return nil
}

// CreateDBFromSnapshot creates the RDS instance from a snapshot
func CreateDBFromSnapshot(i *rds.RDS, snap *rds.DBSnapshot, dbname string, vpcsecuritygroupids []string) error {

	input := &rds.RestoreDBInstanceFromDBSnapshotInput{
		AutoMinorVersionUpgrade: aws.Bool(false),
		DBInstanceClass:         aws.String("db.t2.micro"),
		DBInstanceIdentifier:    aws.String(*snap.DBInstanceIdentifier + "-" + *snap.DBSnapshotIdentifier),
		DBSnapshotIdentifier:    aws.String(*snap.DBSnapshotIdentifier),
		DBSubnetGroupName:       aws.String(*snap.DBSnapshotIdentifier),
		DeletionProtection:      aws.Bool(false),
		Engine:                  aws.String(*snap.Engine),
		MultiAZ:                 aws.Bool(false),
		Port:                    aws.Int64(*snap.Port),
		PubliclyAccessible:      aws.Bool(false),
		Tags: []*rds.Tag{
			{
				Key:   aws.String("CreatedBy"),
				Value: aws.String("rdscheck"),
			},
			{
				Key:   aws.String("Snapshot"),
				Value: aws.String(*snap.DBSnapshotIdentifier),
			},
			{
				Key:   aws.String("Status"),
				Value: aws.String("testing"),
			},
		},
		VpcSecurityGroupIds: aws.StringSlice(vpcsecuritygroupids),
	}
	if *snap.Engine != "postgres" {
		input.DBName = aws.String(dbname)
	}

	_, err := i.RestoreDBInstanceFromDBSnapshot(input)
	if err != nil {
		return err
	}

	return nil
}

// Delete the RDS instance created from a specific snapshot
func DeleteDB(i *rds.RDS, snap *rds.DBSnapshot) error {
	input := &rds.DeleteDBInstanceInput{
		DBInstanceIdentifier:   aws.String(*snap.DBInstanceIdentifier + "-" + *snap.DBSnapshotIdentifier),
		DeleteAutomatedBackups: aws.Bool(true),
		SkipFinalSnapshot:      aws.Bool(true),
	}

	_, err := i.DeleteDBInstance(input)
	if err != nil {
		return err
	}

	return nil
}

// UpdateStatusTag updates the "Status" tag on the snapshot
func UpdateStatusTag(snap *rds.DBSnapshot, i *rds.RDS, status string) {
	inputRemove := &rds.RemoveTagsFromResourceInput{
		ResourceName: aws.String(*snap.DBSnapshotArn),
		TagKeys: []*string{
			aws.String("Status"),
		},
	}
	_, err := i.RemoveTagsFromResource(inputRemove)
	if err != nil {
		log.WithError(err).Error("Could not remove status tag")
	}

	inputAdd := &rds.AddTagsToResourceInput{
		ResourceName: aws.String(*snap.DBSnapshotArn),
		Tags: []*rds.Tag{
			{
				Key:   aws.String("Status"),
				Value: aws.String(status),
			},
		},
	}
	_, err = i.AddTagsToResource(inputAdd)
	if err != nil {
		log.WithError(err).Error("Could not update status tag")
	}
}

// CheckTag checks the value of a specific tag (key) on a AWS resource
func CheckTag(i *rds.RDS, arn, key, value string) bool {
	input := &rds.ListTagsForResourceInput{
		ResourceName: aws.String(arn),
	}
	o, err := i.ListTagsForResource(input)
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
func GetDBInstanceInfo(i *rds.RDS, snap *rds.DBSnapshot) (*rds.DBInstance, error) {
	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(*snap.DBInstanceIdentifier + "-" + *snap.DBSnapshotIdentifier),
	}
	o, err := i.DescribeDBInstances(input)
	if err != nil {
		return nil, err
	}
	for _, db := range o.DBInstances {
		return db, nil
	}
	return nil, nil
}

// DeleteDatabaseSubnetGroup deletes the subnet group created for a rds instance
func DeleteDatabaseSubnetGroup(i *rds.RDS, snap *rds.DBSnapshot) error {
	input := &rds.DeleteDBSubnetGroupInput{
		DBSubnetGroupName: aws.String(*snap.DBSnapshotIdentifier),
	}
	_, err := i.DeleteDBSubnetGroup(input)
	if err != nil {
		return err
	}

	return nil
}

// ChangeDBpassword changes the database password of a rds instance
func ChangeDBpassword(i *rds.RDS, snap *rds.DBSnapshot, DBArn, password string) error {
	input := &rds.ModifyDBInstanceInput{
		DBInstanceIdentifier: aws.String(*snap.DBInstanceIdentifier + "-" + *snap.DBSnapshotIdentifier),
		MasterUserPassword:   aws.String(password),
	}
	_, err := i.ModifyDBInstance(input)
	if err != nil {
		return err
	}

	statusOk := false

	for !statusOk {
		time.Sleep(2 * time.Second)
		db, err := GetDBInstanceInfo(i, snap)
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
func GetDBInstanceStatus(i *rds.RDS, snap *rds.DBSnapshot) string {
	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(*snap.DBInstanceIdentifier + "-" + *snap.DBSnapshotIdentifier),
	}
	o, err := i.DescribeDBInstances(input)
	if err != nil {
		return ""
	}
	for _, db := range o.DBInstances {
		return *db.DBInstanceStatus
	}
	return ""
}

// GetTagValue returns the value of a specific tag on a AWS resource
func GetTagValue(i *rds.RDS, arn, key string) string {
	input := &rds.ListTagsForResourceInput{
		ResourceName: aws.String(arn),
	}
	o, err := i.ListTagsForResource(input)
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
