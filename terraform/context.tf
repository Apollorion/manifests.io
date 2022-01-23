module "this" {
  source  = "cloudposse/label/null"
  version = "0.25.0"
  namespace  = "manifestsio"
  stage      = terraform.workspace
  name       = "manifestsio"
}