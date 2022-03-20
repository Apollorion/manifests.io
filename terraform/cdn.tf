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
  dns_alias_enabled   = true
  website_enabled     = true

  lambda_function_association = [{
    event_type   = "origin-request"
    include_body = false
    lambda_arn   = "${aws_lambda_function.edge.arn}:${aws_lambda_function.edge.version}"
  }]

  custom_error_response = [{
    error_code            = 404
    response_code         = 200
    response_page_path    = "/index.html"
    error_caching_min_ttl = 86400
  }]

}

module "status_cdn" {
  count = terraform.workspace == "production" ? 1 : 0

  source  = "cloudposse/cloudfront-s3-cdn/aws"
  version = "0.40.0"

  aliases             = ["status.${local.tld}"]
  acm_certificate_arn = data.aws_acm_certificate.main.arn
  allowed_methods     = ["HEAD", "GET"]
  name                = "status.${local.tld}"
  parent_zone_id      = data.aws_route53_zone.manifests_io.zone_id
  dns_alias_enabled   = true
  website_enabled     = true
}

resource "aws_s3_bucket_object" "status_redirect" {
  count = terraform.workspace == "production" ? 1 : 0

  bucket           = module.status_cdn[count.index].s3_bucket
  key              = "index.html"
  source           = ""
  acl              = "public-read"
  content_type     = "text/html"
  etag             = md5("index.html")
  website_redirect = local.statuspage_url
  tags             = {}
}

resource "aws_s3_bucket_object" "websitefiles" {
  count        = length(local.website_files)
  bucket       = module.cdn.s3_bucket
  key          = local.website_files[count.index]
  source       = "../app/build/${local.website_files[count.index]}"
  content_type = local.extension_to_mime[split(".", local.website_files[count.index])[length(split(".", local.website_files[count.index])) - 1]]
  etag         = filemd5("../app/build/${local.website_files[count.index]}")
}