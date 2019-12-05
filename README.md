# rdscheck
+ Copy command will:
    - Copy snapshot(s) to a different AWS region in the same account. This will copy only automated snapshots.
    - Cleanup old snapshots based on retention setup in the yaml config file
+ Check command will:
    - Creates new rds instance(s) with the snapshots
    - Runs a set of queries on the database to validate the content of the backup

## TODO

- Handle things gracefully when there is more than 5 snapshots to copy
- Handle different retentions between automatic and manual backups. (tag automatic snapshot with something like "CopiedBy" "rdscheck" and skip if set)

## check: state machine diagram

![state machine](/img/state-machine.png)

## yaml configuration file

+ instances: `all the rds instances that we want to copy/restore/check to an AWS region.`
    - name: `the name of the source rds instance`
    - database: `the name of the databse that we copied and restored we use this field to initiate the db connection`
    - type: `the rds instance type we want to use to restore the snapshot`
    - password: `the password that we will use to connect to the database. It doesn't need to be the original one. We will use this one to reset the original password`
    - retention: `how many days we want to keep the copied snapshot around. Right now it should be equal to the number of days the automatic backups are kept`
    - destination: `the aws region where we will copy/restore the snapshot`
    - kmsid: `the id (ARN) of the kms key that you want to use on the destination region. This is needed if your original snapshot is encrypted`
    - queries: `all the sql queries we want to run on the restored snapshot to validate it and the expected results as regex`
      - query: `the sql query to run`
      - regex: `the regex of the expected result`

Example:
```
instances:
  - name: rdscheck
    database: rdscheck
    type: db.t2.micro
    password: thisisatest
    retention: 1
    destination: us-east-1
    kmsid: "arn:aws:kms:us-east-1:1234567890:key/123456-7890-123456"
    queries:
      - query: "SELECT tablename FROM pg_catalog.pg_tables;"
        regex: "^pg_statistic$"
  - name: rdscheck2
    database: rdscheck
    type: db.t2.micro
    password: thisisatest
    retention: 10
    destination: us-east-2
    queries:
      - query: "SELECT tablename FROM pg_catalog.pg_tables;"
        regex: "^pg_statistic$"
```

## Releases

Github Workflow is setup to create a new release when a tag is created and pushed.
[.github/workflows/release.yml](.github/workflows/release.yml) will get triggered, will create a new release, build the commands and upload them as two seperate zip files in the release.
By doing so we can then download the command zip file for a release and use it when creating a lambda function with terraform.

## Terraform

```hcl

module "rdscheck-copy" {
  source = "github.com/techdroplabs/rdscheck//terraform?ref=v0.0.9"

  release_version = "v0.0.8"
  command = "copy"
  lambda_env_vars {
    variables = {
      S3_BUCKET         = "s3-bucket-with-yaml-file"
      S3_KEY            = "rdscheck.yml"
      AWS_REGION_SOURCE = "us-west-2"
      DD_API_KEY        = "lked78t4iuhweoih8oi"
      DD_APP_KEY        = "lknsdc8754liwhefp90"
    }
  }
}

```

```hcl

module "rdscheck-check" {
  source = "github.com/techdroplabs/rdscheck//terraform?ref=v0.0.9"

  lambda_rate = "rate(30 minutes)"
  release_version = "v0.0.8"
  command = "check"
  subnet_ids = ["subnet-12345,subnet-6789"]
  security_group_ids = ["sg-1234,sg-5678"]
  lambda_env_vars {
    variables = {
      S3_BUCKET         = "s3-bucket-with-yaml-file"
      S3_KEY            = "rdscheck.yml"
      AWS_REGION_SOURCE = "us-west-2"
      AWS_SG_IDS        = "sg-1234,sg-5678"
      AWS_SUBNETS_IDS   = "subnet-qwerty1234576,subnet-azerty123456"
      DD_API_KEY        = "lked78t4iuhweoih8oi"
      DD_APP_KEY        = "lknsdc8754liwhefp90"
    }
  }
}

```
