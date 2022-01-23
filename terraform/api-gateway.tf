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

  depends_on = [
    aws_api_gateway_method.latest,
    aws_api_gateway_method.k8Version,
    aws_api_gateway_method.fieldPath,
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

resource "aws_api_gateway_resource" "k8Version" {
  parent_id   = aws_api_gateway_rest_api.manifests_io.root_resource_id
  path_part   = "{k8Version}"
  rest_api_id = aws_api_gateway_rest_api.manifests_io.id
}

resource "aws_api_gateway_integration" "k8Version" {
  rest_api_id             = aws_api_gateway_rest_api.manifests_io.id
  resource_id             = aws_api_gateway_resource.k8Version.id
  http_method             = aws_api_gateway_method.k8Version.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.api.invoke_arn
  content_handling        = "CONVERT_TO_TEXT"
}

resource "aws_api_gateway_method" "k8Version" {
  rest_api_id   = aws_api_gateway_rest_api.manifests_io.id
  http_method   = "GET"
  authorization = "NONE"
  resource_id   = aws_api_gateway_resource.k8Version.id
  request_parameters = {
    "method.request.path.k8Version" = true
  }
}

resource "aws_api_gateway_resource" "fieldPath" {
  parent_id   = aws_api_gateway_resource.k8Version.id
  path_part   = "{fieldPath}"
  rest_api_id = aws_api_gateway_rest_api.manifests_io.id
}

resource "aws_api_gateway_integration" "fieldPath" {
  rest_api_id             = aws_api_gateway_rest_api.manifests_io.id
  resource_id             = aws_api_gateway_resource.fieldPath.id
  http_method             = aws_api_gateway_method.fieldPath.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.api.invoke_arn
  content_handling        = "CONVERT_TO_TEXT"
}

resource "aws_api_gateway_method" "fieldPath" {
  rest_api_id   = aws_api_gateway_rest_api.manifests_io.id
  http_method   = "GET"
  authorization = "NONE"
  resource_id   = aws_api_gateway_resource.fieldPath.id
  request_parameters = {
    "method.request.path.fieldPath" = true
  }
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