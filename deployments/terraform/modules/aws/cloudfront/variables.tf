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
# Tagging
# ========================================

variable "tags" {
  description = "Additional tags to apply to all resources. Merged with module defaults (managed-by, service-type and environment (optional))."
  type        = map(string)
  default     = {}

  validation {
    condition     = length(var.tags) <= 47
    error_message = "Too many tags: ${length(var.tags)} user tags + 3 module default tags would exceed AWS limit of 50."
  }

  validation {
    condition = alltrue([
      for key in keys(var.tags) : length(key) > 0 && length(key) <= 128
    ])
    error_message = "All tag keys must be non-empty and not exceed 128 characters."
  }

  validation {
    condition = alltrue([
      for key in keys(var.tags) : !can(regex("^(?i)aws:", key))
    ])
    error_message = "Tag keys cannot use the reserved 'aws:' prefix (case-insensitive)."
  }

  validation {
    condition = alltrue([
      for key in keys(var.tags) : can(regex("^[a-zA-Z0-9\\s+\\-=._:/@]+$", key))
    ])
    error_message = "Tag keys must only contain letters, numbers, spaces, and the following characters: + - = . _ : / @"
  }

  validation {
    condition = alltrue([
      for value in values(var.tags) : length(value) <= 256
    ])
    error_message = "All tag values must not exceed 256 characters."
  }

  validation {
    condition = alltrue([
      for value in values(var.tags) : can(regex("^[a-zA-Z0-9\\s+\\-=._:/@]*$", value))
    ])
    error_message = "Tag values must only contain letters, numbers, spaces, and the following characters: + - = . _ : / @"
  }
}

# ========================================
# Naming
# ========================================

variable "service_name" {
  description = "Name of the service (used for resource naming)"
  type        = string
  default     = "gate"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]*[a-z0-9]$", var.service_name)) && length(var.service_name) >= 2 && length(var.service_name) <= 32
    error_message = "service_name must start with a letter, contain only lowercase letters, numbers, and hyphens, end with a letter or number, and be 2-32 characters."
  }
}

variable "resource_prefix" {
  description = "Prefix for resource names. If empty, service_name is used."
  type        = string
  default     = ""
}

# ========================================
# Origin
# ========================================

variable "origin_type" {
  description = <<-EOT
    Origin type for CloudFront distribution.

    - custom: Internet-facing origin (default)
    - vpc:    Internal VPC origin via AWS PrivateLink
  EOT
  type        = string
  default     = "custom"

  validation {
    condition     = contains(["custom", "vpc"], var.origin_type)
    error_message = "origin_type must be 'custom' or 'vpc'."
  }
}

variable "origin_domain" {
  description = <<-EOT
    Origin domain name for CloudFront distribution.

    For custom origins:
    - ECS deployments: Use the ALB DNS name (e.g., module.ecs.alb_dns_name)
    - EKS deployments: Use the Ingress endpoint (from kubectl or Helm output)
    - Custom endpoints: Any valid HTTP/HTTPS endpoint

    For VPC origins:
    - Use the internal ALB DNS name (e.g., internal-gate-xxx.us-east-1.elb.amazonaws.com)
  EOT
  type        = string

  validation {
    condition     = can(regex("^[a-zA-Z0-9][a-zA-Z0-9-.]+[a-zA-Z0-9]$", var.origin_domain))
    error_message = "origin_domain must be a valid domain name."
  }
}

