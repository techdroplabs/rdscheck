package main

import (
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/techdroplabs/rdscheck/checks"
)

type mockDefaultChecks struct {
	checks.DefaultChecks
	mock.Mock
}

var doc = checks.Doc{
	Instances: []checks.Instances{
		checks.Instances{
			Name:     "test",
			Database: "test",
			Password: "password",
			Queries: []checks.Queries{
				checks.Queries{
					Query: "SELECT tablename FROM pg_catalog.pg_tables;",
					Regex: "^pg_statistic$",
				},
			},
		},
	},
}

var snapshots = []*rds.DBSnapshot{
	&rds.DBSnapshot{
		Status:               aws.String("available"),
		DBSnapshotIdentifier: aws.String("test"),
		SnapshotCreateTime:   aws.Time(time.Now().AddDate(0, 0, -10)),
		DBSnapshotArn:        aws.String("arn:aws:rds:us-west-2:123456789012:snapshot:test"),
	},
	&rds.DBSnapshot{
		Status:               aws.String("available"),
		DBSnapshotIdentifier: aws.String("test-2"),
		SnapshotCreateTime:   aws.Time(time.Now()),
		DBSnapshotArn:        aws.String("arn:aws:rds:us-west-2:123456789012:snapshot:test-2"),
	},
}

var rdsInstance = &rds.DBInstance{
	DBInstanceArn:    aws.String("arn:aws:rds:us-west-2:123456789012:rds:test"),
	DBInstanceStatus: aws.String("resetting-master-credentials"),
	DBName:           aws.String("test"),
	Endpoint: &rds.Endpoint{
		Address: aws.String("mystack-mydb-1apw1j4phylrk.cg034hpkmmjt.us-west-2.rds.amazonaws.com"),
		Port:    aws.Int64(5234),
	},
	MasterUsername: aws.String("sendwithus"),
}

func (m *mockDefaultChecks) GetSnapshots(DBInstanceIdentifier string) ([]*rds.DBSnapshot, error) {
	args := m.Called(DBInstanceIdentifier)
	return args.Get(0).([]*rds.DBSnapshot), args.Error(1)
}

func (m *mockDefaultChecks) CheckTag(arn string, key string, value string) bool {
	args := m.Called(arn, key, value)
	return args.Bool(0)
}

func (m *mockDefaultChecks) GetTagValue(arn, key string) string {
	args := m.Called(arn, key)
	return args.Get(0).(string)
}

func (m *mockDefaultChecks) PostDatadogChecks(snapshot *rds.DBSnapshot, metricName, status string) error {
	args := m.Called(snapshot, metricName, status)
	return args.Error(0)
}

func (m *mockDefaultChecks) CreateDatabaseSubnetGroup(snapshot *rds.DBSnapshot, subnetids []string) error {
	args := m.Called(snapshot, subnetids)
	return args.Error(0)
}

func (m *mockDefaultChecks) UpdateTag(snapshot *rds.DBSnapshot, key, value string) error {
	args := m.Called(snapshot, key, value)
	return args.Error(0)
}

func (m *mockDefaultChecks) CreateDBFromSnapshot(snapshot *rds.DBSnapshot, dbname string, vpcsecuritygroupids []string) error {
	args := m.Called(snapshot, dbname, vpcsecuritygroupids)
	return args.Error(0)
}

func (m *mockDefaultChecks) GetDBInstanceStatus(snapshot *rds.DBSnapshot) string {
	args := m.Called(snapshot)
	return args.Get(0).(string)
}

func (m *mockDefaultChecks) GetDBInstanceInfo(snapshot *rds.DBSnapshot) (*rds.DBInstance, error) {
	args := m.Called(snapshot)
	return args.Get(0).(*rds.DBInstance), args.Error(1)
}

func (m *mockDefaultChecks) ChangeDBpassword(snapshot *rds.DBSnapshot, DBArn, password string) error {
	args := m.Called(snapshot, DBArn, password)
	return args.Error(0)
}

func (m *mockDefaultChecks) CheckSQLQueries(query, regex string) bool {
	args := m.Called(query, regex)
	return args.Bool(0)
}

func (m *mockDefaultChecks) DeleteDB(snapshot *rds.DBSnapshot) error {
	args := m.Called(snapshot)
	return args.Error(0)
}

func (m *mockDefaultChecks) CheckIfDatabaseSubnetGroupExist(snapshot *rds.DBSnapshot) bool {
	args := m.Called(snapshot)
	return args.Bool(0)
}

func (m *mockDefaultChecks) DeleteDatabaseSubnetGroup(snapshot *rds.DBSnapshot) error {
	args := m.Called(snapshot)
	return args.Error(0)
}

func (m *mockDefaultChecks) InitDb(endpoint rds.Endpoint, user, password, dbname string) {
	m.Called(endpoint, user, password, dbname)
}

func (m *mockDefaultChecks) SetSessions(region string) {
	m.Called(region)
}

func (m *mockDefaultChecks) GetYamlFileFromS3(bucket string, key string) (io.Reader, error) {
	args := m.Called(bucket, key)
	return args.Get(0).(io.Reader), args.Error(1)
}

func (m *mockDefaultChecks) UnmarshalYamlFile(body io.Reader) (checks.Doc, error) {
	args := m.Called(body)
	return args.Get(0).(checks.Doc), args.Error(1)
}

