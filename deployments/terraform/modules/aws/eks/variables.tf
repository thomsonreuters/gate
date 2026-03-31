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
# AWS
# ========================================

variable "aws_account_number" {
  description = "AWS account number used in OIDC provider ARN when eks_oidc_provider_arn is not provided. Required if eks_oidc_provider_arn is not provided."
  type        = string
  default     = null
  validation {
    condition     = var.aws_account_number == null || can(regex("^[0-9]{12}$", var.aws_account_number))
    error_message = "The AWS account number must be a 12-digit number when provided."
  }
}

# ========================================
# Tagging
# ========================================

variable "tags" {
  description = "Additional tags to apply to all resources. Use this for both organization-wide tags (e.g., environment, owner, cost-center, service-name) and deployment-specific tags. These are merged with module defaults (managed-by, deployment-type, eks-cluster, kubernetes-namespace)."
  type        = map(string)
  default     = {}

  # AWS tag limit: Maximum 50 tags per resource
  # Module adds 4 default tags (managed-by, deployment-type, eks-cluster, kubernetes-namespace)
  # Therefore, user-provided tags are limited to 46
  validation {
    condition     = length(var.tags) <= 46
    error_message = "Too many tags: ${length(var.tags)} user tags + 4 module default tags would exceed AWS limit of 50. Maximum allowed: 46 user tags."
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

  # AWS Tag Value Requirements
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
# EKS
# ========================================

variable "eks_cluster_name" {
  description = "Name of the existing EKS cluster"
  type        = string
}

variable "eks_oidc_provider_arn" {
  description = "ARN of the EKS cluster's OIDC provider (e.g., arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-east-1.amazonaws.com/id/XXXXX). If not provided, will be looked up from cluster."
  type        = string
  default     = null
}

variable "eks_oidc_provider_url" {
  description = "URL of the EKS cluster's OIDC provider (e.g., oidc.eks.us-east-1.amazonaws.com/id/XXXXX). If not provided, will be looked up from cluster."
  type        = string
  default     = null
}

# ========================================
# Kubernetes
# ========================================

variable "kubernetes_namespace" {
  description = "Kubernetes namespace where the service will be deployed"
  type        = string
  default     = "gate"

  validation {
    condition     = can(regex("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$", var.kubernetes_namespace))
    error_message = "kubernetes_namespace must be a valid Kubernetes namespace name (lowercase alphanumeric and hyphens, must start and end with alphanumeric)."
  }
}

variable "kubernetes_service_account_name" {
  description = "Kubernetes service account name that will assume the IAM role"
  type        = string
  default     = "gate"

  validation {
    condition     = can(regex("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$", var.kubernetes_service_account_name))
    error_message = "kubernetes_service_account_name must be a valid Kubernetes service account name (lowercase alphanumeric and hyphens, must start and end with alphanumeric)."
  }
}

# ========================================
# IAM
# ========================================

variable "iam_role_name" {
  description = "IAM role name for IRSA (optional, auto-generated if not provided)"
  type        = string
  default     = null
}

variable "iam_role_prefix" {
  description = "Path prefix for IAM roles"
  type        = string
  default     = "/service-role/"
}

variable "permissions_boundary_arn" {
  description = "ARN of the IAM permissions boundary policy to attach to roles. Leave empty to disable permissions boundary."
  type        = string
  default     = ""
}

# ========================================
# Application
# ========================================

variable "service_name" {
  description = "Name of the service (used for resource naming)"
  type        = string
  default     = "gate"
}

variable "resource_prefix" {
  description = "Prefix for all resource names. If not provided, defaults to service_name."
  type        = string
  default     = ""
}

# ========================================
# GitHub Apps
# ========================================

variable "github_apps_secret_name" {
  description = "Secrets Manager secret name for GitHub Apps (optional, auto-generated if not provided)"
  type        = string
  default     = null
}

variable "github_apps_secret_file" {
  description = <<-EOT
    Path to a local JSON file containing GitHub Apps credentials.

    Optional: If not provided, a placeholder value will be used for initial secret creation.

    Recommended workflow:
    1. Run terraform apply (with or without this file) to create infrastructure
    2. Use the provided script to inject GitHub Apps credentials:
       ./deployments/terraform/scripts/aws/push-github-apps-to-secrets-manager.sh
    3. Future terraform runs will not overwrite the secret (lifecycle ignore_changes)
  EOT
  type        = string
  sensitive   = true
  default     = null

  validation {
    condition     = var.github_apps_secret_file == null || can(file(var.github_apps_secret_file))
    error_message = "The file specified in github_apps_secret_file does not exist or cannot be read. Please provide a valid file path."
  }

  validation {
    condition     = var.github_apps_secret_file == null || can(jsondecode(file(var.github_apps_secret_file)))
    error_message = "The file specified in github_apps_secret_file does not contain valid JSON. Please ensure the file contains properly formatted JSON."
  }
}

variable "github_apps_secret_policy_file" {
  description = <<-EOT
    Path to a local JSON file containing a resource policy for the GitHub Apps Secrets Manager secret.

    Optional: If not provided, no resource policy is attached to the secret.
  EOT
  type        = string
  default     = null

  validation {
    condition     = var.github_apps_secret_policy_file == null || can(file(var.github_apps_secret_policy_file))
    error_message = "The file specified in github_apps_secret_policy_file does not exist or cannot be read. Please provide a valid file path."
  }

  validation {
    condition     = var.github_apps_secret_policy_file == null || can(jsondecode(file(var.github_apps_secret_policy_file)))
    error_message = "The file specified in github_apps_secret_policy_file does not contain valid JSON. Please ensure the file contains a properly formatted IAM policy document."
  }
}

variable "github_apps_secret_recovery_window" {
  description = "Number of days to retain secret after deletion (0 for immediate deletion)"
  type        = number
  default     = 7
  validation {
    condition     = var.github_apps_secret_recovery_window == 0 || (var.github_apps_secret_recovery_window >= 7 && var.github_apps_secret_recovery_window <= 30)
    error_message = "Recovery window must be 0 (immediate deletion) or between 7 and 30 days."
  }
}

variable "github_apps_secret_kms_key_arn" {
  description = <<-EOT
    ARN of an existing customer-managed KMS key for encrypting the GitHub Apps Secrets Manager secret.

    If not provided and github_apps_secret_create_kms_key is false, Secrets Manager uses
    the AWS-managed key (aws/secretsmanager).

    Cannot be used together with github_apps_secret_create_kms_key.
  EOT
  type        = string
  default     = null

  validation {
    condition     = var.github_apps_secret_kms_key_arn == null || can(regex("^arn:aws:kms:[a-z0-9-]+:[0-9]{12}:key/[a-f0-9-]+$", var.github_apps_secret_kms_key_arn))
    error_message = "github_apps_secret_kms_key_arn must be a valid KMS key ARN (e.g., arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012)."
  }
}

variable "github_apps_secret_create_kms_key" {
  description = <<-EOT
    Whether to create a new customer-managed KMS key for encrypting the GitHub Apps Secrets Manager secret.

    When true, the module creates a KMS key with automatic key rotation enabled.
    Cannot be used together with github_apps_secret_kms_key_arn.
  EOT
  type        = bool
  default     = false
}

variable "github_apps_secret_kms_key_policy_file" {
  description = <<-EOT
    Path to a local JSON file containing a key policy for the module-created KMS key.

    Optional: Only valid when github_apps_secret_create_kms_key is true.
    If not provided, the KMS key uses the default key policy.
  EOT
  type        = string
  default     = null

  validation {
    condition     = var.github_apps_secret_kms_key_policy_file == null || can(file(var.github_apps_secret_kms_key_policy_file))
    error_message = "The file specified in github_apps_secret_kms_key_policy_file does not exist or cannot be read. Please provide a valid file path."
  }

  validation {
    condition     = var.github_apps_secret_kms_key_policy_file == null || can(jsondecode(file(var.github_apps_secret_kms_key_policy_file)))
    error_message = "The file specified in github_apps_secret_kms_key_policy_file does not contain valid JSON. Please ensure the file contains a properly formatted KMS key policy document."
  }
}

# ========================================
# Origin Verification
# ========================================

variable "origin_verify_secret_arn" {
  description = <<-EOT
    ARN of an existing AWS Secrets Manager secret for CloudFront origin verification.

    When provided, IAM permissions are granted for the IRSA role to read this secret,
    allowing the External Secrets Operator to sync it to Kubernetes.

    The secret should be created separately (e.g., via the dev-origin-secret module
    or AWS CLI) and shared between CloudFront and the application.

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
    Name of an existing AWS Secrets Manager secret for CloudFront origin verification.

    Convenience alternative to origin_verify_secret_arn. When provided, the module
    looks up the secret ARN by name and grants IAM permissions for the IRSA role
    to read it.

    Mutually exclusive with origin_verify_secret_arn.
  EOT
  type        = string
  default     = null
}

# ========================================
# Audit backend (DynamoDB)
# ========================================

variable "audit_dynamodb_enabled" {
  description = "Whether to create DynamoDB table for audit logs"
  type        = bool
  default     = true
}

variable "audit_table_name" {
  description = "DynamoDB table name for audit logs (optional, auto-generated if not provided)"
  type        = string
  default     = null
}

variable "audit_table_billing_mode" {
  description = "DynamoDB billing mode (PROVISIONED or PAY_PER_REQUEST)"
  type        = string
  default     = "PAY_PER_REQUEST"
  validation {
    condition     = contains(["PROVISIONED", "PAY_PER_REQUEST"], var.audit_table_billing_mode)
    error_message = "Billing mode must be PROVISIONED or PAY_PER_REQUEST."
  }
}

variable "audit_table_read_capacity" {
  description = "DynamoDB read capacity units (only for PROVISIONED mode)"
  type        = number
  default     = 5
}

variable "audit_table_write_capacity" {
  description = "DynamoDB write capacity units (only for PROVISIONED mode)"
  type        = number
  default     = 5
}

variable "audit_table_ttl_enabled" {
  description = "Enable TTL for audit log entries"
  type        = bool
  default     = true
}

variable "audit_table_point_in_time_recovery" {
  description = "Enable point-in-time recovery for DynamoDB table"
  type        = bool
  default     = true
}

variable "audit_table_kms_key_arn" {
  description = "ARN of a customer-managed KMS key for DynamoDB encryption. If not provided, AWS-managed keys are used."
  type        = string
  default     = null
}

# ========================================
# Selector store (DynamoDB)
# ========================================

variable "selector_dynamodb_enabled" {
  description = "Whether to create DynamoDB table for selector rate limit state (set to true to use DynamoDB as selector store)"
  type        = bool
  default     = false
}

variable "selector_table_name" {
  description = "DynamoDB table name for selector rate limit state (optional, auto-generated if not provided)"
  type        = string
  default     = null
}

variable "selector_table_billing_mode" {
  description = "DynamoDB billing mode (PROVISIONED or PAY_PER_REQUEST)"
  type        = string
  default     = "PAY_PER_REQUEST"
  validation {
    condition     = contains(["PROVISIONED", "PAY_PER_REQUEST"], var.selector_table_billing_mode)
    error_message = "Billing mode must be PROVISIONED or PAY_PER_REQUEST."
  }
}

variable "selector_table_read_capacity" {
  description = "DynamoDB read capacity units (only for PROVISIONED mode)"
  type        = number
  default     = 5
}

variable "selector_table_write_capacity" {
  description = "DynamoDB write capacity units (only for PROVISIONED mode)"
  type        = number
  default     = 5
}

variable "selector_table_ttl_enabled" {
  description = "Enable TTL for selector rate limit state entries"
  type        = bool
  default     = true
}

variable "selector_table_point_in_time_recovery" {
  description = "Enable point-in-time recovery for DynamoDB table"
  type        = bool
  default     = false
}

variable "selector_table_kms_key_arn" {
  description = "ARN of a customer-managed KMS key for DynamoDB encryption. If not provided, AWS-managed keys are used."
  type        = string
  default     = null
}

# ========================================
# Selector store (ElastiCache Redis)
# ========================================

variable "selector_elasticache_enabled" {
  description = "Whether to create ElastiCache Redis cluster for selector rate limit state (set to true to use Redis as selector store)"
  type        = bool
  default     = false
}

variable "elasticache_node_type" {
  description = "ElastiCache node type (e.g., cache.t4g.micro, cache.r7g.large)"
  type        = string
  default     = "cache.t4g.micro"
}

variable "elasticache_num_cache_nodes" {
  description = "Number of cache nodes (1 for standalone, 2+ for replication)"
  type        = number
  default     = 1
  validation {
    condition     = var.elasticache_num_cache_nodes >= 1 && var.elasticache_num_cache_nodes <= 6
    error_message = "Number of cache nodes must be between 1 and 6."
  }
}

variable "elasticache_engine_version" {
  description = "Redis engine version"
  type        = string
  default     = "7.0"
}

variable "elasticache_parameter_group_name" {
  description = "ElastiCache parameter group name (optional, uses default for engine version if not provided)"
  type        = string
  default     = "default.redis7"
}

variable "elasticache_multi_az_enabled" {
  description = "Enable Multi-AZ for high availability (requires num_cache_nodes >= 2)"
  type        = bool
  default     = false
}

variable "elasticache_automatic_failover_enabled" {
  description = "Enable automatic failover (requires Multi-AZ and num_cache_nodes >= 2)"
  type        = bool
  default     = false
}

variable "elasticache_at_rest_encryption_enabled" {
  description = "Enable encryption at rest"
  type        = bool
  default     = true
}

variable "elasticache_transit_encryption_enabled" {
  description = "Enable encryption in transit (TLS)"
  type        = bool
  default     = true
}

variable "elasticache_auth_token_enabled" {
  description = "Enable Redis AUTH token (password) for authentication"
  type        = bool
  default     = true
}

variable "elasticache_auth_token" {
  description = "Redis AUTH token (password). Required if auth_token_enabled is true. Must be 16-128 alphanumeric characters."
  type        = string
  default     = null
  sensitive   = true
}

variable "elasticache_maintenance_window" {
  description = "Maintenance window (e.g., sun:05:00-sun:06:00)"
  type        = string
  default     = "sun:05:00-sun:06:00"
}

variable "elasticache_snapshot_retention_limit" {
  description = "Number of days to retain automatic snapshots (0 to disable)"
  type        = number
  default     = 5
  validation {
    condition     = var.elasticache_snapshot_retention_limit >= 0 && var.elasticache_snapshot_retention_limit <= 35
    error_message = "Snapshot retention must be between 0 and 35 days."
  }
}

variable "elasticache_snapshot_window" {
  description = "Daily time range for snapshots (e.g., 03:00-04:00)"
  type        = string
  default     = "03:00-04:00"
}

variable "elasticache_auto_minor_version_upgrade" {
  description = "Enable automatic minor version upgrades"
  type        = bool
  default     = true
}

variable "elasticache_subnet_ids" {
  description = "List of subnet IDs for ElastiCache subnet group (must be in same VPC as EKS cluster)"
  type        = list(string)
  default     = []
}

variable "vpc_id" {
  description = "VPC ID where ElastiCache will be created (required if elasticache_enabled is true)"
  type        = string
  default     = ""
}

variable "eks_cluster_security_group_id" {
  description = "EKS cluster security group ID to allow Redis access from (required if elasticache_enabled is true)"
  type        = string
  default     = ""
}

# ========================================
# CloudWatch logs
# ========================================

variable "cloudwatch_log_group_name" {
  description = "CloudWatch log group name (optional, auto-generated if not provided)"
  type        = string
  default     = null
}

variable "cloudwatch_log_retention_days" {
  description = "CloudWatch logs retention period in days"
  type        = number
  default     = 30
  validation {
    condition     = contains([1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1096, 1827, 2192, 2557, 2922, 3288, 3653], var.cloudwatch_log_retention_days)
    error_message = "Log retention days must be a valid CloudWatch logs retention value."
  }
}

# ========================================
# CloudWatch alarms
# ========================================

variable "alarm_sns_topic_arn" {
  description = "SNS topic ARN for CloudWatch alarms (optional). If not provided, alarms will not be created."
  type        = string
  default     = ""
}

variable "alarm_dynamodb_read_throttle_threshold" {
  description = "Threshold for DynamoDB read throttle alarm (number of throttled requests)"
  type        = number
  default     = 5
}

variable "alarm_dynamodb_write_throttle_threshold" {
  description = "Threshold for DynamoDB write throttle alarm (number of throttled requests)"
  type        = number
  default     = 5
}

variable "alarm_dynamodb_system_errors_threshold" {
  description = "Threshold for DynamoDB system errors alarm"
  type        = number
  default     = 1
}
