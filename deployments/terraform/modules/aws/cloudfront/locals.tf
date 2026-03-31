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

locals {
  # ========================================
  # Resource naming
  # ========================================

  resource_prefix = var.resource_prefix != "" ? var.resource_prefix : var.service_name

  cloudfront_comment = var.cloudfront_comment != null ? var.cloudfront_comment : "${local.resource_prefix} CloudFront distribution"

  waf_web_acl_name      = var.waf_name != null ? var.waf_name : "${local.resource_prefix}-waf"
  waf_ip_set_allow_name = "${local.resource_prefix}-waf-allowlist"
  waf_ip_set_block_name = "${local.resource_prefix}-waf-blocklist"

  origin_id = "${local.resource_prefix}-origin"

  vpc_origin_name = "${local.resource_prefix}-vpc-origin"

  # ========================================
  # Origin type
  # ========================================

  is_vpc_origin    = var.origin_type == "vpc"
  is_custom_origin = var.origin_type == "custom"

  # ========================================
  # Origin verification
  # ========================================

  origin_verify_enabled    = var.origin_verify_secret_arn != null || var.origin_verify_secret_name != null
  origin_verify_secret_ref = coalesce(var.origin_verify_secret_arn, var.origin_verify_secret_name)

  origin_verify_header = local.origin_verify_enabled ? [
    {
      name  = var.origin_verify_header_name
      value = data.aws_secretsmanager_secret_version.origin_verify[0].secret_string
    }
  ] : []

  all_origin_headers = concat(var.origin_custom_headers, local.origin_verify_header)

  # ========================================
  # Cache behavior (hardcoded)
  # ========================================

  # AWS Managed Cache Policy: CachingDisabled
  # https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/using-managed-cache-policies.html
  cache_policy_id = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad"

  # Origin request policy (configurable)
  # https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/using-managed-origin-request-policies.html
  origin_request_policy_id = var.origin_request_policy_id

  # ========================================
  # Viewer certificate
  # ========================================

  use_custom_certificate = length(var.domain_aliases) > 0 && var.acm_certificate_arn != null

  # ========================================
  # WAF
  # ========================================

  create_ip_allowlist = var.waf_enabled && length(var.waf_ip_allowlist) > 0
  create_ip_blocklist = var.waf_enabled && length(var.waf_ip_blocklist) > 0

  # ========================================
  # Tags
  # ========================================

  default_tags = merge(
    {
      "managed-by"   = "terraform"
      "service-type" = "cloudfront-edge"
    },
    var.tags
  )
}
