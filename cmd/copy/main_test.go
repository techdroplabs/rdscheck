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
	},
	&rds.DBSnapshot{
		Status:               aws.String("available"),
		DBSnapshotIdentifier: aws.String("test-2"),
		SnapshotCreateTime:   aws.Time(time.Now()),
		DBSnapshotArn:        aws.String("arn:aws:rds:us-west-2:123456789012:snapshot:test-2"),
	},
}

func (m *mockDefaultChecks) CopySnapshots(snapshot *rds.DBSnapshot) error {
	args := m.Called(snapshot)
	return args.Error(0)
}

func (m *mockDefaultChecks) GetSnapshots(DBInstanceIdentifier string) ([]*rds.DBSnapshot, error) {
	args := m.Called(DBInstanceIdentifier)
	return args.Get(0).([]*rds.DBSnapshot), args.Error(1)
}

func (m *mockDefaultChecks) GetOldSnapshots(snapshots []*rds.DBSnapshot) ([]*rds.DBSnapshot, error) {
	args := m.Called(snapshots)
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

func (m *mockDefaultChecks) copy(destination checks.DefaultChecks, snapshots []*rds.DBSnapshot, instance checks.Instances) error {
	args := m.Called(destination, snapshots, instance)
	return args.Error(0)
}

func TestRun(t *testing.T) {
	c := &mockDefaultChecks{}

	yaml, _ := ioutil.ReadFile("../../example/checks.yaml")
	input := bytes.NewReader(yaml)

	c.On("SetSessions", mock.Anything).Return()
	c.On("GetYamlFileFromS3", mock.Anything, mock.Anything).Return(input, nil)
	c.On("UnmarshalYamlFile", mock.Anything).Return(doc, nil)
	c.On("GetSnapshots", mock.Anything).Return(snapshots, nil)
	c.On("CopySnapshots", mock.Anything).Return(nil)
	c.On("GetOldSnapshots", mock.Anything).Return(snapshots, nil)
	c.On("DeleteOldSnapshots", mock.Anything).Return(nil)

	err := run(c, c)

	assert.Nil(t, err)
	c.AssertExpectations(t)
}
