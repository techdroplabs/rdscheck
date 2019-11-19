# rdscheck
+ Copy command will:
    - Copy snapshot(s) to a different AWS region in the same account
    - Cleanup old snapshots based on retention setup in the yaml config file
+ Check command will:
    - Creates new rds instance(s) with the snapshots
    - Runs a set of queries on the database to validate the content of the backup


## check: state machine diagram

![state machine](/img/state-machine.png)

## yaml configuration file

+ instances: `all the rds instances that we want to copy/restore/check to an AWS region.`
    - name: `the name of the source rds instance`
    - database: `the name of the databse that we copied and restored we use this field to initiate the db connection`
    - type: `the rds instance type we want to use to restore the snapshot`
    - password: `the password that we will use to connect to the database. It doesn't need to be the original one. We will use this one to reset the original password`
    - retention: `how many days we want to keep the copied snapshot around`
    - destination: `the aws region where we will copy/restore the snapshot`
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
  source = git::git@github.com:techdroplabs/rdscheck.git//terraform/copy?ref=v0.0.1

  lambda_rate = "rate(1 day)
  release_version = "v0.0.1"
  command_name = "copy"
  s3_bucket = "s3-bucket-with-yaml-file"
  s3_key = "rdscheck.yaml"
  aws_region_source = "us-west-2"
  dd_api_key = "lked78t4iuhweoih8oi"
  dd_app_key = "lknsdc8754liwhefp90"
}

```

```hcl

module "rdscheck-check" {
  source = git::git@github.com:techdroplabs/rdscheck.git//terraform/check?ref=v0.0.1

  lambda_rate = "rate(30 minutes)
  release_version = "v0.0.1"
  command_name = "copy"
  s3_bucket = "s3-bucket-with-yaml-file"
  s3_key = "rdscheck.yaml"
  aws_region_source = "us-west-2"
  aws_sg_ids = "sg-1234,sg-5678"
  aws_subnets_ids = "subnet-qwerty1234576,subnet-azerty123456"
  dd_api_key = "lked78t4iuhweoih8oi"
  dd_app_key = "lknsdc8754liwhefp90"
}

```
