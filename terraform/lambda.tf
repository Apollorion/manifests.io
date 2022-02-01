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

  # Environment variables for production
  dynamic "environment" {
    for_each = terraform.workspace == "production" ? [0] : []
    content {
      variables = {
        REDIS_HOST         = module.redis[environment.value].endpoint,
        SENTRY_INGEST      = local.secrets["SENTRY_INGEST"]
        SENTRY_ENVIRONMENT = "production"
      }
    }
  }

  # Environment variables for staging
  dynamic "environment" {
    for_each = terraform.workspace == "staging" ? [0] : []
    content {
      variables = {
        SENTRY_INGEST      = local.secrets["SENTRY_INGEST"]
        SENTRY_ENVIRONMENT = "staging"
      }
    }
  }

  dynamic "vpc_config" {
    for_each = terraform.workspace == "production" ? [0] : []
    content {
      subnet_ids         = module.subnets[vpc_config.value].private_subnet_ids
      security_group_ids = [module.sg[vpc_config.value].id]
    }
  }
}