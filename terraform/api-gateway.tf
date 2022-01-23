resource "aws_api_gateway_rest_api" "manifests_io" {
  name        = "manifests_io"
  description = "manifests_io API"
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
    aws_api_gateway_method.k8Version
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