variable "vpc_origin_config" {
  description = <<-EOT
    VPC origin configuration. Required when origin_type is 'vpc'.

    Allows CloudFront to connect to internal resources via AWS PrivateLink
    without exposing them to the internet.

    - origin_arn: ARN of the internal ALB or other supported endpoint
    - http_port: HTTP port on the origin (default: 80)
    - https_port: HTTPS port on the origin (default: 443)
    - origin_protocol_policy: Protocol to use (http-only, https-only, match-viewer)
    - origin_ssl_protocols: SSL/TLS protocols for HTTPS connections
  EOT
  type = object({
    origin_arn             = string
    http_port              = optional(number, 80)
    https_port             = optional(number, 443)
    origin_protocol_policy = optional(string, "http-only")
    origin_ssl_protocols   = optional(list(string), ["TLSv1.2"])
  })
  default = null

  validation {
    condition     = var.vpc_origin_config == null ? true : can(regex("^arn:aws:elasticloadbalancing:[a-z0-9-]+:[0-9]{12}:loadbalancer/", var.vpc_origin_config.origin_arn))
    error_message = "vpc_origin_config.origin_arn must be a valid ALB ARN."
  }

  validation {
    condition     = var.vpc_origin_config == null ? true : (var.vpc_origin_config.http_port >= 1 && var.vpc_origin_config.http_port <= 65535)
    error_message = "vpc_origin_config.http_port must be between 1 and 65535."
  }

  validation {
    condition     = var.vpc_origin_config == null ? true : (var.vpc_origin_config.https_port >= 1 && var.vpc_origin_config.https_port <= 65535)
    error_message = "vpc_origin_config.https_port must be between 1 and 65535."
  }

  validation {
    condition     = var.vpc_origin_config == null ? true : contains(["http-only", "https-only", "match-viewer"], var.vpc_origin_config.origin_protocol_policy)
    error_message = "vpc_origin_config.origin_protocol_policy must be 'http-only', 'https-only', or 'match-viewer'."
  }

  validation {
    condition = var.vpc_origin_config == null ? true : alltrue([
      for p in var.vpc_origin_config.origin_ssl_protocols : contains(["SSLv3", "TLSv1", "TLSv1.1", "TLSv1.2"], p)
    ])
    error_message = "vpc_origin_config.origin_ssl_protocols must contain valid SSL/TLS protocols ('SSLv3', 'TLSv1', 'TLSv1.1', 'TLSv1.2')."
  }
}

variable "origin_protocol_policy" {
  description = "Protocol policy for origin requests (http-only, https-only, match-viewer)"
  type        = string
  default     = "https-only"

  validation {
    condition     = contains(["http-only", "https-only", "match-viewer"], var.origin_protocol_policy)
    error_message = "origin_protocol_policy must be http-only, https-only, or match-viewer."
  }
}

variable "origin_http_port" {
  description = "HTTP port for the origin"
  type        = number
  default     = 80

  validation {
    condition     = var.origin_http_port >= 1 && var.origin_http_port <= 65535
    error_message = "origin_http_port must be between 1 and 65535."
  }
}

variable "origin_https_port" {
  description = "HTTPS port for the origin"
  type        = number
  default     = 443

  validation {
    condition     = var.origin_https_port >= 1 && var.origin_https_port <= 65535
    error_message = "origin_https_port must be between 1 and 65535."
  }
}

variable "origin_ssl_protocols" {
  description = "SSL/TLS protocols for origin communication"
  type        = list(string)
  default     = ["TLSv1.2"]

  validation {
    condition = alltrue([
      for p in var.origin_ssl_protocols : contains(["SSLv3", "TLSv1", "TLSv1.1", "TLSv1.2"], p)
    ])
    error_message = "origin_ssl_protocols must contain valid SSL/TLS protocols (SSLv3, TLSv1, TLSv1.1, TLSv1.2)."
  }
}

variable "origin_read_timeout" {
  description = "Read timeout for origin requests (seconds)"
  type        = number
  default     = 30

  validation {
    condition     = var.origin_read_timeout >= 1 && var.origin_read_timeout <= 60
    error_message = "origin_read_timeout must be between 1 and 60 seconds."
  }
}

variable "origin_keepalive_timeout" {
  description = "Keep-alive timeout for origin connections (seconds)"
  type        = number
  default     = 5

  validation {
    condition     = var.origin_keepalive_timeout >= 1 && var.origin_keepalive_timeout <= 60
    error_message = "origin_keepalive_timeout must be between 1 and 60 seconds."
  }
}

