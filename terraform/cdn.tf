data "aws_route53_zone" "manifests_io" {
  name = local.prod_tld
}

module "cdn" {
  source  = "cloudposse/cloudfront-s3-cdn/aws"
  version = "0.40.0"

  aliases             = [local.tld, "www.${local.tld}"]
  acm_certificate_arn = data.aws_acm_certificate.main.arn
  allowed_methods     = ["HEAD", "GET"]
  name                = local.tld
  parent_zone_id      = data.aws_route53_zone.manifests_io.zone_id
  error_document      = "index.html"
  dns_alias_enabled   = true
  website_enabled     = true
}

resource "aws_s3_bucket_object" "websitefiles" {
  count        = length(local.website_files)
  bucket       = module.cdn.s3_bucket
  key          = local.website_files[count.index]
  source       = "../app/build/${local.website_files[count.index]}"
  content_type = local.extension_to_mime[split(".", local.website_files[count.index])[length(split(".", local.website_files[count.index])) - 1]]
  etag         = filemd5("../app/build/${local.website_files[count.index]}")
}