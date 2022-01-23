module "vpc" {
  source  = "cloudposse/vpc/aws"
  count   = terraform.workspace == "production" ? 1 : 0
  version = "0.28.0"

  cidr_block = "172.16.0.0/16"

  context = module.this.context
}

module "subnets" {
  source  = "cloudposse/dynamic-subnets/aws"
  count   = terraform.workspace == "production" ? 1 : 0
  version = "0.39.7"

  availability_zones   = ["us-east-1a", "us-east-1b"]
  vpc_id               = module.vpc[count.index].vpc_id
  igw_id               = module.vpc[count.index].igw_id
  cidr_block           = module.vpc[count.index].vpc_cidr_block
  nat_gateway_enabled  = false
  nat_instance_enabled = false

  context = module.this.context
}

module "sg" {
  source = "cloudposse/security-group/aws"
  # Cloud Posse recommends pinning every module to a specific version
  # version = "x.x.x"

  # Security Group names must be unique within a VPC.
  # This module follows Cloud Posse naming conventions and generates the name
  # based on the inputs to the null-label module, which means you cannot
  # reuse the label as-is for more than one security group in the VPC.
  #
  # Here we add an attribute to give the security group a unique name.
  attributes = ["redis"]

  # Allow unlimited egress
  allow_all_egress = true

  vpc_id = module.vpc.vpc_id

  context = module.this.context
}