variable "origin_custom_headers" {
  description = <<-EOT
    Custom headers to send to the origin. Useful for origin verification.

    Example: Setting a secret header that the origin ALB validates ensures
    requests only come through CloudFront.
  EOT
  type = list(object({
    name  = string
    value = string
  }))
  default   = []
  sensitive = true
}

variable "origin_verify_secret_arn" {
  description = <<-EOT
    ARN of AWS Secrets Manager secret for origin verification.

    When provided, the module reads the secret value and adds an origin
    verification header (default: X-Origin-Verify) to all requests.
    The application should validate this header to ensure requests
    come through CloudFront.

    Mutually exclusive with origin_verify_secret_name.
  EOT
  type        = string
  default     = null

  validation {
    condition     = var.origin_verify_secret_arn == null || can(regex("^arn:aws:secretsmanager:[a-z0-9-]+:[0-9]{12}:secret:.+$", var.origin_verify_secret_arn))
    error_message = "origin_verify_secret_arn must be a valid Secrets Manager secret ARN."
  }
}

variable "origin_verify_secret_name" {
  description = <<-EOT
    Name of AWS Secrets Manager secret for origin verification.

    Convenience alternative to origin_verify_secret_arn. When provided,
    the module reads the secret value and adds an origin verification
    header (default: X-Origin-Verify) to all requests.

    Mutually exclusive with origin_verify_secret_arn.
  EOT
  type        = string
  default     = null
}

variable "origin_verify_header_name" {
  description = "Header name for origin verification"
  type        = string
  default     = "X-Origin-Verify"

  validation {
    condition     = can(regex("^[A-Za-z][A-Za-z0-9-]*$", var.origin_verify_header_name))
    error_message = "origin_verify_header_name must be a valid HTTP header name."
  }
}

variable "origin_request_policy_id" {
  description = <<-EOT
    Origin request policy ID for CloudFront distribution.

    Common AWS Managed Policies:
    - AllViewerExceptHostHeader: b689b0a8-53d0-40ab-baf2-68738e2966ac (default)
    - Managed-AllViewer: 216adef6-5c7f-47e4-b989-5492eafa07d3 (forwards Host header)
    - CORS-S3Origin: 88a5eaf4-2fd4-4709-b370-b4c650ea3fcf

    More information: https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/using-managed-origin-request-policies.html
  EOT
  type        = string
  default     = "b689b0a8-53d0-40ab-baf2-68738e2966ac"
}

# ========================================
# CloudFront distribution
# ========================================

variable "cloudfront_enabled" {
  description = "Whether the CloudFront distribution is enabled to accept requests"
  type        = bool
  default     = true
}

variable "cloudfront_comment" {
  description = "Comment for the CloudFront distribution"
  type        = string
  default     = null
}

variable "cloudfront_price_class" {
  description = <<-EOT
    CloudFront price class. Controls which edge locations are used.

    - PriceClass_100: US, Canada, Europe (lowest cost)
    - PriceClass_200: US, Canada, Europe, Asia, Middle East, Africa
    - PriceClass_All: All edge locations (best performance, highest cost)
  EOT
  type        = string
  default     = "PriceClass_100"

  validation {
    condition     = contains(["PriceClass_100", "PriceClass_200", "PriceClass_All"], var.cloudfront_price_class)
    error_message = "cloudfront_price_class must be PriceClass_100, PriceClass_200, or PriceClass_All."
  }
}

variable "cloudfront_http_version" {
  description = "Maximum HTTP version for CloudFront (http1.1, http2, http2and3, http3)"
  type        = string
  default     = "http2and3"

  validation {
    condition     = contains(["http1.1", "http2", "http2and3", "http3"], var.cloudfront_http_version)
    error_message = "cloudfront_http_version must be http1.1, http2, http2and3, or http3."
  }
}

