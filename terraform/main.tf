provider "aws" {
  region = "us-east-1"
  profile = "apollorion"
}

terraform {
  backend "s3" {
    bucket = "apollorion-us-east-1-tfstates"
    key    = "manifests.io.tfstate"
    region = "us-east-1"
  }
}

locals {
  prod_tld  = "manifests.io"
  prod_cert = "arn:aws:acm:us-east-1:874575230586:certificate/9bb1e798-cd45-4f68-a542-379b0e6459c9"

  stage_tld  = "stage.manifests.io"
  stage_cert = "arn:aws:acm:us-east-1:874575230586:certificate/e414cda4-1c68-4c6c-8f10-06ab396e8546"

  prod_api_tld = "api.manifests.io"
  prod_api_cert = "arn:aws:acm:us-east-1:874575230586:certificate/9bb1e798-cd45-4f68-a542-379b0e6459c9"

  tld           = terraform.workspace == "production" ? local.prod_tld : local.stage_tld
  acm_cert      = terraform.workspace == "production" ? local.prod_cert : local.stage_cert

  website_files = tolist(fileset("../app/build/", "**"))
}