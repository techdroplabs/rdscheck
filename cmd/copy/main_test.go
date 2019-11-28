package main

import (
	"bytes"
	"io"
	"io/ioutil"
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
		SnapshotType:         aws.String("automated"),
		Encrypted:            aws.Bool(true),
	},
	&rds.DBSnapshot{
		Status:               aws.String("available"),
		DBSnapshotIdentifier: aws.String("test-2"),
		SnapshotCreateTime:   aws.Time(time.Now()),
		DBSnapshotArn:        aws.String("arn:aws:rds:us-west-2:123456789012:snapshot:test-2"),
		SnapshotType:         aws.String("automated"),
		Encrypted:            aws.Bool(true),
	},
}

func (m *mockDefaultChecks) CopySnapshots(snapshot *rds.DBSnapshot, destination, kmsid, preSignedUrl, cleanArn string) error {
	args := m.Called(snapshot, destination, kmsid)
	return args.Error(0)
}

func (m *mockDefaultChecks) GetSnapshots(DBInstanceIdentifier string) ([]*rds.DBSnapshot, error) {
	args := m.Called(DBInstanceIdentifier)
	return args.Get(0).([]*rds.DBSnapshot), args.Error(1)
}

func (m *mockDefaultChecks) GetOldSnapshots(snapshots []*rds.DBSnapshot, retention int) ([]*rds.DBSnapshot, error) {
	args := m.Called(snapshots, retention)
	return args.Get(0).([]*rds.DBSnapshot), args.Error(1)
}

func (m *mockDefaultChecks) DeleteOldSnapshots(snapshots []*rds.DBSnapshot) error {
	args := m.Called(snapshots)
	return args.Error(0)
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

func (m *mockDefaultChecks) PreSignUrl(destinationRegion, snapshotArn, kmsid, cleanArn string) (string, error) {
	args := m.Called(destinationRegion, snapshotArn, kmsid, cleanArn)
	return args.Get(0).(string), args.Error(1)
}

func (m *mockDefaultChecks) CheckTag(arn string, key string, value string) bool {
	args := m.Called(arn, key, value)
	return args.Bool(0)
}

func (m *mockDefaultChecks) CleanArn(snapshot *rds.DBSnapshot) string {
	args := m.Called(snapshot)
	return args.Get(0).(string)
}

func (m *mockDefaultChecks) PostDatadogChecks(snapshot *rds.DBSnapshot, metricName, status, cmdName string) error {
	args := m.Called(snapshot, metricName, status, cmdName)
	return args.Error(0)
}

func TestCopy(t *testing.T) {
	c := &mockDefaultChecks{}

	yaml, _ := ioutil.ReadFile("../../example/checks.yml")
	input := bytes.NewReader(yaml)

	c.On("SetSessions", mock.Anything).Return()
	c.On("GetYamlFileFromS3", mock.Anything, mock.Anything).Return(input, nil)
	c.On("UnmarshalYamlFile", mock.Anything).Return(doc, nil)
	c.On("GetSnapshots", mock.Anything).Return(snapshots, nil)
	c.On("PostDatadogChecks", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	c.On("CleanArn", mock.Anything).Return("test")
	c.On("PreSignUrl", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("https://url.local", nil)
	c.On("CopySnapshots", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	c.On("CheckTag", mock.Anything, mock.Anything, mock.Anything).Return(true)
	c.On("GetOldSnapshots", mock.Anything, mock.Anything).Return(snapshots, nil)
	c.On("DeleteOldSnapshots", mock.Anything).Return(nil)

	err := copy(c, c)

	assert.Nil(t, err)
	c.AssertExpectations(t)
}
