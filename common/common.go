package common

import (
	"io"
	"io/ioutil"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/s3"
	log "github.com/sirupsen/logrus"
	"github.com/techdroplabs/rdscheck/utils"
	"gopkg.in/yaml.v2"
	datadog "gopkg.in/zorkian/go-datadog-api.v2"
)

// DBInstance is our primary structure
type DBInstance struct {
	RDS       *rds.RDS
	Snapshots []*rds.DBSnapshot
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

// AWSSessions initiate a new aws session
func AWSSessions(region string) *session.Session {
	conf := aws.Config{
		Region: aws.String(region),
	}
	sess := session.Must(session.NewSession(&conf))
	return sess
}

func UnmarshalYamlFile(body io.Reader) (Doc, error) {
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

func DataDogSession(apiKey, applicationKey string) *datadog.Client {
	ddsess := datadog.NewClient(apiKey, applicationKey)
	return ddsess
}

func PostDatadogChecks(c *datadog.Client, metricName, status string, s *rds.DBSnapshot) {

	tags := []string{
		"database:" + *s.DBInstanceIdentifier,
		"snapshot:" + *s.DBSnapshotIdentifier,
	}

	ddStatus := map[string]datadog.Status{
		"ok":       datadog.OK,
		"warning":  datadog.WARNING,
		"critical": datadog.CRITICAL,
		"unknow":   datadog.UNKNOWN,
	}

	timeNow := utils.GetUnixTimeAsString()

	m := datadog.Check{}
	m.Check = datadog.String(metricName)
	m.Timestamp = datadog.String(timeNow)
	m.Tags = tags

	if v, ok := ddStatus[status]; ok {
		s := v
		m.Status = &s
	}

	err := c.PostCheck(m)
	if err != nil {
		log.Errorf("Could post Datadog Check: %s", err)
	}
}

func GetYamlFileFromS3(s *s3.S3, bucket, key string) (io.Reader, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}
	o, err := s.GetObject(input)
	if err != nil {
		return nil, err
	}
	file := o.Body
	return file, nil
}
