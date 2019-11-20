resource "aws_iam_role" "rdscheck_iam_role" {
  name = "rdscheck_copy_role"

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
    command = "wget -O check.zip https://github.com/techdroplabs/rdscheck/releases/download/${var.release_version}/copy.zip"
  }
  # We do that so null_resource is called everytime we run terraform apply or plan
  triggers = {
    always_run = "${timestamp()}"
  }
}

resource "aws_lambda_function" "rdscheck_lambda" {
  filename         = "copy.zip"
  function_name    = "copy-rdscheck"
  role             = "${aws_iam_role.rdscheck_iam_role.arn}"
  handler          = "main"
  source_code_hash = "${base64sha256(file("copy.zip"))}"
  runtime          = "go1.x"
  memory_size      = 128
  timeout          = 60
  environment {
    variables = {
      S3_BUCKET         = "${var.s3_bucket}"
      S3_KEY            = "${var.s3_key}"
      AWS_REGION_SOURCE = "${var.aws_region_source}"
      DD_API_KEY        = "${var.dd_api_key}"
      DD_APP_KEY        = "${var.dd_app_key}"
    }
  }
  depends_on = ["${null_resource.get_release}"]
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
  name                = "rdscheck_copy_rule"
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
  default = "rate(1 day)"
}

variable "release_version" {}

variable "s3_bucket" {}

variable "s3_key" {}

variable "aws_region_source" {}

variable "dd_api_key" {}

variable "dd_app_key" {}