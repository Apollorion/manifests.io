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
}

data "aws_acm_certificate" "main" {
  domain      = local.tld
  statuses    = ["ISSUED"]
  types       = ["AMAZON_ISSUED"]
  most_recent = true
}

locals {
  prod_tld  = "manifests.io"
  stage_tld = "stage.manifests.io"

  prod_api_tld  = "api.manifests.io"
  stage_api_tld = "api.stage.manifests.io"

  api_tld = terraform.workspace == "production" ? local.prod_api_tld : local.stage_api_tld
  tld     = terraform.workspace == "production" ? local.prod_tld : local.stage_tld

  statuspage_url = "https://stats.uptimerobot.com/RrvD5UG7yP"

  website_files = tolist(fileset("../app/build/", "**"))
}