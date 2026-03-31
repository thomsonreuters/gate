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
# Route53 DNS records
# =============================================================================
#
# Creates Route53 alias records pointing to the CloudFront distribution.
# Both IPv4 (A record) and IPv6 (AAAA record) are created when IPv6 is enabled.
#
# Prerequisites:
# - Route53 hosted zone must exist
# - domain_aliases must include the domain being created
#
# =============================================================================

resource "aws_route53_record" "main" {
  count = var.route53_zone_id != null ? 1 : 0

  zone_id = var.route53_zone_id
  name    = var.route53_record_name != null ? var.route53_record_name : ""
  type    = "A"

  alias {
    name                   = aws_cloudfront_distribution.main.domain_name
    zone_id                = aws_cloudfront_distribution.main.hosted_zone_id
    evaluate_target_health = false
  }

  depends_on = [
    terraform_data.route53_domain_alias_validation,
  ]
}

# IPv6 AAAA record (if IPv6 is enabled)
resource "aws_route53_record" "main_ipv6" {
  count = var.route53_zone_id != null && var.cloudfront_ipv6_enabled ? 1 : 0

  zone_id = var.route53_zone_id
  name    = var.route53_record_name != null ? var.route53_record_name : ""
  type    = "AAAA"

  alias {
    name                   = aws_cloudfront_distribution.main.domain_name
    zone_id                = aws_cloudfront_distribution.main.hosted_zone_id
    evaluate_target_health = false
  }
}