variable "cloudfront_ipv6_enabled" {
  description = "Whether IPv6 is enabled for the distribution"
  type        = bool
  default     = true
}

variable "cloudfront_default_root_object" {
  description = "Default root object (e.g., index.html). Leave empty for API endpoints."
  type        = string
  default     = ""
}

# ========================================
# Custom domain (aliases)
# ========================================

variable "domain_aliases" {
  description = <<-EOT
    Custom domain names for the CloudFront distribution.

    Requires acm_certificate_arn to be set with a certificate covering these domains.
    The certificate MUST be in us-east-1 region.

    Example: ["gate.example.com", "api.example.com"]
  EOT
  type        = list(string)
  default     = []

  validation {
    condition = alltrue([
      for d in var.domain_aliases : can(regex("^[a-zA-Z0-9*][a-zA-Z0-9-.*]*[a-zA-Z0-9]$", d)) || can(regex("^[a-zA-Z0-9]$", d))
    ])
    error_message = "All domain aliases must be valid domain names."
  }
}

variable "acm_certificate_arn" {
  description = <<-EOT
    ARN of the ACM certificate for custom domain aliases.

    IMPORTANT: The certificate MUST be in us-east-1 region (CloudFront requirement).
    Required when domain_aliases is not empty.
  EOT
  type        = string
  default     = null

  validation {
    condition     = var.acm_certificate_arn == null || can(regex("^arn:aws:acm:us-east-1:[0-9]{12}:certificate/[a-f0-9-]+$", var.acm_certificate_arn))
    error_message = "acm_certificate_arn must be a valid ACM certificate ARN in us-east-1 region."
  }
}

variable "cloudfront_minimum_protocol_version" {
  description = <<-EOT
    Minimum TLS protocol version for viewer connections.

    Recommended: TLSv1.2_2021 for security compliance.
    Use TLSv1.2_2018 only if legacy client compatibility is required.
  EOT
  type        = string
  default     = "TLSv1.2_2021"

  validation {
    condition = contains([
      "TLSv1.2_2018",
      "TLSv1.2_2019",
      "TLSv1.2_2021",
    ], var.cloudfront_minimum_protocol_version)
    error_message = "cloudfront_minimum_protocol_version must enforce TLS 1.2 minimum (TLSv1.2_2018, TLSv1.2_2019, or TLSv1.2_2021)."
  }
}

# ========================================
# Viewer settings
# ========================================

variable "viewer_protocol_policy" {
  description = "Protocol policy for viewer connections (allow-all, https-only, redirect-to-https)"
  type        = string
  default     = "redirect-to-https"

  validation {
    condition     = contains(["allow-all", "https-only", "redirect-to-https"], var.viewer_protocol_policy)
    error_message = "viewer_protocol_policy must be allow-all, https-only, or redirect-to-https."
  }
}

variable "allowed_methods" {
  description = "HTTP methods allowed by CloudFront"
  type        = list(string)
  default     = ["DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT"]

  validation {
    condition = (
      (length(var.allowed_methods) == 2 && contains(var.allowed_methods, "GET") && contains(var.allowed_methods, "HEAD")) ||
      (length(var.allowed_methods) == 3 && contains(var.allowed_methods, "GET") && contains(var.allowed_methods, "HEAD") && contains(var.allowed_methods, "OPTIONS")) ||
      (length(var.allowed_methods) == 7 && contains(var.allowed_methods, "DELETE") && contains(var.allowed_methods, "GET") && contains(var.allowed_methods, "HEAD") && contains(var.allowed_methods, "OPTIONS") && contains(var.allowed_methods, "PATCH") && contains(var.allowed_methods, "POST") && contains(var.allowed_methods, "PUT"))
    )
    error_message = "allowed_methods must be one of: [GET, HEAD], [GET, HEAD, OPTIONS], or [DELETE, GET, HEAD, OPTIONS, PATCH, POST, PUT]."
  }
}

