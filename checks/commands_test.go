package checks

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockRDS struct {
	rdsiface.RDSAPI
	mock.Mock
}

func (m *mockRDS) DescribeDBSnapshots(input *rds.DescribeDBSnapshotsInput) (*rds.DescribeDBSnapshotsOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*rds.DescribeDBSnapshotsOutput), args.Error(1)
}

func (m *mockRDS) CopyDBSnapshot(input *rds.CopyDBSnapshotInput) (*rds.CopyDBSnapshotOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*rds.CopyDBSnapshotOutput), args.Error(1)
}

func (m *mockRDS) DeleteDBSnapshot(input *rds.DeleteDBSnapshotInput) (*rds.DeleteDBSnapshotOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*rds.DeleteDBSnapshotOutput), args.Error(1)
}

func (m *mockRDS) DescribeDBSubnetGroups(input *rds.DescribeDBSubnetGroupsInput) (*rds.DescribeDBSubnetGroupsOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*rds.DescribeDBSubnetGroupsOutput), args.Error(1)
}

func (m *mockRDS) CreateDBSubnetGroup(input *rds.CreateDBSubnetGroupInput) (*rds.CreateDBSubnetGroupOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*rds.CreateDBSubnetGroupOutput), args.Error(1)
}

func (m *mockRDS) RestoreDBInstanceFromDBSnapshot(input *rds.RestoreDBInstanceFromDBSnapshotInput) (*rds.RestoreDBInstanceFromDBSnapshotOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*rds.RestoreDBInstanceFromDBSnapshotOutput), args.Error(1)
}

func (m *mockRDS) DeleteDBInstance(input *rds.DeleteDBInstanceInput) (*rds.DeleteDBInstanceOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*rds.DeleteDBInstanceOutput), args.Error(1)
}

func (m *mockRDS) RemoveTagsFromResource(input *rds.RemoveTagsFromResourceInput) (*rds.RemoveTagsFromResourceOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*rds.RemoveTagsFromResourceOutput), args.Error(1)
}

func (m *mockRDS) AddTagsToResource(input *rds.AddTagsToResourceInput) (*rds.AddTagsToResourceOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*rds.AddTagsToResourceOutput), args.Error(1)
}

func (m *mockRDS) ListTagsForResource(input *rds.ListTagsForResourceInput) (*rds.ListTagsForResourceOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*rds.ListTagsForResourceOutput), args.Error(1)
}

func (m *mockRDS) DescribeDBInstances(input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*rds.DescribeDBInstancesOutput), args.Error(1)
}

func (m *mockRDS) DeleteDBSubnetGroup(input *rds.DeleteDBSubnetGroupInput) (*rds.DeleteDBSubnetGroupOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*rds.DeleteDBSubnetGroupOutput), args.Error(1)
}

func (m *mockRDS) ModifyDBInstance(input *rds.ModifyDBInstanceInput) (*rds.ModifyDBInstanceOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*rds.ModifyDBInstanceOutput), args.Error(1)
}

func (m *mockRDS) CopyDBSnapshotRequest(input *rds.CopyDBSnapshotInput) (*request.Request, *rds.CopyDBSnapshotOutput) {
	args := m.Called(input)
	return args.Get(0).(*request.Request), args.Get(1).(*rds.CopyDBSnapshotOutput)
}

func TestGetSnapshots(t *testing.T) {
	rdsc := &mockRDS{}

	c := &Client{
		RDS: rdsc,
	}

	time1 := time.Now()

	rdsc.On("DescribeDBSnapshots", mock.Anything).Return(&rds.DescribeDBSnapshotsOutput{
		DBSnapshots: []*rds.DBSnapshot{
			&rds.DBSnapshot{
				Status:             aws.String("available"),
				SnapshotCreateTime: aws.Time(time1),
			},
			&rds.DBSnapshot{
				Status:             aws.String("available"),
				SnapshotCreateTime: aws.Time(time1),
			},
		},
	}, nil)

	value, err := c.GetSnapshots("test")
	assert.Nil(t, err)
	assert.Len(t, value, 2, "Expect two snapshots")
	rdsc.AssertExpectations(t)
}

