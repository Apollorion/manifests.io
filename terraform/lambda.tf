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

resource "aws_lambda_function" "api" {
  filename         = "../payload.zip"
  function_name    = "manifests_io_api_${terraform.workspace}"
  role             = aws_iam_role.api.arn
  handler          = "main.handler"
  source_code_hash = filebase64sha256("../payload.zip")
  timeout          = 10
  memory_size      = 512

  runtime = "python3.9"

  environment {
    variables = {
      SENTRY_INGEST      = local.secrets["SENTRY_INGEST"]
      SENTRY_ENVIRONMENT = terraform.workspace
    }
  }
}