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
# AWS CloudFront module validation
# =============================================================================
#
# This file contains cross-variable validations using lifecycle preconditions.
# Single-variable validations are defined in variables.tf using validation blocks.
#
# Validations enforce:
# - Required dependencies between variables (e.g., certificate for custom domains)
# - Security best practices
# - AWS service constraints
# - Mutual exclusivity of conflicting configurations
#
# =============================================================================

# =============================================================================
# Custom domain validation
# =============================================================================

resource "terraform_data" "custom_domain_certificate_validation" {
  count = length(var.domain_aliases) > 0 ? 1 : 0

  lifecycle {
    precondition {
      condition     = var.acm_certificate_arn != null
      error_message = "acm_certificate_arn is required when domain_aliases is not empty. The certificate must be in us-east-1."
    }
  }
}

# =============================================================================
# VPC origin validation
# =============================================================================

resource "terraform_data" "vpc_origin_config_required_validation" {
  count = var.origin_type == "vpc" ? 1 : 0

  lifecycle {
    precondition {
      condition     = var.vpc_origin_config != null
      error_message = "vpc_origin_config is required when origin_type is 'vpc'."
    }
  }
}

resource "terraform_data" "vpc_origin_config_not_allowed_validation" {
  count = var.origin_type == "custom" ? 1 : 0

  lifecycle {
    precondition {
      condition     = var.vpc_origin_config == null
      error_message = "vpc_origin_config should not be set when origin_type is 'custom'. Use origin_protocol_policy and other origin_* variables instead."
    }
  }
}

# =============================================================================
# Origin verification validation
# =============================================================================

resource "terraform_data" "origin_verify_secret_mutual_exclusivity" {
  lifecycle {
    precondition {
      condition     = var.origin_verify_secret_arn == null || var.origin_verify_secret_name == null
      error_message = "origin_verify_secret_arn and origin_verify_secret_name are mutually exclusive. Provide only one."
    }
  }
}

# =============================================================================
# Geo restriction validation
# =============================================================================

resource "terraform_data" "geo_restriction_validation" {
  count = var.geo_restriction_type != "none" ? 1 : 0

  lifecycle {
    precondition {
      condition     = length(var.geo_restriction_locations) > 0
      error_message = "geo_restriction_locations must not be empty when geo_restriction_type is '${var.geo_restriction_type}'."
    }
  }
}

# =============================================================================
# Logging validation
# =============================================================================

resource "terraform_data" "logging_bucket_validation" {
  count = var.logging_enabled ? 1 : 0

  lifecycle {
    precondition {
      condition     = var.logging_bucket != null && var.logging_bucket != ""
      error_message = "logging_bucket is required when logging_enabled is true."
    }
  }
}

# =============================================================================
# Route53 validation
# =============================================================================

resource "terraform_data" "route53_domain_alias_validation" {
  count = var.route53_zone_id != null && var.route53_record_name != null ? 1 : 0

  lifecycle {
    precondition {
      condition     = length(var.domain_aliases) > 0
      error_message = "domain_aliases must include the Route53 record domain when creating DNS records. The CloudFront distribution needs a matching alias to serve traffic for the custom domain."
    }
  }
}

# =============================================================================
# WAF validation
# =============================================================================

resource "terraform_data" "waf_rule_priority_validation" {
  count = var.waf_enabled && length(var.waf_managed_rules) + length(var.waf_custom_rule_groups) > 0 ? 1 : 0

  lifecycle {
    precondition {
      condition = length(
        concat([for r in var.waf_managed_rules : r.priority], [for r in var.waf_custom_rule_groups : r.priority])
        ) == length(distinct(
          concat([for r in var.waf_managed_rules : r.priority], [for r in var.waf_custom_rule_groups : r.priority])
      ))
      error_message = "All WAF rule priorities must be unique across managed rules and custom rule groups."
    }
  }
}
