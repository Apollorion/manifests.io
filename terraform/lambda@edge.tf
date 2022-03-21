data "aws_iam_policy_document" "assume_role_policy_doc_edge" {
  statement {
    sid    = "AllowAwsToAssumeRole"
    effect = "Allow"

    actions = ["sts:AssumeRole"]

    principals {
      type = "Service"

      identifiers = [
        "edgelambda.amazonaws.com",
        "lambda.amazonaws.com",
      ]
    }
  }
}

resource "aws_iam_role" "edge" {
  name               = "manifests_io_edge_${terraform.workspace}"
  assume_role_policy = data.aws_iam_policy_document.assume_role_policy_doc_edge.json
}

resource "aws_iam_role_policy_attachment" "edge" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
  role       = aws_iam_role.edge.name
}

resource "aws_iam_role_policy_attachment" "edge_vpc_access" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaVPCAccessExecutionRole"
  role       = aws_iam_role.edge.name
}

data "archive_file" "edge" {
  type        = "zip"
  source_dir  = "${path.module}/../lambda@edgefunction/"
  output_path = "${path.module}/edge_payload.zip"
}

resource "aws_lambda_function" "edge" {
  filename         = data.archive_file.edge.output_path
  function_name    = "manifests_io_edge_${terraform.workspace}"
  role             = aws_iam_role.edge.arn
  handler          = "main.handler"
  source_code_hash = data.archive_file.edge.output_base64sha256
  timeout          = 5
  memory_size      = 128

  publish = true

  runtime = "python3.9"

  lifecycle {
    ignore_changes = [
      last_modified
    ]
  }
}