func TestCopySnapshotsNoKms(t *testing.T) {
	rdsc := &mockRDS{}

	c := &Client{
		RDS: rdsc,
	}

	input := &rds.DBSnapshot{
		DBSnapshotIdentifier: aws.String("test"),
		DBSnapshotArn:        aws.String("arn:aws:rds:us-west-2:123456789012:snapshot:test"),
		Encrypted:            aws.Bool(false),
	}

	rdsc.On("CopyDBSnapshot", mock.Anything).Return(&rds.CopyDBSnapshotOutput{
		DBSnapshot: &rds.DBSnapshot{},
	}, nil)

	err := c.CopySnapshots(input, "us-west-2", "", "", "test")
	assert.Nil(t, err)
	rdsc.AssertExpectations(t)

}

func TestCopySnapshotsWithKms(t *testing.T) {
	rdsc := &mockRDS{}

	c := &Client{
		RDS: rdsc,
	}

	input := &rds.DBSnapshot{
		DBSnapshotIdentifier: aws.String("test"),
		DBSnapshotArn:        aws.String("arn:aws:rds:us-west-2:123456789012:snapshot:test"),
		KmsKeyId:             aws.String("arn:aws:kms:us-east-1:1234567890:key/123456-7890-123456"),
		Encrypted:            aws.Bool(true),
	}

	rdsc.On("CopyDBSnapshot", mock.Anything).Return(&rds.CopyDBSnapshotOutput{
		DBSnapshot: &rds.DBSnapshot{},
	}, nil)

	err := c.CopySnapshots(input, "us-west-2", "arn:aws:kms:us-east-1:1234567890:key/123456-7890-123456", "https://url.local", "test")
	assert.Nil(t, err)
	rdsc.AssertExpectations(t)

}

func TestGetOldSnapshots(t *testing.T) {
	rdsc := &mockRDS{}

	c := &Client{
		RDS: rdsc,
	}

	time1 := time.Now().AddDate(0, 0, -10)
	time2 := time.Now()

	input := []*rds.DBSnapshot{
		&rds.DBSnapshot{
			Status:               aws.String("available"),
			DBSnapshotIdentifier: aws.String("old-test-1"),
			SnapshotCreateTime:   aws.Time(time1),
		},
		&rds.DBSnapshot{
			Status:               aws.String("available"),
			DBSnapshotIdentifier: aws.String("old-test-2"),
			SnapshotCreateTime:   aws.Time(time2),
		},
	}

	value, err := c.GetOldSnapshots(input, 1)
	assert.Nil(t, err)
	assert.Len(t, value, 1, "Expect one old snapshots")
	rdsc.AssertExpectations(t)
}

func TestDeleteOldSnapshot(t *testing.T) {
	rdsc := &mockRDS{}

	c := &Client{
		RDS: rdsc,
	}

	input := &rds.DBSnapshot{
		DBSnapshotIdentifier: aws.String("old-test-1"),
	}

	rdsc.On("DeleteDBSnapshot", mock.Anything).Return(&rds.DeleteDBSnapshotOutput{
		DBSnapshot: &rds.DBSnapshot{},
	}, nil)

	err := c.DeleteOldSnapshot(input)
	assert.Nil(t, err)
	rdsc.AssertExpectations(t)
}

func TestCheckIfDatabaseSubnetGroupExist(t *testing.T) {
	rdsc := &mockRDS{}

	c := &Client{
		RDS: rdsc,
	}

	input := &rds.DBSnapshot{
		DBSnapshotIdentifier: aws.String("test"),
	}

	rdsc.On("DescribeDBSubnetGroups", mock.Anything).Return(&rds.DescribeDBSubnetGroupsOutput{
		DBSubnetGroups: []*rds.DBSubnetGroup{},
	}, nil)

	value := c.CheckIfDatabaseSubnetGroupExist(input)
	assert.True(t, value)
	rdsc.AssertExpectations(t)
}

