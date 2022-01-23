# Create a zone in order to validate fix for https://github.com/cloudposse/terraform-aws-elasticache-redis/issues/82
resource "aws_route53_zone" "private" {
  count = terraform.workspace == "production" ? 1 : 0

  name = format("elasticache-redis-terratest-%s.testing.cloudposse.co", try(module.this.attributes[0], "default"))

  vpc {
    vpc_id = module.vpc[count.index].vpc_id
  }
}

module "redis" {
  source = "cloudposse/elasticache-redis/aws"
  count = terraform.workspace == "production" ? 1 : 0
  version = "0.41.6"

  transit_encryption_enabled = false
  availability_zones         = ["us-east-1a", "us-east-1b"]
  zone_id                    = [aws_route53_zone.private[count.index].id]
  vpc_id                     = module.vpc[count.index].vpc_id
  allowed_security_group_ids = [module.vpc[count.index].vpc_default_security_group_id]
  subnets                    = module.subnets[count.index].private_subnet_ids
  apply_immediately          = true
  automatic_failover_enabled = false

  context = module.this.context
}