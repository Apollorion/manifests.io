variable path_part {
  type = string
}

variable parent_id {
  type = string
}

variable rest_api_id {
  type = string
}

variable lambda_invoke_arn {
  type = string
}

variable http_methods {
  type = list(string)
}

resource "aws_api_gateway_resource" "MnI" {
  rest_api_id = var.rest_api_id
  parent_id   = var.parent_id
  path_part   = var.path_part
}

resource "aws_api_gateway_method" "MnI" {
  count         = length(var.http_methods)

  rest_api_id   = var.rest_api_id
  http_method   = var.http_methods[count.index]
  resource_id   = aws_api_gateway_resource.MnI.id
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "MnI" {
  count                   = length(var.http_methods)
  rest_api_id             = var.rest_api_id
  resource_id             = aws_api_gateway_resource.MnI.id
  http_method             = aws_api_gateway_method.MnI[count.index].http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = var.lambda_invoke_arn
  content_handling        = "CONVERT_TO_TEXT"
}

output resource_id {
  value = aws_api_gateway_resource.MnI.id
}