func TestCreateDatabaseSubnetGroup(t *testing.T) {
	rdsc := &mockRDS{}

	c := &Client{
		RDS: rdsc,
	}

	subnetids := []string{"subnet-12345", "subnet-6789"}

	input := &rds.DBSnapshot{
		DBSnapshotIdentifier: aws.String("test"),
	}

	rdsc.On("CreateDBSubnetGroup", mock.Anything).Return(&rds.CreateDBSubnetGroupOutput{
		DBSubnetGroup: &rds.DBSubnetGroup{},
	}, nil)

	err := c.CreateDatabaseSubnetGroup(input, subnetids)
	assert.Nil(t, err)
	rdsc.AssertExpectations(t)
}

func TestCreateDBFromSnapshot(t *testing.T) {
	rdsc := &mockRDS{}

	c := &Client{
		RDS: rdsc,
	}

	vpcsecuritygroupids := []string{"sg-12345", "sg6789"}

	input := &rds.DBSnapshot{
		DBInstanceIdentifier: aws.String("instance"),
		DBSnapshotIdentifier: aws.String("test"),
		Engine:               aws.String("postgres"),
		Port:                 aws.Int64(1234),
	}

	rdsc.On("RestoreDBInstanceFromDBSnapshot", mock.Anything).Return(&rds.RestoreDBInstanceFromDBSnapshotOutput{
		DBInstance: &rds.DBInstance{},
	}, nil)

	err := c.CreateDBFromSnapshot(input, "db.t2.micro", vpcsecuritygroupids)
	assert.Nil(t, err)
	rdsc.AssertExpectations(t)
}

func TestDeleteDB(t *testing.T) {
	rdsc := &mockRDS{}

	c := &Client{
		RDS: rdsc,
	}

	input := &rds.DBSnapshot{
		DBInstanceIdentifier: aws.String("instance"),
		DBSnapshotIdentifier: aws.String("test"),
	}

	rdsc.On("DeleteDBInstance", mock.Anything).Return(&rds.DeleteDBInstanceOutput{
		DBInstance: &rds.DBInstance{},
	}, nil)

	err := c.DeleteDB(input)
	assert.Nil(t, err)
	rdsc.AssertExpectations(t)
}

func TestUpdateTag(t *testing.T) {
	rdsc := &mockRDS{}

	c := &Client{
		RDS: rdsc,
	}

	input := &rds.DBSnapshot{
		DBSnapshotArn: aws.String("arn:aws:rds:us-west-2:123456789012:snapshot:test"),
	}

	rdsc.On("RemoveTagsFromResource", mock.Anything).Return(&rds.RemoveTagsFromResourceOutput{}, nil)

	rdsc.On("AddTagsToResource", mock.Anything).Return(&rds.AddTagsToResourceOutput{}, nil)

	err := c.UpdateTag(input, "Status", "restore")
	assert.Nil(t, err)
	rdsc.AssertExpectations(t)
}

func TestCheckTag(t *testing.T) {
	rdsc := &mockRDS{}

	c := &Client{
		RDS: rdsc,
	}

	rdsc.On("ListTagsForResource", mock.Anything).Return(&rds.ListTagsForResourceOutput{
		TagList: []*rds.Tag{
			&rds.Tag{
				Key:   aws.String("Status"),
				Value: aws.String("restore"),
			},
		},
	}, nil)

	value := c.CheckTag("arn:aws:rds:us-west-2:123456789012:snapshot:test", "Status", "restore")
	assert.True(t, value)
	rdsc.AssertExpectations(t)
}

func TestGetDBInstanceInfo(t *testing.T) {
	rdsc := &mockRDS{}

	c := &Client{
		RDS: rdsc,
	}

	input := &rds.DBSnapshot{
		DBInstanceIdentifier: aws.String("instance"),
		DBSnapshotIdentifier: aws.String("test"),
	}

	rdsc.On("DescribeDBInstances", mock.Anything).Return(&rds.DescribeDBInstancesOutput{
		DBInstances: []*rds.DBInstance{
			&rds.DBInstance{
				DBInstanceIdentifier: aws.String("instance-test"),
			},
		},
	}, nil)

	value, err := c.GetDBInstanceInfo(input)
	assert.Nil(t, err)
	assert.Equal(t, *value.DBInstanceIdentifier, "instance-test")
	rdsc.AssertExpectations(t)
}

