resource "aws_api_gateway_rest_api" "manifests_io" {
  name        = "manifests_io_${terraform.workspace}"
  description = "manifests_io ${terraform.workspace} API"
  endpoint_configuration {
    types = ["EDGE"]
  }
}

resource "aws_api_gateway_stage" "main" {
  deployment_id = aws_api_gateway_deployment.main.id
  rest_api_id   = aws_api_gateway_rest_api.manifests_io.id
  stage_name    = "main"
}

resource "aws_api_gateway_deployment" "main" {
  rest_api_id = aws_api_gateway_rest_api.manifests_io.id

  variables = {
    deployed_at = timestamp()
  }

  lifecycle {
    create_before_destroy = true
  }

  depends_on = [
    aws_api_gateway_integration.latest,
    module.keys_keys_RMI,
    module.keys_fieldPath_RMI,
    module.keys_k8sVersion_RMI,
    module.search_search_RMI,
    module.search_k8sVersion_RMI,
    module.search_fieldPath_RMI,
    module.resources_resources_RMI,
    module.resources_k8sVersion_RMI,
    module.examples_examples_RMI,
    module.examples_fieldPath_RMI,
    module.examples_k8sVersion_RMI,
  ]
}

resource "aws_lambda_permission" "manifests_io" {
  statement_id  = "AllowExecutionFromAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.api.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.manifests_io.execution_arn}/*/*/*"
}

resource "aws_api_gateway_integration" "latest" {
  rest_api_id             = aws_api_gateway_rest_api.manifests_io.id
  resource_id             = aws_api_gateway_rest_api.manifests_io.root_resource_id
  http_method             = aws_api_gateway_method.latest.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.api.invoke_arn
  content_handling        = "CONVERT_TO_TEXT"
}

resource "aws_api_gateway_method" "latest" {
  rest_api_id   = aws_api_gateway_rest_api.manifests_io.id
  resource_id   = aws_api_gateway_rest_api.manifests_io.root_resource_id
  http_method   = "GET"
  authorization = "NONE"
}

module "search_search_RMI" {
  source            = "./modules/RMI"
  http_methods      = ["GET"]
  lambda_invoke_arn = aws_lambda_function.api.invoke_arn
  parent_id         = aws_api_gateway_rest_api.manifests_io.root_resource_id
  path_part         = "search"
  rest_api_id       = aws_api_gateway_rest_api.manifests_io.id
}

module "search_k8sVersion_RMI" {
  source            = "./modules/RMI"
  http_methods      = ["GET"]
  lambda_invoke_arn = aws_lambda_function.api.invoke_arn
  parent_id         = module.search_search_RMI.resource_id
  path_part         = "{k8sVersion}"
  rest_api_id       = aws_api_gateway_rest_api.manifests_io.id
}

module "search_fieldPath_RMI" {
  source            = "./modules/RMI"
  http_methods      = ["GET"]
  lambda_invoke_arn = aws_lambda_function.api.invoke_arn
  parent_id         = module.search_k8sVersion_RMI.resource_id
  path_part         = "{fieldPath}"
  rest_api_id       = aws_api_gateway_rest_api.manifests_io.id
}

module "resources_resources_RMI" {
  source            = "./modules/RMI"
  http_methods      = ["GET"]
  lambda_invoke_arn = aws_lambda_function.api.invoke_arn
  parent_id         = aws_api_gateway_rest_api.manifests_io.root_resource_id
  path_part         = "resources"
  rest_api_id       = aws_api_gateway_rest_api.manifests_io.id
}

module "resources_k8sVersion_RMI" {
  source            = "./modules/RMI"
  http_methods      = ["GET"]
  lambda_invoke_arn = aws_lambda_function.api.invoke_arn
  parent_id         = module.resources_resources_RMI.resource_id
  path_part         = "{k8sVersion}"
  rest_api_id       = aws_api_gateway_rest_api.manifests_io.id
}

module "keys_keys_RMI" {
  source            = "./modules/RMI"
  http_methods      = ["GET"]
  lambda_invoke_arn = aws_lambda_function.api.invoke_arn
  parent_id         = aws_api_gateway_rest_api.manifests_io.root_resource_id
  path_part         = "keys"
  rest_api_id       = aws_api_gateway_rest_api.manifests_io.id
}

module "keys_k8sVersion_RMI" {
  source            = "./modules/RMI"
  http_methods      = ["GET"]
  lambda_invoke_arn = aws_lambda_function.api.invoke_arn
  parent_id         = module.keys_keys_RMI.resource_id
  path_part         = "{k8sVersion}"
  rest_api_id       = aws_api_gateway_rest_api.manifests_io.id
}

module "keys_fieldPath_RMI" {
  source            = "./modules/RMI"
  http_methods      = ["GET"]
  lambda_invoke_arn = aws_lambda_function.api.invoke_arn
  parent_id         = module.keys_k8sVersion_RMI.resource_id
  path_part         = "{fieldPath}"
  rest_api_id       = aws_api_gateway_rest_api.manifests_io.id
}

module "examples_examples_RMI" {
  source            = "./modules/RMI"
  http_methods      = ["GET"]
  lambda_invoke_arn = aws_lambda_function.api.invoke_arn
  parent_id         = aws_api_gateway_rest_api.manifests_io.root_resource_id
  path_part         = "examples"
  rest_api_id       = aws_api_gateway_rest_api.manifests_io.id
}

module "examples_k8sVersion_RMI" {
  source            = "./modules/RMI"
  http_methods      = ["GET"]
  lambda_invoke_arn = aws_lambda_function.api.invoke_arn
  parent_id         = module.examples_examples_RMI.resource_id
  path_part         = "{k8sVersion}"
  rest_api_id       = aws_api_gateway_rest_api.manifests_io.id
}

module "examples_fieldPath_RMI" {
  source            = "./modules/RMI"
  http_methods      = ["GET"]
  lambda_invoke_arn = aws_lambda_function.api.invoke_arn
  parent_id         = module.examples_k8sVersion_RMI.resource_id
  path_part         = "{fieldPath}"
  rest_api_id       = aws_api_gateway_rest_api.manifests_io.id
}

resource "aws_api_gateway_domain_name" "api" {
  certificate_arn = data.aws_acm_certificate.main.arn
  domain_name     = local.api_tld
}

resource "aws_route53_record" "api" {
  name    = aws_api_gateway_domain_name.api.domain_name
  type    = "A"
  zone_id = data.aws_route53_zone.manifests_io.zone_id

  alias {
    evaluate_target_health = true
    name                   = aws_api_gateway_domain_name.api.cloudfront_domain_name
    zone_id                = aws_api_gateway_domain_name.api.cloudfront_zone_id
  }
}

resource "aws_api_gateway_base_path_mapping" "apie" {
  api_id      = aws_api_gateway_rest_api.manifests_io.id
  stage_name  = aws_api_gateway_stage.main.stage_name
  domain_name = aws_api_gateway_domain_name.api.domain_name
}