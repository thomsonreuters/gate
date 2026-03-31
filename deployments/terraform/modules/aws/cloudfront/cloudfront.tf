# Copyright 2026 Thomson Reuters
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# =============================================================================
# CloudFront distribution
# =============================================================================
#
# Creates a CloudFront distribution with:
# - Custom origin (internet-facing ALB) or VPC origin (internal ALB via PrivateLink)
# - Configurable caching behavior (default: caching disabled for API traffic)
# - Optional custom domain aliases with ACM certificate
# - Optional WAF Web ACL attachment
# - Geo restriction support
# - Access logging support
#
# =============================================================================

data "aws_secretsmanager_secret_version" "origin_verify" {
  count     = local.origin_verify_enabled ? 1 : 0
  secret_id = local.origin_verify_secret_ref
}

resource "aws_cloudfront_distribution" "main" {
  enabled             = var.cloudfront_enabled
  comment             = local.cloudfront_comment
  price_class         = var.cloudfront_price_class
  http_version        = var.cloudfront_http_version
  is_ipv6_enabled     = var.cloudfront_ipv6_enabled
  default_root_object = var.cloudfront_default_root_object != "" ? var.cloudfront_default_root_object : null
  aliases             = var.domain_aliases
  web_acl_id          = var.waf_enabled ? aws_wafv2_web_acl.main[0].arn : null

  dynamic "origin" {
    for_each = local.is_custom_origin ? [1] : []
    content {
      domain_name = var.origin_domain
      origin_id   = local.origin_id

      custom_origin_config {
        http_port                = var.origin_http_port
        https_port               = var.origin_https_port
        origin_protocol_policy   = var.origin_protocol_policy
        origin_ssl_protocols     = var.origin_ssl_protocols
        origin_read_timeout      = var.origin_read_timeout
        origin_keepalive_timeout = var.origin_keepalive_timeout
      }

      dynamic "custom_header" {
        for_each = range(nonsensitive(length(local.all_origin_headers)))
        content {
          name  = local.all_origin_headers[custom_header.value].name
          value = local.all_origin_headers[custom_header.value].value
        }
      }
    }
  }

  dynamic "origin" {
    for_each = local.is_vpc_origin ? [1] : []
    content {
      domain_name = var.origin_domain
      origin_id   = local.origin_id

      vpc_origin_config {
        vpc_origin_id            = aws_cloudfront_vpc_origin.main[0].id
        origin_read_timeout      = var.origin_read_timeout
        origin_keepalive_timeout = var.origin_keepalive_timeout
      }

      dynamic "custom_header" {
        for_each = range(nonsensitive(length(local.all_origin_headers)))
        content {
          name  = local.all_origin_headers[custom_header.value].name
          value = local.all_origin_headers[custom_header.value].value
        }
      }
    }
  }

  default_cache_behavior {
    target_origin_id       = local.origin_id
    viewer_protocol_policy = var.viewer_protocol_policy
    allowed_methods        = var.allowed_methods
    cached_methods         = ["GET", "HEAD"]
    compress               = var.compress

    cache_policy_id            = local.cache_policy_id
    origin_request_policy_id   = local.origin_request_policy_id
    response_headers_policy_id = var.response_headers_policy_id
  }

  restrictions {
    geo_restriction {
      restriction_type = var.geo_restriction_type
      locations        = var.geo_restriction_locations
    }
  }

  viewer_certificate {
    # Use custom certificate if aliases are provided
    acm_certificate_arn      = local.use_custom_certificate ? var.acm_certificate_arn : null
    ssl_support_method       = local.use_custom_certificate ? "sni-only" : null
    minimum_protocol_version = local.use_custom_certificate ? var.cloudfront_minimum_protocol_version : null

    # Use CloudFront default certificate if no aliases
    cloudfront_default_certificate = !local.use_custom_certificate
  }

  dynamic "logging_config" {
    for_each = var.logging_enabled ? [1] : []
    content {
      bucket          = var.logging_bucket
      prefix          = var.logging_prefix
      include_cookies = var.logging_include_cookies
    }
  }

  tags = local.default_tags

  depends_on = [
    terraform_data.custom_domain_certificate_validation,
    terraform_data.geo_restriction_validation,
    terraform_data.logging_bucket_validation,
    terraform_data.vpc_origin_config_required_validation,
    terraform_data.vpc_origin_config_not_allowed_validation,
    terraform_data.origin_verify_secret_mutual_exclusivity,
  ]
}
