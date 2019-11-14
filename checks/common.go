package checks

import (
	"database/sql"
	"io"
	"io/ioutil"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	_ "github.com/lib/pq"
	"github.com/techdroplabs/rdscheck/config"
	"github.com/techdroplabs/rdscheck/utils"
	"gopkg.in/yaml.v2"
	datadog "gopkg.in/zorkian/go-datadog-api.v2"
)

type DefaultChecks interface {
	SetSessions(region string)
	GetYamlFileFromS3(bucket, key string) (io.Reader, error)
	UnmarshalYamlFile(body io.Reader) (Doc, error)
	dataDogSession(apiKey, applicationKey string) *datadog.Client
	PostDatadogChecks(snapshot *rds.DBSnapshot, metricName, status string) error
	GetSnapshots(DBInstanceIdentifier string) ([]*rds.DBSnapshot, error)
	CopySnapshots(snapshot *rds.DBSnapshot) error
	GetOldSnapshots(snapshots []*rds.DBSnapshot) ([]*rds.DBSnapshot, error)
	DeleteOldSnapshots(snapshots []*rds.DBSnapshot) error
	CheckIfDatabaseSubnetGroupExist(snapshot *rds.DBSnapshot) bool
	CreateDatabaseSubnetGroup(snapshot *rds.DBSnapshot, subnetids []string) error
	CreateDBFromSnapshot(snapshot *rds.DBSnapshot, dbname string, vpcsecuritygroupids []string) error
	DeleteDB(snapshot *rds.DBSnapshot) error
	UpdateTag(snapshot *rds.DBSnapshot, key, value string) error
	CheckTag(arn, key, value string) bool
	GetDBInstanceInfo(snapshot *rds.DBSnapshot) (*rds.DBInstance, error)
	DeleteDatabaseSubnetGroup(snapshot *rds.DBSnapshot) error
	ChangeDBpassword(snapshot *rds.DBSnapshot, DBArn, password string) error
	GetDBInstanceStatus(snapshot *rds.DBSnapshot) string
	GetTagValue(arn, key string) string
	CheckSQLQueries(query, regex string) bool
	InitDb(endpoint rds.Endpoint, user, password, dbname string)
}

type Client struct {
	Datadog   *datadog.Client
	S3        s3iface.S3API
	Snapshots []*rds.DBSnapshot
	RDS       rdsiface.RDSAPI
	DB        *sql.DB
}

type Doc struct {
	Instances []Instances
}

type Instances struct {
	Name     string
	Database string
	Password string
	Queries  []Queries
}

type Queries struct {
	Query string
	Regex string
}

var Status = map[string]datadog.Status{
	"ok":       datadog.OK,
	"warning":  datadog.WARNING,
	"critical": datadog.CRITICAL,
	"unknow":   datadog.UNKNOWN,
}

func New() DefaultChecks {
	return &Client{}
}

// SetSessions init datadog, RDS and S3 sessions
func (c *Client) SetSessions(region string) {
	c.Datadog = c.dataDogSession(config.DDApiKey, config.DDAplicationKey)
	c.S3 = s3.New(AWSSessions(config.DestinationRegion))
	c.RDS = rds.New(AWSSessions(region))
}

// AWSSessions initiate a new aws session
func AWSSessions(region string) *session.Session {
	conf := aws.Config{
		Region: aws.String(region),
	}
	sess := session.Must(session.NewSession(&conf))
	return sess
}

// dataDogSession creates a new datadog session
func (c *Client) dataDogSession(apiKey, applicationKey string) *datadog.Client {
	session := datadog.NewClient(apiKey, applicationKey)
	return session
}

// GetYamlFileFromS3 reads a file from s3 and returns its body
func (c *Client) GetYamlFileFromS3(bucket, key string) (io.Reader, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}
	o, err := c.S3.GetObject(input)
	if err != nil {
		return nil, err
	}
	file := o.Body
	return file, nil
}

// UnmarshalYamlFile unmarshal the yaml file dowmloaded from s3
func (c *Client) UnmarshalYamlFile(body io.Reader) (Doc, error) {
	doc := Doc{}
	yamlFile, err := ioutil.ReadAll(body)
	if err != nil {
		return Doc{}, err
	}
	err = yaml.Unmarshal(yamlFile, &doc)
	if err != nil {
		return Doc{}, err
	}
	return doc, nil
}

// PostDatadogChecks posts to datadog the status of a check
func (c *Client) PostDatadogChecks(snapshot *rds.DBSnapshot, metricName, status string) error {

	tags := []string{
		"database:" + *snapshot.DBInstanceIdentifier,
		"snapshot:" + *snapshot.DBSnapshotIdentifier,
	}

	timeNow := utils.GetUnixTimeAsString()

	m := datadog.Check{}
	m.Check = datadog.String(metricName)
	m.Timestamp = datadog.String(timeNow)
	m.Tags = tags

	if v, ok := Status[status]; ok {
		s := v
		m.Status = &s
	}

	err := c.Datadog.PostCheck(m)
	if err != nil {
		return err
	}
	return nil
}
