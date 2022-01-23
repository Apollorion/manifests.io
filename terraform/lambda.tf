variable "REDIS_HOST" {
  type = string
  sensitive = true
}

resource "aws_iam_role" "api" {
  name = "manifests_io_api"

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
  role = aws_iam_role.api.name
}

resource "aws_lambda_function" "api" {
  filename      = "../compiled/api_payload.zip"
  function_name = "manifests_io_api"
  role          = aws_iam_role.api.arn
  handler       = "handler.handler"
  source_code_hash = filebase64sha256("../payload.zip")
  timeout = 10

  runtime = "python3.9"

  environment {
    variables = {
      REDIS_HOST = var.REDIS_HOST
    }
  }
}