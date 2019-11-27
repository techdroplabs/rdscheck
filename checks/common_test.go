package checks

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	datadog "github.com/zorkian/go-datadog-api"
	"gopkg.in/h2non/gock.v1"
)

type mockS3 struct {
	s3iface.S3API
	mock.Mock
}

func (m *mockS3) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*s3.GetObjectOutput), args.Error(1)
}

func TestGetYamlFileFromS3(t *testing.T) {
	s3c := &mockS3{}

	c := &Client{
		S3: s3c,
	}

	input, _ := os.Open("../example/checks.yml")
	r := ioutil.NopCloser(input)
	output := &s3.GetObjectOutput{
		Body: r,
	}

	s3c.On("GetObject", mock.Anything).Return(output, nil)

	values, err := c.GetYamlFileFromS3("test-bucket", "checks.yml")
	assert.Nil(t, err)
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(values)
	valuesToString := buf.String()
	assert.Contains(t, valuesToString, "thisisatest")
	s3c.AssertExpectations(t)
}

func TestUnmarshalYamlFile(t *testing.T) {
	input, _ := os.Open("../example/checks.yml")
	r := ioutil.NopCloser(input)

	c := &Client{}

	doc := Doc{
		Instances: []Instances{
			Instances{
				Name:        "rdscheck",
				Database:    "rdscheck",
				Type:        "db.t2.micro",
				Password:    "thisisatest",
				Retention:   1,
				Destination: "us-east-1",
				KmsID:       "arn:aws:kms:us-east-1:1234567890:key/123456-7890-123456",
				Queries: []Queries{
					Queries{
						Query: "SELECT tablename FROM pg_catalog.pg_tables;",
						Regex: "^pg_statistic$",
					},
				},
			},
			Instances{
				Name:        "rdscheck2",
				Database:    "rdscheck2",
				Type:        "db.t2.micro",
				Password:    "thisisatest",
				Retention:   10,
				Destination: "us-east-2",
				Queries: []Queries{
					Queries{
						Query: "SELECT tablename FROM pg_catalog.pg_tables;",
						Regex: "^pg_statistic$",
					},
				},
			},
		},
	}

	value, err := c.UnmarshalYamlFile(r)
	assert.Nil(t, err)
	assert.Equal(t, value, doc)
}

func TestPostDatadogChecks(t *testing.T) {
	defer gock.Off()

	gock.New("http://test.local").
		Post("/v1/check_run").
		Reply(200).
		JSON(map[string]string{"status": "ok"})

	os.Setenv("DATADOG_HOST", "http://test.local")
	defer os.Unsetenv("DATADOG_HOST")

	dd := datadog.NewClient("", "")

	c := &Client{
		Datadog: dd,
	}

	input := &rds.DBSnapshot{
		DBInstanceIdentifier: aws.String("instance"),
		DBSnapshotIdentifier: aws.String("test"),
	}

	err := c.PostDatadogChecks(input, "rdscheck.status", "ok")
	assert.Nil(t, err)
}

func TestCleanArn(t *testing.T) {
	c := &Client{}

	input := &rds.DBSnapshot{
		DBSnapshotArn: aws.String("arn:aws:rds:us-west-2:123456789012:snapshot:test"),
	}

	value := c.CleanArn(input)
	assert.Equal(t, value, "test")
}
