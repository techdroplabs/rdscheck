resource "aws_iam_role" "rdscheck_iam_role" {
  name = "rdscheck_check_role"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "null_resource" "get_release" {
  provisioner "local-exec" {
    command = "wget -O check.zip https://github.com/techdroplabs/rdscheck/releases/download/${var.release_version}/check.zip"
  }
  # We do that so null_resource is called everytime we run terraform apply or plan
  triggers = {
    always_run = "${timestamp()}"
  }
}

resource "aws_lambda_function" "rdscheck_lambda" {
  filename         = "check.zip"
  function_name    = "check-rdscheck"
  role             = "${aws_iam_role.rdscheck_iam_role.arn}"
  handler          = "main"
  source_code_hash = "${base64sha256(file("check.zip"))}"
  runtime          = "go1.x"
  memory_size      = 128
  timeout          = 120
  environment {
    variables = {
      S3_BUCKET         = "${var.s3_bucket}"
      S3_KEY            = "${var.s3_key}"
      AWS_REGION_SOURCE = "${var.aws_region_source}"
      AWS_SG_IDS        = "${var.aws_sg_ids}"
      AWS_SUBNETS_IDS   = "${var.aws_subnets_ids}"
      DD_API_KEY        = "${var.dd_api_key}"
      DD_APP_KEY        = "${var.dd_app_key}"
    }
  }
  depends_on = ["${null_resource.get_command_release}"]
}

data "aws_iam_policy" "AWSLambdaVPCAccessExecutionRole" {
  arn = "arn:aws:iam::aws:policy/AWSLambdaVPCAccessExecutionRole"
}

data "aws_iam_policy" "CloudWatchFullAccess" {
  arn = "arn:aws:iam::aws:policy/CloudWatchFullAccess"
}

data "aws_iam_policy" "AmazonS3ReadOnlyAccess" {
  arn = "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"
}

data "aws_iam_policy" "AmazonRDSFullAccess" {
  arn = "arn:aws:iam::aws:policy/AmazonRDSFullAccess"
}

resource "aws_iam_role_policy_attachment" "rdscheck_role_AWSLambdaVPCAccessExecutionRole_policy_attach" {
  role       = "${aws_iam_role.rdscheck_iam_role.name}"
  policy_arn = "${data.aws_iam_policy.AWSLambdaVPCAccessExecutionRole.arn}"
}

resource "aws_iam_role_policy_attachment" "rdscheck_role_CloudWatchFullAccess_policy_attach" {
  role       = "${aws_iam_role.rdscheck_iam_role.name}"
  policy_arn = "${data.aws_iam_policy.CloudWatchFullAccess.arn}"
}

resource "aws_iam_role_policy_attachment" "rdscheck_role_AmazonS3ReadOnlyAccess_policy_attach" {
  role       = "${aws_iam_role.rdscheck_iam_role.name}"
  policy_arn = "${data.aws_iam_policy.AmazonS3ReadOnlyAccess.arn}"
}

resource "aws_iam_role_policy_attachment" "rdscheck_role_AmazonRDSFullAccess_policy_attach" {
  role       = "${aws_iam_role.rdscheck_iam_role.name}"
  policy_arn = "${data.aws_iam_policy.AmazonRDSFullAccess.arn}"
}

resource "aws_cloudwatch_event_rule" "rdscheck_rule" {
  name                = "rdscheck_check_rule"
  schedule_expression = "${var.lambda_rate}"
  is_enabled          = true
}

resource "aws_cloudwatch_event_target" "rdscheck_target" {
  rule = "${aws_cloudwatch_event_rule.rdscheck_rule.name}"
  arn  = "${aws_lambda_function.rdscheck_lambda.arn}"
}

resource "aws_lambda_permission" "allow_cloudwatch_to_call_rdscheck" {
  statement_id  = "AllowExecutionFromCloudWatch"
  action        = "lambda:InvokeFunction"
  function_name = "${aws_lambda_function.rdscheck_lambda.function_name}"
  principal     = "events.amazonaws.com"
  source_arn    = "${aws_cloudwatch_event_rule.rdscheck_rule.arn}"
}

variable "lambda_rate" {
  default = "rate(30 minutes)"
}

variable "release_version" {}

variable "s3_bucket" {}

variable "s3_key" {}

variable "aws_region_source" {}

variable "aws_sg_ids" {}

variable "aws_subnets_ids" {}

variable "dd_api_key" {}

variable "dd_app_key" {}
