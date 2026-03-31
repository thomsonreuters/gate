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
# GATE (GitHub Authenticated Token Exchange)
# Module: AWS CloudFront
# =============================================================================
#
# This module provisions an AWS CloudFront distribution with optional WAF
# protection and Route53 DNS records. It supports two origin types:
#
# 1. Custom Origin (default): Internet-facing ALB or custom endpoint
# 2. VPC Origin: Internal ALB via AWS PrivateLink (most secure)
#
# Core Resources:
# - CloudFront distribution with configurable caching and security settings
# - Custom origin configuration for internet-facing endpoints
# - VPC origin configuration for internal endpoints via PrivateLink
#
# Optional Resources (conditional based on variables):
# - WAF Web ACL with AWS Managed Rules (when waf_enabled=true)
# - WAF rate limiting rule (when waf_rate_limit_enabled=true)
# - WAF IP allowlist/blocklist (when configured)
# - Route53 alias records (when route53_zone_id is provided)
#
# Prerequisites:
# - Origin endpoint (ALB DNS name or custom domain)
# - ACM certificate in us-east-1 (only if using custom domain aliases)
# - Route53 hosted zone (only if creating DNS records)
# - For VPC origins: Internal ALB ARN
#
# -----------------------------------------------------------------------------
# Usage: Custom Origin (internet-facing ALB)
# -----------------------------------------------------------------------------
#
# Use this when your ALB is internet-facing with HTTPS:
#
#   module "cloudfront" {
#     source = "./modules/aws/cloudfront"
#
#     origin_domain          = module.ecs.alb_dns_name
#     origin_protocol_policy = "https-only"
#
#     providers = {
#       aws.us_east_1 = aws.us_east_1
#     }
#   }
#
# -----------------------------------------------------------------------------
# Usage: VPC Origin (internal ALB via PrivateLink)
# -----------------------------------------------------------------------------
#
# Use this for maximum security (ALB is not exposed to the internet):
#
#   module "cloudfront" {
#     source = "./modules/aws/cloudfront"
#
#     origin_type   = "vpc"
#     origin_domain = module.ecs.alb_dns_name
#
#     vpc_origin_config = {
#       origin_arn             = module.ecs.alb_arn
#       origin_protocol_policy = "http-only"
#     }
#
#     providers = {
#       aws.us_east_1 = aws.us_east_1
#     }
#   }
#
# =============================================================================

terraform {
  required_version = ">= 1.6"

  required_providers {
    aws = {
      source                = "hashicorp/aws"
      version               = "~> 6.0"
      configuration_aliases = [aws.us_east_1]
    }
  }
}
