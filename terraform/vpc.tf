module "vpc" {
  source  = "cloudposse/vpc/aws"
  count = terraform.workspace == "production" ? 1 : 0
  version = "0.28.0"

  cidr_block = "172.16.0.0/16"

  context = module.this.context
}

module "subnets" {
  source  = "cloudposse/dynamic-subnets/aws"
  count = terraform.workspace == "production" ? 1 : 0
  version = "0.39.7"

  availability_zones   = ["us-east-1a", "us-east-1b"]
  vpc_id               = module.vpc[count.index].vpc_id
  igw_id               = module.vpc[count.index].igw_id
  cidr_block           = module.vpc[count.index].vpc_cidr_block
  nat_gateway_enabled  = false
  nat_instance_enabled = false

  context = module.this.context
}