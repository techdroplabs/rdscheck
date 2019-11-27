resource "aws_iam_role" "rdscheck_iam_role" {
  name = "rdscheck_${var.command}_role"

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
    command = "rm -rf ${path.module}/lambda-files && mkdir ${path.module}/lambda-files && wget -O ${path.module}/lambda-files/main https://github.com/techdroplabs/rdscheck/releases/download/${var.release_version}/${var.command}"
  }

  # We do that so null_resource is called everytime we run terraform apply or plan
  triggers = {
    always_run = "${timestamp()}"
  }
}

data "archive_file" "lambda_code" {
  type        = "zip"
  source_file = "${path.module}/lambda-files/main"
  output_path = "${path.module}/lambda-files/main.zip"
  depends_on  = ["null_resource.get_release"]
}

resource "aws_lambda_function" "rdscheck_lambda" {
  filename         = "${data.archive_file.lambda_code.output_path}"
  function_name    = "${var.command}-rdscheck"
  role             = "${aws_iam_role.rdscheck_iam_role.arn}"
  handler          = "main"
  source_code_hash = "${data.archive_file.lambda_code.output_base64sha256}"
  runtime          = "go1.x"
  memory_size      = 128
  timeout          = 120
  environment      = ["${slice(list(var.lambda_env_vars), 0, length(var.lambda_env_vars) == 0 ? 0 : 1)}"]
}

data "aws_iam_policy" "AWSLambdaVPCAccessExecutionRole" {
  count = "${var.command != "copy" ? 1 : 0}"
  arn   = "arn:aws:iam::aws:policy/AWSLambdaVPCAccessExecutionRole"
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
  count      = "${var.command != "copy" ? 1 : 0}"
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
  name                = "rdscheck_${var.command}_rule"
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

variable "command" {}

variable "lambda_env_vars" {
  type    = "map"
  default = {}
}