variable "compress" {
  description = "Whether CloudFront automatically compresses content"
  type        = bool
  default     = true
}

# ========================================
# Response headers
# ========================================

variable "response_headers_policy_id" {
  description = <<-EOT
    CloudFront response headers policy ID. Optional.

    AWS Managed Policies:
    - SecurityHeadersPolicy: 67f7725c-6f97-4210-82d7-5512b31e9d03
    - CORS-and-SecurityHeadersPolicy: e61eb60c-9c35-4d20-a928-2b84e02af89c
  EOT
  type        = string
  default     = null

  validation {
    condition     = var.response_headers_policy_id == null || can(regex("^[a-f0-9-]{36}$", var.response_headers_policy_id))
    error_message = "response_headers_policy_id must be a valid UUID or null."
  }
}

# ========================================
# Geo restriction
# ========================================

variable "geo_restriction_type" {
  description = "Geo restriction type (none, whitelist, blacklist)"
  type        = string
  default     = "none"

  validation {
    condition     = contains(["none", "whitelist", "blacklist"], var.geo_restriction_type)
    error_message = "geo_restriction_type must be none, whitelist, or blacklist."
  }
}

variable "geo_restriction_locations" {
  description = "List of ISO 3166-1-alpha-2 country codes for geo restriction"
  type        = list(string)
  default     = []

  validation {
    condition = alltrue([
      for loc in var.geo_restriction_locations : can(regex("^[A-Z]{2}$", loc))
    ])
    error_message = "All geo_restriction_locations must be valid ISO 3166-1-alpha-2 country codes (e.g., US, GB, DE)."
  }
}

# ========================================
# Logging
# ========================================

variable "logging_enabled" {
  description = "Enable CloudFront access logging to S3"
  type        = bool
  default     = false
}

variable "logging_bucket" {
  description = "S3 bucket domain name for access logs (e.g., mybucket.s3.amazonaws.com)"
  type        = string
  default     = null
}

variable "logging_prefix" {
  description = "S3 prefix for access logs"
  type        = string
  default     = "cloudfront/"
}

variable "logging_include_cookies" {
  description = "Whether to include cookies in access logs"
  type        = bool
  default     = false
}

# ========================================
# WAF (Web Application Firewall)
# ========================================

variable "waf_enabled" {
  description = <<-EOT
    Whether to create and attach a WAF Web ACL.

    Recommended for production deployments to protect against common web attacks.
    WAF resources are created using the aws.us_east_1 provider (required for CloudFront).

    When enabled from a non-us-east-1 region, ensure you pass a properly configured
    us-east-1 provider via the providers block. See main.tf for examples.
  EOT
  type        = bool
  default     = true
}

variable "waf_name" {
  description = "Name for the WAF Web ACL. If not provided, auto-generated from resource_prefix."
  type        = string
  default     = null
}

variable "waf_default_action" {
  description = "Default action for requests not matching any rule (allow or block)"
  type        = string
  default     = "allow"

  validation {
    condition     = contains(["allow", "block"], var.waf_default_action)
    error_message = "waf_default_action must be allow or block."
  }
}

variable "waf_managed_rules" {
  description = <<-EOT
    AWS Managed Rules to enable in the WAF Web ACL.

    Common rule groups:
    - AWSManagedRulesCommonRuleSet: OWASP Top 10 protections
    - AWSManagedRulesKnownBadInputsRuleSet: Log4j, Java deserialization, etc.
    - AWSManagedRulesSQLiRuleSet: SQL injection protection
    - AWSManagedRulesLinuxRuleSet: Linux-specific protections
    - AWSManagedRulesAmazonIpReputationList: Known malicious IPs

    override_action: "none" to use rule actions, "count" to only count matches
  EOT
  type = list(object({
    name            = string
    priority        = number
    override_action = optional(string, "none")
  }))
  default = [
    { name = "AWSManagedRulesCommonRuleSet", priority = 10 },
    { name = "AWSManagedRulesKnownBadInputsRuleSet", priority = 20 },
  ]

  validation {
    condition = alltrue([
      for rule in var.waf_managed_rules : rule.priority >= 0 && rule.priority <= 2147483647
    ])
    error_message = "All WAF rule priorities must be between 0 and 2147483647."
  }

  validation {
    condition = alltrue([
      for rule in var.waf_managed_rules : contains(["none", "count"], coalesce(rule.override_action, "none"))
    ])
    error_message = "WAF rule override_action must be 'none' or 'count'."
  }
}

