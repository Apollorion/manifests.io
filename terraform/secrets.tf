data "aws_secretsmanager_secret" "main" {
  name = "manifests-io-${terraform.workspace}"
}

data "aws_secretsmanager_secret_version" "main" {
  secret_id = data.aws_secretsmanager_secret.main.id
}

locals {
  secrets = jsondecode(data.aws_secretsmanager_secret_version.main.secret_string)
}