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

# ========================================
# Application
# ========================================

output "resource_prefix" {
  description = "Resource prefix used for naming"
  value       = local.resource_prefix
}

# ========================================
# CloudFront distribution
# ========================================

output "cloudfront_distribution_id" {
  description = "CloudFront distribution ID"
  value       = aws_cloudfront_distribution.main.id
}

output "cloudfront_distribution_arn" {
  description = "CloudFront distribution ARN"
  value       = aws_cloudfront_distribution.main.arn
}

output "cloudfront_domain_name" {
  description = "CloudFront distribution domain name"
  value       = aws_cloudfront_distribution.main.domain_name
}

output "cloudfront_hosted_zone_id" {
  description = "CloudFront hosted zone ID (for Route53 alias records)"
  value       = aws_cloudfront_distribution.main.hosted_zone_id
}

output "cloudfront_etag" {
  description = "CloudFront distribution ETag (for updates)"
  value       = aws_cloudfront_distribution.main.etag
}

output "cloudfront_status" {
  description = "CloudFront distribution status"
  value       = aws_cloudfront_distribution.main.status
}

# ========================================
# URLs
# ========================================

output "cloudfront_url" {
  description = "CloudFront distribution URL (default domain)"
  value       = "https://${aws_cloudfront_distribution.main.domain_name}"
}

output "service_url" {
  description = "Service URL"
  value       = length(var.domain_aliases) > 0 ? "https://${var.domain_aliases[0]}" : "https://${aws_cloudfront_distribution.main.domain_name}"
}

# ========================================
# WAF
# ========================================

output "waf_web_acl_id" {
  description = "WAF Web ACL ID (if enabled)"
  value       = var.waf_enabled ? aws_wafv2_web_acl.main[0].id : null
}

output "waf_web_acl_arn" {
  description = "WAF Web ACL ARN (if enabled)"
  value       = var.waf_enabled ? aws_wafv2_web_acl.main[0].arn : null
}

output "waf_web_acl_name" {
  description = "WAF Web ACL name (if enabled)"
  value       = var.waf_enabled ? aws_wafv2_web_acl.main[0].name : null
}

output "waf_ip_allowlist_arn" {
  description = "WAF IP allowlist ARN (if configured)"
  value       = local.create_ip_allowlist ? aws_wafv2_ip_set.allowlist[0].arn : null
}

output "waf_ip_blocklist_arn" {
  description = "WAF IP blocklist ARN (if configured)"
  value       = local.create_ip_blocklist ? aws_wafv2_ip_set.blocklist[0].arn : null
}

# ========================================
# Route53
# ========================================

output "route53_record_fqdn" {
  description = "Route53 record FQDN (if created)"
  value       = var.route53_zone_id != null ? aws_route53_record.main[0].fqdn : null
}

output "route53_record_name" {
  description = "Route53 record name (if created)"
  value       = var.route53_zone_id != null ? aws_route53_record.main[0].name : null
}

# ========================================
# Origin
# ========================================

output "origin_type" {
  description = "Origin type (custom or vpc)"
  value       = var.origin_type
}

output "origin_domain" {
  description = "Origin domain name configured for the distribution"
  value       = var.origin_domain
}

output "origin_id" {
  description = "Origin ID used in the distribution"
  value       = local.origin_id
}

output "origin_verify_secret_name" {
  description = "Origin verification secret name"
  value       = local.origin_verify_enabled ? local.origin_verify_secret_ref : null
}

# ========================================
# VPC Origin
# ========================================

output "vpc_origin_id" {
  description = "VPC Origin ID (if using VPC origin)"
  value       = local.is_vpc_origin ? aws_cloudfront_vpc_origin.main[0].id : null
}

output "vpc_origin_arn" {
  description = "VPC Origin ARN (if using VPC origin)"
  value       = local.is_vpc_origin ? aws_cloudfront_vpc_origin.main[0].arn : null
}