func TestDeleteDatabaseSubnetGroup(t *testing.T) {
	rdsc := &mockRDS{}

	c := &Client{
		RDS: rdsc,
	}

	input := &rds.DBSnapshot{
		DBSnapshotIdentifier: aws.String("test"),
	}

	rdsc.On("DeleteDBSubnetGroup", mock.Anything).Return(&rds.DeleteDBSubnetGroupOutput{}, nil)

	err := c.DeleteDatabaseSubnetGroup(input)
	assert.Nil(t, err)
	rdsc.AssertExpectations(t)
}

func TestChangeDBpassword(t *testing.T) {
	rdsc := &mockRDS{}

	c := &Client{
		RDS: rdsc,
	}

	input := &rds.DBSnapshot{
		DBSnapshotIdentifier: aws.String("test"),
		DBInstanceIdentifier: aws.String("instance"),
	}

	rdsc.On("ModifyDBInstance", mock.Anything).Return(&rds.ModifyDBInstanceOutput{}, nil)

	rdsc.On("DescribeDBInstances", mock.Anything).Return(&rds.DescribeDBInstancesOutput{
		DBInstances: []*rds.DBInstance{
			&rds.DBInstance{
				DBInstanceIdentifier: aws.String("instance-test"),
				DBInstanceStatus:     aws.String("resetting-master-credentials"),
			},
		},
	}, nil)

	err := c.ChangeDBpassword(input, "arn:aws:rds:us-west-2:123456789012:database:test", "password")
	assert.Nil(t, err)
	rdsc.AssertExpectations(t)
}

func TestGetDBInstanceStatus(t *testing.T) {
	rdsc := &mockRDS{}

	c := &Client{
		RDS: rdsc,
	}

	input := &rds.DBSnapshot{
		DBSnapshotIdentifier: aws.String("test"),
		DBInstanceIdentifier: aws.String("instance"),
	}

	rdsc.On("DescribeDBInstances", mock.Anything).Return(&rds.DescribeDBInstancesOutput{
		DBInstances: []*rds.DBInstance{
			&rds.DBInstance{
				DBInstanceIdentifier: aws.String("instance-test"),
				DBInstanceStatus:     aws.String("available"),
			},
		},
	}, nil)

	value := c.GetDBInstanceStatus(input)
	assert.Equal(t, value, "available")
	rdsc.AssertExpectations(t)
}

func TestGetTagValue(t *testing.T) {
	rdsc := &mockRDS{}

	c := &Client{
		RDS: rdsc,
	}

	rdsc.On("ListTagsForResource", mock.Anything).Return(&rds.ListTagsForResourceOutput{
		TagList: []*rds.Tag{
			&rds.Tag{
				Key:   aws.String("Status"),
				Value: aws.String("restore"),
			},
		},
	}, nil)

	value := c.GetTagValue("arn:aws:rds:us-west-2:123456789012:snapshot:test", "Status")
	assert.Equal(t, value, "restore")
	rdsc.AssertExpectations(t)
}

func TestPreSignUrl(t *testing.T) {
	rdsc := &mockRDS{}

	c := &Client{
		RDS: rdsc,
	}

	u := &url.URL{
		Scheme: "http",
		Host:   "fakeurl.aws.com",
	}

	req := &request.Request{
		HTTPRequest: &http.Request{
			URL: u,
		},
		Operation: &request.Operation{},
	}

	output := &rds.CopyDBSnapshotOutput{}

	rdsc.On("CopyDBSnapshotRequest", mock.Anything).Return(req, output)

	value, err := c.PreSignUrl("us-east-2", "arn:aws:rds:us-west-2:123456789012:snapshot:test", "arn:aws:kms:us-east-1:1234567890:key/123456-7890-123456", "test")
	assert.Nil(t, err)
	assert.Equal(t, value, "http://fakeurl.aws.com")
	rdsc.AssertExpectations(t)
}
