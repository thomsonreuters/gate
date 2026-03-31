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
# CloudFront VPC Origin
# =============================================================================
#
# Creates a CloudFront VPC Origin when origin_type is 'vpc'.
#
# VPC Origins allow CloudFront to connect to internal resources (like internal
# ALBs) via AWS PrivateLink without exposing them to the internet. This provides
# the most secure architecture for CloudFront + ALB deployments.
#
# Benefits:
# - Internal ALB is not exposed to the internet
# - Traffic between CloudFront and ALB stays within AWS network
# - HTTP can be used safely between CloudFront and internal ALB
# - Reduces attack surface compared to internet-facing ALB
#
# =============================================================================

resource "aws_cloudfront_vpc_origin" "main" {
  count = local.is_vpc_origin ? 1 : 0

  vpc_origin_endpoint_config {
    name                   = local.vpc_origin_name
    arn                    = var.vpc_origin_config.origin_arn
    http_port              = var.vpc_origin_config.http_port
    https_port             = var.vpc_origin_config.https_port
    origin_protocol_policy = var.vpc_origin_config.origin_protocol_policy

    origin_ssl_protocols {
      items    = var.vpc_origin_config.origin_ssl_protocols
      quantity = length(var.vpc_origin_config.origin_ssl_protocols)
    }
  }

  tags = local.default_tags

  depends_on = [
    terraform_data.vpc_origin_config_required_validation,
  ]
}
