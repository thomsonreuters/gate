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
# AWS EKS module validation
# =============================================================================
#
# This file contains cross-variable validations using lifecycle preconditions.
# Single-variable validations are defined in variables.tf using validation blocks.
#
# Validations enforce:
# - Required dependencies between variables
# - Security best practices (encryption, auth tokens, multi-AZ)
# - AWS service constraints (capacity requirements, resource limits)
# - Mutual exclusivity of conflicting configurations
#
# =============================================================================

# =============================================================================
# OIDC provider
# =============================================================================

resource "terraform_data" "oidc_provider_configuration_validation" {
  lifecycle {
    precondition {
      condition     = var.eks_oidc_provider_arn != null || var.aws_account_number != null
      error_message = "Either eks_oidc_provider_arn or aws_account_number must be provided. If you provide eks_oidc_provider_arn, the module uses it directly. Otherwise, aws_account_number is required to construct the OIDC provider ARN."
    }
  }
}

# =============================================================================
# Secrets Manager KMS
# =============================================================================

resource "terraform_data" "github_apps_secret_kms_validation" {
  lifecycle {
    # Cannot provide both an existing KMS key ARN and create a new one
    precondition {
      condition     = !(var.github_apps_secret_kms_key_arn != null && var.github_apps_secret_create_kms_key)
      error_message = "Cannot set both github_apps_secret_kms_key_arn and github_apps_secret_create_kms_key. Either provide an existing KMS key ARN or let the module create one."
    }

    # Key policy file only valid when creating a new key
    precondition {
      condition     = var.github_apps_secret_kms_key_policy_file == null || var.github_apps_secret_create_kms_key
      error_message = "github_apps_secret_kms_key_policy_file can only be used when github_apps_secret_create_kms_key is true."
    }
  }
}

# =============================================================================
# Origin verification
# =============================================================================

resource "terraform_data" "origin_verify_secret_validation" {
  lifecycle {
    precondition {
      condition     = !(var.origin_verify_secret_arn != null && var.origin_verify_secret_name != null)
      error_message = "Cannot set both origin_verify_secret_arn and origin_verify_secret_name. Provide one or the other."
    }
  }
}

# =============================================================================
# DynamoDB
# =============================================================================

resource "terraform_data" "audit_table_provisioned_validation" {
  count = var.audit_dynamodb_enabled && var.audit_table_billing_mode == "PROVISIONED" ? 1 : 0

  lifecycle {
    precondition {
      condition     = var.audit_table_read_capacity > 0
      error_message = "audit_table_read_capacity must be greater than 0 when audit_table_billing_mode is PROVISIONED."
    }

    precondition {
      condition     = var.audit_table_write_capacity > 0
      error_message = "audit_table_write_capacity must be greater than 0 when audit_table_billing_mode is PROVISIONED."
    }
  }
}

resource "terraform_data" "selector_table_provisioned_validation" {
  count = var.selector_dynamodb_enabled && var.selector_table_billing_mode == "PROVISIONED" ? 1 : 0

  lifecycle {
    precondition {
      condition     = var.selector_table_read_capacity > 0
      error_message = "selector_table_read_capacity must be greater than 0 when selector_table_billing_mode is PROVISIONED."
    }

    precondition {
      condition     = var.selector_table_write_capacity > 0
      error_message = "selector_table_write_capacity must be greater than 0 when selector_table_billing_mode is PROVISIONED."
    }
  }
}

# =============================================================================
# Selector store
# =============================================================================

resource "terraform_data" "selector_backend_validation" {
  lifecycle {
    precondition {
      condition     = !(var.selector_dynamodb_enabled && var.selector_elasticache_enabled)
      error_message = "Cannot enable both selector_dynamodb_enabled and selector_elasticache_enabled. Choose exactly one selector store backend: memory (default), DynamoDB, or ElastiCache Redis."
    }
  }
}

# =============================================================================
# ElastiCache Redis
# =============================================================================

resource "terraform_data" "elasticache_network_validation" {
  count = var.selector_elasticache_enabled ? 1 : 0

  lifecycle {
    precondition {
      condition     = var.vpc_id != ""
      error_message = "vpc_id is required when selector_elasticache_enabled is true. ElastiCache must be deployed in a VPC."
    }

    precondition {
      condition     = length(var.elasticache_subnet_ids) > 0
      error_message = "elasticache_subnet_ids must contain at least one subnet when selector_elasticache_enabled is true. ElastiCache requires a subnet group."
    }

    precondition {
      condition     = var.eks_cluster_security_group_id != ""
      error_message = "eks_cluster_security_group_id is required when selector_elasticache_enabled is true. This security group is needed to allow EKS pods to communicate with ElastiCache."
    }
  }
}

resource "terraform_data" "elasticache_security_validation" {
  count = var.selector_elasticache_enabled ? 1 : 0

  lifecycle {
    # Auth token requires transit encryption (AWS requirement)
    precondition {
      condition     = !var.elasticache_auth_token_enabled || var.elasticache_transit_encryption_enabled
      error_message = "elasticache_transit_encryption_enabled must be true when elasticache_auth_token_enabled is true. Redis AUTH requires TLS encryption."
    }

    # Auth token length validation (AWS requirement: 16-128 characters)
    precondition {
      condition     = !var.elasticache_auth_token_enabled || (var.elasticache_auth_token != null && length(var.elasticache_auth_token) >= 16 && length(var.elasticache_auth_token) <= 128)
      error_message = "elasticache_auth_token must be 16-128 characters long when elasticache_auth_token_enabled is true. This is an AWS ElastiCache requirement."
    }
  }
}

resource "terraform_data" "elasticache_ha_validation" {
  count = var.selector_elasticache_enabled ? 1 : 0

  lifecycle {
    # Automatic failover requires at least 2 nodes (AWS requirement)
    precondition {
      condition     = !var.elasticache_automatic_failover_enabled || var.elasticache_num_cache_nodes >= 2
      error_message = "elasticache_num_cache_nodes must be at least 2 when elasticache_automatic_failover_enabled is true. Automatic failover requires a replica for promotion."
    }

    # Multi-AZ requires at least 2 nodes (AWS requirement)
    precondition {
      condition     = !var.elasticache_multi_az_enabled || var.elasticache_num_cache_nodes >= 2
      error_message = "elasticache_num_cache_nodes must be at least 2 when elasticache_multi_az_enabled is true. Multi-AZ deployment requires nodes in multiple availability zones."
    }

    # Production best practice: Multi-AZ and automatic failover should be enabled together
    precondition {
      condition     = var.elasticache_multi_az_enabled == var.elasticache_automatic_failover_enabled
      error_message = "For production deployments, elasticache_multi_az_enabled and elasticache_automatic_failover_enabled should both be true (or both false for dev/test). Multi-AZ without automatic failover provides no resilience benefit."
    }
  }
}
