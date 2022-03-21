resource "aws_iam_role" "api" {
  name = "manifests_io_api_${terraform.workspace}"

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

resource "aws_iam_role_policy_attachment" "api" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
  role       = aws_iam_role.api.name
}

resource "aws_iam_role_policy_attachment" "api_vpc_access" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaVPCAccessExecutionRole"
  role       = aws_iam_role.api.name
}

data "archive_file" "api" {
  type        = "zip"
  source_dir  = "${path.module}/../lambda/"
  output_path = "${path.module}/api_payload.zip"
}

resource "aws_lambda_function" "api" {
  filename         = data.archive_file.api.output_path
  function_name    = "manifests_io_api_${terraform.workspace}"
  role             = aws_iam_role.api.arn
  handler          = "main.handler"
  source_code_hash = data.archive_file.api.output_base64sha256
  timeout          = 10
  memory_size      = 512

  runtime = "python3.9"

  environment {
    variables = {
      SENTRY_INGEST      = local.secrets["SENTRY_INGEST"]
      SENTRY_ENVIRONMENT = terraform.workspace
    }
  }

  lifecycle {
    ignore_changes = [
      last_modified
    ]
  }
}