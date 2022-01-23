module "redis" {
  source  = "cloudposse/elasticache-redis/aws"
  count   = terraform.workspace == "production" ? 1 : 0
  version = "0.41.6"

  transit_encryption_enabled = false
  availability_zones         = ["us-east-1a", "us-east-1b"]
  vpc_id                     = module.vpc[count.index].vpc_id
  allowed_security_group_ids = [module.vpc[count.index].vpc_default_security_group_id]
  subnets                    = module.subnets[count.index].private_subnet_ids
  apply_immediately          = true
  automatic_failover_enabled = false

  context = module.this.context
}