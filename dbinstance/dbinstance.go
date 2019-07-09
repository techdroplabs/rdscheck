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
	"gopkg.in/zorkian/go-datadog-api.v2"
)

func NewDBInstance() *common.DBInstance {
	return &common.DBInstance{}
}

// GetSnapshots gets the latest snapshot of a RDS instance
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
func CopySnapshots(i *rds.RDS, s *rds.DBSnapshot) error {

	arn := strings.SplitN(*s.DBSnapshotArn, ":", 8)
	cleanArn := arn[len(arn)-1]

	input := &rds.CopyDBSnapshotInput{
		SourceRegion:               aws.String(config.AWSRegion),
		DestinationRegion:          aws.String(config.SnapshotDestinationRegion),
		SourceDBSnapshotIdentifier: aws.String(*s.DBSnapshotArn),
		TargetDBSnapshotIdentifier: aws.String(cleanArn),
		Tags: []*rds.Tag{
			{
				Key:   aws.String("CreatedBy"),
				Value: aws.String("rdscheck"),
			},
			{
				Key:   aws.String("RDS Instance"),
				Value: aws.String(*s.DBSnapshotIdentifier),
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
func CheckIfDatabaseSubnetGroupExist(i *rds.RDS, s *rds.DBSnapshot) bool {
	input := &rds.DescribeDBSubnetGroupsInput{
		DBSubnetGroupName: aws.String(*s.DBSnapshotIdentifier),
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
func CreateDatabaseSubnetGroup(dd *datadog.Client, i *rds.RDS, s *rds.DBSnapshot, SubnetIds []string) error {
	input := &rds.CreateDBSubnetGroupInput{
		DBSubnetGroupDescription: aws.String(*s.DBSnapshotIdentifier),
		DBSubnetGroupName:        aws.String(*s.DBSnapshotIdentifier),
		SubnetIds:                aws.StringSlice(SubnetIds),
		Tags: []*rds.Tag{
			{
				Key:   aws.String("CreatedBy"),
				Value: aws.String("rdscheck"),
			},
			{
				Key:   aws.String("Snapshot"),
				Value: aws.String(*s.DBSnapshotIdentifier),
			},
		},
	}

	_, err := i.CreateDBSubnetGroup(input)
	if err != nil {
		common.PostDatadogChecks(dd, "rdscheck.status", "critical", s)
		return err
	}
	common.PostDatadogChecks(dd, "rdscheck.status", "ok", s)
	return err
}

// CheckIfRDSInstanceExist returns true if the RDS instance already exist
func CheckIfRDSInstanceExist(i *rds.RDS, s *rds.DBSnapshot) bool {
	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(*s.DBInstanceIdentifier + "-" + *s.DBSnapshotIdentifier),
	}
	_, err := i.DescribeDBInstances(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case rds.ErrCodeDBInstanceAlreadyExistsFault:
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

// CreateDBFromSnapshot creates the RDS instance from a snapshot
func CreateDBFromSnapshot(dd *datadog.Client, i *rds.RDS, s *rds.DBSnapshot, dbName string, VpcSecurityGroupIds []string) error {
	input := &rds.RestoreDBInstanceFromDBSnapshotInput{
		AutoMinorVersionUpgrade: aws.Bool(false),
		DBInstanceClass:         aws.String("db.t2.micro"),
		DBInstanceIdentifier:    aws.String(*s.DBInstanceIdentifier + "-" + *s.DBSnapshotIdentifier),
		DBSnapshotIdentifier:    aws.String(*s.DBSnapshotIdentifier),
		DBSubnetGroupName:       aws.String(*s.DBSnapshotIdentifier),
		DeletionProtection:      aws.Bool(false),
		Engine:                  aws.String(*s.Engine),
		MultiAZ:                 aws.Bool(false),
		Port:                    aws.Int64(*s.Port),
		PubliclyAccessible:      aws.Bool(false),
		Tags: []*rds.Tag{
			{
				Key:   aws.String("CreatedBy"),
				Value: aws.String("rdscheck"),
			},
			{
				Key:   aws.String("Snapshot"),
				Value: aws.String(*s.DBSnapshotIdentifier),
			},
			{
				Key:   aws.String("Status"),
				Value: aws.String("ready"),
			},
		},
		VpcSecurityGroupIds: aws.StringSlice(VpcSecurityGroupIds),
	}
	if *s.Engine != "postgres" {
		input.DBName = aws.String(dbName)
	}

	_, err := i.RestoreDBInstanceFromDBSnapshot(input)
	if err != nil {
		common.PostDatadogChecks(dd, "rdscheck.status", "critical", s)
		return err
	}

	err = UpdateStatusTag(dd, s, i, *s.DBSnapshotArn, "testing")
	if err != nil {
		log.WithError(err).Error("Could not update snapshot status")
		return err
	}
	common.PostDatadogChecks(dd, "rdscheck.status", "ok", s)
	return nil
}

// Delete the RDS instance
func DeleteDB(dd *datadog.Client, i *rds.RDS, s *rds.DBSnapshot) error {
	input := &rds.DeleteDBInstanceInput{
		DBInstanceIdentifier:   aws.String(*s.DBInstanceIdentifier + "-" + *s.DBSnapshotIdentifier),
		DeleteAutomatedBackups: aws.Bool(true),
		SkipFinalSnapshot:      aws.Bool(true),
	}
	_, err := i.DeleteDBInstance(input)
	if err != nil {
		common.PostDatadogChecks(dd, "rdscheck.status", "critical", s)
		return err
	}

	err = UpdateStatusTag(dd, s, i, *s.DBSnapshotArn, "tested")
	if err != nil {
		log.WithError(err).Error("Could not update snapshot status")
		return err
	}

	common.PostDatadogChecks(dd, "rdscheck.status", "ok", s)
	return nil
}

func UpdateStatusTag(dd *datadog.Client, s *rds.DBSnapshot, i *rds.RDS, arn, status string) error {
	inputRemove := &rds.RemoveTagsFromResourceInput{
		ResourceName: aws.String(arn),
		TagKeys: []*string{
			aws.String("Status"),
		},
	}
	_, err := i.RemoveTagsFromResource(inputRemove)
	if err != nil {
		common.PostDatadogChecks(dd, "rdscheck.status", "critical", s)
		return err
	}

	inputAdd := &rds.AddTagsToResourceInput{
		ResourceName: aws.String(arn),
		Tags: []*rds.Tag{
			{
				Key:   aws.String("Status"),
				Value: aws.String(status),
			},
		},
	}
	_, err = i.AddTagsToResource(inputAdd)
	if err != nil {
		common.PostDatadogChecks(dd, "rdscheck.status", "critical", s)
		return err
	}
	return nil
}

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

func GetDBInstanceInfo(dd *datadog.Client, i *rds.RDS, s *rds.DBSnapshot) (*rds.DBInstance, error) {
	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(*s.DBInstanceIdentifier + "-" + *s.DBSnapshotIdentifier),
	}
	o, err := i.DescribeDBInstances(input)
	if err != nil {
		common.PostDatadogChecks(dd, "rdscheck.status", "critical", s)
		return nil, err
	}
	for _, db := range o.DBInstances {
		return db, nil
	}
	common.PostDatadogChecks(dd, "rdscheck.status", "ok", s)
	return nil, nil
}

func DeleteDatabaseSubnetGroup(dd *datadog.Client, i *rds.RDS, s *rds.DBSnapshot) error {
	input := &rds.DeleteDBSubnetGroupInput{
		DBSubnetGroupName: aws.String(*s.DBSnapshotIdentifier),
	}
	_, err := i.DeleteDBSubnetGroup(input)
	if err != nil {
		common.PostDatadogChecks(dd, "rdscheck.status", "critical", s)
		return err
	}

	common.PostDatadogChecks(dd, "rdscheck.status", "ok", s)
	return err
}

func ChangeDBpassword(dd *datadog.Client, i *rds.RDS, s *rds.DBSnapshot, DBArn, password string) error {
	input := &rds.ModifyDBInstanceInput{
		DBInstanceIdentifier: aws.String(*s.DBInstanceIdentifier + "-" + *s.DBSnapshotIdentifier),
		MasterUserPassword:   aws.String(password),
	}
	_, err := i.ModifyDBInstance(input)
	if err != nil {
		common.PostDatadogChecks(dd, "rdscheck.status", "critical", s)
		return err
	}

	statusOk := false

	for !statusOk {
		time.Sleep(2 * time.Second)
		db, err := GetDBInstanceInfo(dd, i, s)
		if err != nil {
			return err
		}
		if *db.DBInstanceStatus == "resetting-master-credentials" {
			statusOk = true
		}
	}

	err = UpdateStatusTag(dd, s, i, DBArn, "testing")
	if err != nil {
		log.WithError(err).Error("Could not update snapshot status")
		return err
	}
	common.PostDatadogChecks(dd, "rdscheck.status", "ok", s)
	return nil
}

func GetDBInstanceStatus(i *rds.RDS, s *rds.DBSnapshot) string {
	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(*s.DBInstanceIdentifier + "-" + *s.DBSnapshotIdentifier),
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