variable "waf_rate_limit_enabled" {
  description = "Enable rate-based rule to limit requests per IP"
  type        = bool
  default     = true
}

variable "waf_rate_limit" {
  description = "Maximum requests per 5-minute period per IP address"
  type        = number
  default     = 2000

  validation {
    condition     = var.waf_rate_limit >= 100 && var.waf_rate_limit <= 2000000000
    error_message = "waf_rate_limit must be between 100 and 2,000,000,000."
  }
}

variable "waf_rate_limit_action" {
  description = "Action when rate limit is exceeded (block or count)"
  type        = string
  default     = "block"

  validation {
    condition     = contains(["block", "count"], var.waf_rate_limit_action)
    error_message = "waf_rate_limit_action must be block or count."
  }
}

variable "waf_ip_allowlist" {
  description = <<-EOT
    IPv4 addresses/CIDR blocks to always allow (bypasses rate limiting and other rules).
    Useful for trusted internal services or monitoring systems.

    NOTE: This only applies to IPv4 traffic. If cloudfront_ipv6_enabled=true (default),
    IPv6 clients will not match this allowlist and will be subject to all other WAF rules.

    Example: ["10.0.0.0/8", "192.168.1.100/32"]
  EOT
  type        = list(string)
  default     = []

  validation {
    condition = alltrue([
      for ip in var.waf_ip_allowlist : can(cidrhost(ip, 0))
    ])
    error_message = "All entries in waf_ip_allowlist must be valid IPv4 CIDR blocks."
  }
}

variable "waf_ip_blocklist" {
  description = <<-EOT
    IPv4 addresses/CIDR blocks to always block.

    NOTE: This only applies to IPv4 traffic. If cloudfront_ipv6_enabled=true (default),
    IPv6 clients will bypass this blocklist. Set cloudfront_ipv6_enabled=false if strict
    IP blocking is required.

    Example: ["203.0.113.0/24"]
  EOT
  type        = list(string)
  default     = []

  validation {
    condition = alltrue([
      for ip in var.waf_ip_blocklist : can(cidrhost(ip, 0))
    ])
    error_message = "All entries in waf_ip_blocklist must be valid IPv4 CIDR blocks."
  }
}

variable "waf_cloudwatch_metrics_enabled" {
  description = "Enable CloudWatch metrics for WAF"
  type        = bool
  default     = true
}

variable "waf_sampled_requests_enabled" {
  description = "Enable sampling of requests for WAF"
  type        = bool
  default     = true
}

# ========================================
# Route53 DNS
# ========================================

variable "route53_zone_id" {
  description = <<-EOT
    Route53 hosted zone ID for creating DNS records.

    If provided along with route53_record_name, creates an alias record
    pointing to the CloudFront distribution.
  EOT
  type        = string
  default     = null

  validation {
    condition     = var.route53_zone_id == null || can(regex("^Z[A-Z0-9]+$", var.route53_zone_id))
    error_message = "route53_zone_id must be a valid Route53 hosted zone ID."
  }
}

variable "route53_record_name" {
  description = <<-EOT
    DNS record name to create (e.g., "gate" for gate.example.com).

    The full domain will be: {route53_record_name}.{zone_domain}
    If empty string but route53_zone_id is set, creates a record for the zone apex.
  EOT
  type        = string
  default     = null
}
