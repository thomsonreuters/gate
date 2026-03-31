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
# WAF Web ACL for CloudFront
# =============================================================================
#
# Creates a WAF Web ACL with:
# - IP blocklist (highest priority, block malicious IPs first)
# - IP allowlist (bypass other rules for trusted IPs)
# - Rate limiting (protect against DDoS and abuse)
# - AWS Managed Rules (OWASP protections, known bad inputs, etc.)
#
# IMPORTANT: WAF resources for CloudFront MUST be created in us-east-1 (AWS requirement).
# All WAF resources in this file use the aws.us_east_1 provider, which must be passed
# from the root module when waf_enabled=true and deploying from regions other than us-east-1.
#
# =============================================================================

# =============================================================================
# IP sets
# =============================================================================

resource "aws_wafv2_ip_set" "allowlist" {
  count    = local.create_ip_allowlist ? 1 : 0
  provider = aws.us_east_1

  name               = local.waf_ip_set_allow_name
  description        = "Allowed IP addresses for ${local.resource_prefix}"
  scope              = "CLOUDFRONT"
  ip_address_version = "IPV4"
  addresses          = var.waf_ip_allowlist

  tags = local.default_tags
}

resource "aws_wafv2_ip_set" "blocklist" {
  count    = local.create_ip_blocklist ? 1 : 0
  provider = aws.us_east_1

  name               = local.waf_ip_set_block_name
  description        = "Blocked IP addresses for ${local.resource_prefix}"
  scope              = "CLOUDFRONT"
  ip_address_version = "IPV4"
  addresses          = var.waf_ip_blocklist

  tags = local.default_tags
}

# =============================================================================
# Web ACL
# =============================================================================

resource "aws_wafv2_web_acl" "main" {
  count    = var.waf_enabled ? 1 : 0
  provider = aws.us_east_1

  name        = local.waf_web_acl_name
  description = "WAF Web ACL for ${local.resource_prefix} CloudFront distribution"
  scope       = "CLOUDFRONT"

  default_action {
    dynamic "allow" {
      for_each = var.waf_default_action == "allow" ? [1] : []
      content {}
    }
    dynamic "block" {
      for_each = var.waf_default_action == "block" ? [1] : []
      content {}
    }
  }

  dynamic "rule" {
    for_each = local.create_ip_blocklist ? [1] : []
    content {
      name     = "ip-blocklist"
      priority = 0

      statement {
        ip_set_reference_statement {
          arn = aws_wafv2_ip_set.blocklist[0].arn
        }
      }

      action {
        block {}
      }

      visibility_config {
        cloudwatch_metrics_enabled = var.waf_cloudwatch_metrics_enabled
        metric_name                = "${local.resource_prefix}-ip-blocklist"
        sampled_requests_enabled   = var.waf_sampled_requests_enabled
      }
    }
  }

  dynamic "rule" {
    for_each = local.create_ip_allowlist ? [1] : []
    content {
      name     = "ip-allowlist"
      priority = 1

      statement {
        ip_set_reference_statement {
          arn = aws_wafv2_ip_set.allowlist[0].arn
        }
      }

      action {
        allow {}
      }

      visibility_config {
        cloudwatch_metrics_enabled = var.waf_cloudwatch_metrics_enabled
        metric_name                = "${local.resource_prefix}-ip-allowlist"
        sampled_requests_enabled   = var.waf_sampled_requests_enabled
      }
    }
  }

  dynamic "rule" {
    for_each = var.waf_rate_limit_enabled ? [1] : []
    content {
      name     = "rate-limit"
      priority = 5

      statement {
        rate_based_statement {
          limit              = var.waf_rate_limit
          aggregate_key_type = "IP"
        }
      }

      action {
        dynamic "block" {
          for_each = var.waf_rate_limit_action == "block" ? [1] : []
          content {}
        }
        dynamic "count" {
          for_each = var.waf_rate_limit_action == "count" ? [1] : []
          content {}
        }
      }

      visibility_config {
        cloudwatch_metrics_enabled = var.waf_cloudwatch_metrics_enabled
        metric_name                = "${local.resource_prefix}-rate-limit"
        sampled_requests_enabled   = var.waf_sampled_requests_enabled
      }
    }
  }

  dynamic "rule" {
    for_each = var.waf_managed_rules
    content {
      name     = rule.value.name
      priority = rule.value.priority

      override_action {
        dynamic "none" {
          for_each = coalesce(rule.value.override_action, "none") == "none" ? [1] : []
          content {}
        }
        dynamic "count" {
          for_each = coalesce(rule.value.override_action, "none") == "count" ? [1] : []
          content {}
        }
      }

      statement {
        managed_rule_group_statement {
          name        = rule.value.name
          vendor_name = "AWS"
        }
      }

      visibility_config {
        cloudwatch_metrics_enabled = var.waf_cloudwatch_metrics_enabled
        metric_name                = "${local.resource_prefix}-${lower(replace(rule.value.name, "AWSManagedRules", ""))}"
        sampled_requests_enabled   = var.waf_sampled_requests_enabled
      }
    }
  }

  visibility_config {
    cloudwatch_metrics_enabled = var.waf_cloudwatch_metrics_enabled
    metric_name                = "${local.resource_prefix}-waf"
    sampled_requests_enabled   = var.waf_sampled_requests_enabled
  }

  tags = local.default_tags

  depends_on = [
    terraform_data.waf_rule_priority_validation,
  ]
}