func TestGetDoc(t *testing.T) {
	c := &mockDefaultChecks{}

	file, _ := os.Open("../example/checks.yaml")
	output := ioutil.NopCloser(file)

	doc := checks.Doc{
		Instances: []checks.Instances{
			checks.Instances{
				Name:     "rdscheck",
				Database: "rdscheck",
				Password: "thisisatest",
				Queries: []checks.Queries{
					checks.Queries{
						Query: "SELECT tablename FROM pg_catalog.pg_tables;",
						Regex: "^pg_statistic$",
					},
				},
			},
		},
	}

	c.On("SetSessions", mock.Anything).Return()
	c.On("GetYamlFileFromS3", mock.Anything, mock.Anything).Return(output, nil)
	c.On("UnmarshalYamlFile", mock.Anything).Return(doc, nil)

	_, err := getDoc(c)
	assert.Nil(t, err)
}

func TestValidateReady(t *testing.T) {
	c := &mockDefaultChecks{}

	c.On("GetSnapshots", mock.Anything).Return(snapshots, nil)
	c.On("CheckTag", mock.Anything, mock.Anything, mock.Anything).Return(true)
	c.On("GetTagValue", mock.Anything, mock.Anything).Return("ready")
	c.On("PostDatadogChecks", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	c.On("CreateDatabaseSubnetGroup", mock.Anything, mock.Anything).Return(nil)
	c.On("UpdateTag", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	err := validate(c, doc)

	assert.Nil(t, err)
	c.AssertExpectations(t)
}

func TestValidateRestore(t *testing.T) {
	c := &mockDefaultChecks{}

	c.On("GetSnapshots", mock.Anything).Return(snapshots, nil)
	c.On("CheckTag", mock.Anything, mock.Anything, mock.Anything).Return(true)
	c.On("GetTagValue", mock.Anything, mock.Anything).Return("restore")
	c.On("CreateDBFromSnapshot", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	c.On("UpdateTag", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	err := validate(c, doc)

	assert.Nil(t, err)
	c.AssertExpectations(t)
}

func TestValidateModify(t *testing.T) {
	c := &mockDefaultChecks{}

	c.On("GetSnapshots", mock.Anything).Return(snapshots, nil)
	c.On("CheckTag", mock.Anything, mock.Anything, mock.Anything).Return(true)
	c.On("GetTagValue", mock.Anything, mock.Anything).Return("modify")
	c.On("GetDBInstanceStatus", mock.Anything).Return("available")
	c.On("GetDBInstanceInfo", mock.Anything).Return(rdsInstance, nil)
	c.On("ChangeDBpassword", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	c.On("UpdateTag", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	err := validate(c, doc)

	assert.Nil(t, err)
	c.AssertExpectations(t)
}

func TestValidateVerify(t *testing.T) {
	c := &mockDefaultChecks{}

	c.On("GetSnapshots", mock.Anything).Return(snapshots, nil)
	c.On("CheckTag", mock.Anything, mock.Anything, mock.Anything).Return(true)
	c.On("GetTagValue", mock.Anything, mock.Anything).Return("verify")
	c.On("GetDBInstanceStatus", mock.Anything).Return("available")
	c.On("GetDBInstanceInfo", mock.Anything).Return(rdsInstance, nil)
	c.On("InitDb", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	c.On("CheckSQLQueries", mock.Anything, mock.Anything).Return(true)
	c.On("UpdateTag", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	err := validate(c, doc)

	assert.Nil(t, err)
	c.AssertExpectations(t)
}

func TestValidateAlarm(t *testing.T) {
	c := &mockDefaultChecks{}

	c.On("GetSnapshots", mock.Anything).Return(snapshots, nil)
	c.On("CheckTag", mock.Anything, mock.Anything, mock.Anything).Return(true)
	c.On("GetTagValue", mock.Anything, mock.Anything).Return("alarm")
	c.On("PostDatadogChecks", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	c.On("UpdateTag", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	err := validate(c, doc)

	assert.Nil(t, err)
	c.AssertExpectations(t)
}

func TestValidateClean(t *testing.T) {
	c := &mockDefaultChecks{}

	c.On("GetSnapshots", mock.Anything).Return(snapshots, nil)
	c.On("CheckTag", mock.Anything, mock.Anything, mock.Anything).Return(true)
	c.On("GetTagValue", mock.Anything, mock.Anything).Return("clean")
	c.On("DeleteDB", mock.Anything).Return(nil)
	c.On("UpdateTag", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	err := validate(c, doc)

	assert.Nil(t, err)
	c.AssertExpectations(t)
}

func TestValidateTested(t *testing.T) {
	c := &mockDefaultChecks{}

	c.On("GetSnapshots", mock.Anything).Return(snapshots, nil)
	c.On("CheckTag", mock.Anything, mock.Anything, mock.Anything).Return(true)
	c.On("GetTagValue", mock.Anything, mock.Anything).Return("tested")
	c.On("GetDBInstanceStatus", mock.Anything).Return("")
	c.On("CheckIfDatabaseSubnetGroupExist", mock.Anything).Return(true)
	c.On("DeleteDatabaseSubnetGroup", mock.Anything).Return(nil)

	err := validate(c, doc)

	assert.Nil(t, err)
	c.AssertExpectations(t)
}
