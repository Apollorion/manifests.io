resource "aws_lambda_function" "lambda_load" {
  count            = terraform.workspace == "production" ? 1 : 0
  filename         = "../lambda_load_payload.zip"
  function_name    = "manifests_io_api_lambda_load_production"
  role             = aws_iam_role.api.arn
  handler          = "load_redis.lambda_handler"
  source_code_hash = filebase64sha256("../lambda_load_payload.zip")
  timeout          = 900
  memory_size      = 1024

  runtime = "python3.9"

  environment {
    variables = {
      REDIS_HOST = module.redis[count.index].endpoint,
    }
  }

  vpc_config {
    subnet_ids         = module.subnets[count.index].private_subnet_ids
    security_group_ids = [module.sg[count.index].id]
  }
}