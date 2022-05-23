provider "aws" {
  region = "us-east-1"
}

provider "archive" {}

terraform {
  backend "s3" {
    bucket = "apollorion-us-east-1-tfstates"
    key    = "manifests.io.tfstate"
    region = "us-east-1"
  }
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
    template = {
      source = "hashicorp/template"
    }
  }
}

data "aws_acm_certificate" "main" {
  domain      = "manifests.io"
  statuses    = ["ISSUED"]
  types       = ["AMAZON_ISSUED"]
  most_recent = true
}

locals {
  prod_tld  = "manifests.io"
  stage_tld = "stage.manifests.io"

  prod_api_tld  = "api.manifests.io"
  stage_api_tld = "api-stage.manifests.io"

  api_tld = terraform.workspace == "production" ? local.prod_api_tld : local.stage_api_tld
  tld     = terraform.workspace == "production" ? local.prod_tld : local.stage_tld
}