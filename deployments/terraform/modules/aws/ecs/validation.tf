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
# AWS ECS module validation
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
# - Fargate CPU/memory compatibility
#
# =============================================================================

# =============================================================================
# ECS cluster
# =============================================================================

resource "terraform_data" "ecs_cluster_validation" {
  lifecycle {
    precondition {
      condition     = var.create_ecs_cluster || var.ecs_cluster_name != null
      error_message = "When create_ecs_cluster is false, ecs_cluster_name must be provided to reference an existing cluster."
    }
  }
}

# =============================================================================
# Fargate CPU/Memory compatibility
# =============================================================================

resource "terraform_data" "fargate_cpu_memory_validation" {
  lifecycle {
    # Fargate has specific valid CPU/memory combinations
    # https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-cpu-memory-error.html
    precondition {
      condition = (
        (var.ecs_task_cpu == 256 && var.ecs_task_memory >= 512 && var.ecs_task_memory <= 2048) ||
        (var.ecs_task_cpu == 512 && var.ecs_task_memory >= 1024 && var.ecs_task_memory <= 4096) ||
        (var.ecs_task_cpu == 1024 && var.ecs_task_memory >= 2048 && var.ecs_task_memory <= 8192) ||
        (var.ecs_task_cpu == 2048 && var.ecs_task_memory >= 4096 && var.ecs_task_memory <= 16384) ||
        (var.ecs_task_cpu == 4096 && var.ecs_task_memory >= 8192 && var.ecs_task_memory <= 30720) ||
        (var.ecs_task_cpu == 8192 && var.ecs_task_memory >= 16384 && var.ecs_task_memory <= 61440) ||
        (var.ecs_task_cpu == 16384 && var.ecs_task_memory >= 32768 && var.ecs_task_memory <= 122880)
      )
      error_message = <<-EOT
        Invalid Fargate CPU/memory combination. Valid combinations are:
        - 256 CPU: 512-2048 MiB memory
        - 512 CPU: 1024-4096 MiB memory
        - 1024 CPU: 2048-8192 MiB memory
        - 2048 CPU: 4096-16384 MiB memory (in 1024 MiB increments)
        - 4096 CPU: 8192-30720 MiB memory (in 1024 MiB increments)
        - 8192 CPU: 16384-61440 MiB memory (in 4096 MiB increments)
        - 16384 CPU: 32768-122880 MiB memory (in 8192 MiB increments)
        Current: ${var.ecs_task_cpu} CPU, ${var.ecs_task_memory} MiB memory
      EOT
    }
  }
}

# =============================================================================
# Load Balancer
# =============================================================================

resource "terraform_data" "alb_public_subnets_validation" {
  count = var.alb_enabled ? 1 : 0

  lifecycle {
    precondition {
      condition     = length(var.public_subnet_ids) >= 2 || var.alb_internal
      error_message = "When alb_enabled is true and alb_internal is false, at least 2 public subnet IDs must be provided for the internet-facing ALB."
    }
  }
}

resource "terraform_data" "alb_certificate_validation" {
  count = var.alb_enabled && var.alb_listener_protocol == "HTTPS" ? 1 : 0

  lifecycle {
    precondition {
      condition     = var.alb_certificate_arn != ""
      error_message = "alb_certificate_arn is required when alb_listener_protocol is HTTPS."
    }
  }
}

resource "terraform_data" "alb_access_logs_validation" {
  count = var.alb_enabled && var.alb_access_logs_enabled ? 1 : 0

  lifecycle {
    precondition {
      condition     = var.alb_access_logs_bucket != ""
      error_message = "alb_access_logs_bucket is required when alb_access_logs_enabled is true."
    }
  }
}

resource "terraform_data" "alb_ingress_validation" {
  count = var.alb_enabled ? 1 : 0

  lifecycle {
    precondition {
      condition     = length(var.alb_ingress_cidr_blocks) > 0 || length(var.alb_ingress_security_group_ids) > 0
      error_message = "At least one of alb_ingress_cidr_blocks or alb_ingress_security_group_ids must be provided when alb_enabled is true. This defines who can access the ALB."
    }
  }
}

# =============================================================================
# Auto scaling
# =============================================================================

resource "terraform_data" "autoscaling_capacity_validation" {
  count = var.autoscaling_enabled ? 1 : 0

  lifecycle {
    precondition {
      condition     = var.autoscaling_min_capacity <= var.autoscaling_max_capacity
      error_message = "autoscaling_min_capacity (${var.autoscaling_min_capacity}) must be less than or equal to autoscaling_max_capacity (${var.autoscaling_max_capacity})."
    }

    precondition {
      condition     = var.ecs_service_desired_count >= var.autoscaling_min_capacity && var.ecs_service_desired_count <= var.autoscaling_max_capacity
      error_message = "ecs_service_desired_count (${var.ecs_service_desired_count}) must be between autoscaling_min_capacity (${var.autoscaling_min_capacity}) and autoscaling_max_capacity (${var.autoscaling_max_capacity})."
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

# =============================================================================
# Deployment configuration
# =============================================================================

resource "terraform_data" "deployment_configuration_validation" {
  lifecycle {
    precondition {
      condition     = var.ecs_service_deployment_minimum_healthy_percent < var.ecs_service_deployment_maximum_percent
      error_message = "ecs_service_deployment_minimum_healthy_percent (${var.ecs_service_deployment_minimum_healthy_percent}) must be less than ecs_service_deployment_maximum_percent (${var.ecs_service_deployment_maximum_percent})."
    }
  }
}

# =============================================================================
# Health check configuration
# =============================================================================

resource "terraform_data" "health_check_validation" {
  count = var.alb_enabled ? 1 : 0

  lifecycle {
    precondition {
      condition     = var.alb_health_check_timeout < var.alb_health_check_interval
      error_message = "alb_health_check_timeout (${var.alb_health_check_timeout}s) must be less than alb_health_check_interval (${var.alb_health_check_interval}s)."
    }
  }
}
