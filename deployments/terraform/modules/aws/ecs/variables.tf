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

variable "aws_region" {
  description = "AWS region where resources will be deployed"
  type        = string
  default     = "us-east-1"
  validation {
    condition     = can(regex("^(af|ap|ca|eu|me|sa|us)-(central|north|(north(?:east|west))|south|south(?:east|west)|east|west)-\\d+$", var.aws_region))
    error_message = "You must enter a valid AWS region."
  }
}

# ========================================
# Tagging
# ========================================

variable "tags" {
  description = "Additional tags to apply to all resources. Use this for both organization-wide tags (e.g., environment, owner, cost-center, service-name) and deployment-specific tags. These are merged with module defaults (managed-by, deployment-type, ecs-cluster, ecs-service)."
  type        = map(string)
  default     = {}

  # AWS tag limit: Maximum 50 tags per resource
  # Module adds 4 default tags (managed-by, deployment-type, ecs-cluster, ecs-service)
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
# ECS cluster
# ========================================

variable "ecs_cluster_name" {
  description = "Name of the existing ECS cluster. If not provided, a new cluster will be created."
  type        = string
  default     = null
}

variable "create_ecs_cluster" {
  description = "Whether to create a new ECS cluster. If false, ecs_cluster_name must reference an existing cluster."
  type        = bool
  default     = true
}

variable "ecs_cluster_capacity_providers" {
  description = "List of capacity providers to associate with the cluster. Valid values: FARGATE, FARGATE_SPOT, or EC2 Auto Scaling group capacity provider names."
  type        = list(string)
  default     = ["FARGATE", "FARGATE_SPOT"]

  validation {
    condition     = length(var.ecs_cluster_capacity_providers) > 0
    error_message = "At least one capacity provider must be specified."
  }
}

variable "ecs_cluster_default_capacity_provider" {
  description = "Default capacity provider strategy for the cluster"
  type = object({
    capacity_provider = string
    weight            = optional(number, 1)
    base              = optional(number, 0)
  })
  default = {
    capacity_provider = "FARGATE"
    weight            = 1
    base              = 0
  }
}

variable "ecs_cluster_container_insights" {
  description = "Enable CloudWatch Container Insights for the ECS cluster"
  type        = bool
  default     = true
}

# ========================================
# ECS service
# ========================================

variable "ecs_service_name" {
  description = "Name of the ECS service (optional, defaults to service_name)"
  type        = string
  default     = null
}

variable "ecs_service_desired_count" {
  description = <<-EOT
    Desired number of tasks running in the service.

    When autoscaling is enabled, this value is used for initial deployment only.
    The autoscaler will then manage the task count within the configured
    min/max capacity bounds.
  EOT
  type        = number
  default     = 2

  validation {
    condition     = var.ecs_service_desired_count >= 0 && var.ecs_service_desired_count <= 1000
    error_message = "Desired count must be between 0 and 1000."
  }
}

variable "ecs_service_deployment_minimum_healthy_percent" {
  description = "Minimum healthy percent during deployment (lower bound of running tasks during deployment)"
  type        = number
  default     = 100

  validation {
    condition     = var.ecs_service_deployment_minimum_healthy_percent >= 0 && var.ecs_service_deployment_minimum_healthy_percent <= 200
    error_message = "Deployment minimum healthy percent must be between 0 and 200."
  }
}

variable "ecs_service_deployment_maximum_percent" {
  description = "Maximum percent during deployment (upper bound of running tasks during deployment)"
  type        = number
  default     = 200

  validation {
    condition     = var.ecs_service_deployment_maximum_percent >= 100 && var.ecs_service_deployment_maximum_percent <= 200
    error_message = "Deployment maximum percent must be between 100 and 200."
  }
}

variable "ecs_service_enable_execute_command" {
  description = <<-EOT
    Enable ECS Exec for interactive debugging sessions.

    Note: Requires a shell (/bin/sh) in the container image.
    This feature will not work with distroless images unless using
    debug variants.
  EOT
  type        = bool
  default     = false
}

variable "ecs_service_force_new_deployment" {
  description = "Force a new deployment of the service when Terraform applies"
  type        = bool
  default     = false
}

variable "ecs_service_wait_for_steady_state" {
  description = "Wait for the service to reach a steady state (all tasks running) during Terraform apply"
  type        = bool
  default     = true
}

# ========================================
# ECS task definition
# ========================================

variable "ecs_task_cpu" {
  description = "CPU units for the task (256, 512, 1024, 2048, 4096, 8192, 16384)"
  type        = number
  default     = 512

  validation {
    condition     = contains([256, 512, 1024, 2048, 4096, 8192, 16384], var.ecs_task_cpu)
    error_message = "Task CPU must be one of: 256, 512, 1024, 2048, 4096, 8192, 16384."
  }
}

variable "ecs_task_memory" {
  description = "Memory (in MiB) for the task. Valid values depend on CPU (see AWS documentation: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definition_parameters.html#task_size)."
  type        = number
  default     = 1024

  validation {
    condition     = var.ecs_task_memory >= 512 && var.ecs_task_memory <= 122880
    error_message = "Task memory must be between 512 MiB and 122880 MiB (120 GB)."
  }
}

variable "ecs_task_architecture" {
  description = "CPU architecture for the task (X86_64 or ARM64)"
  type        = string
  default     = "X86_64"

  validation {
    condition     = contains(["X86_64", "ARM64"], var.ecs_task_architecture)
    error_message = "Task architecture must be X86_64 or ARM64."
  }
}

variable "ecs_task_operating_system_family" {
  description = "Operating system family for the task"
  type        = string
  default     = "LINUX"

  validation {
    condition     = contains(["LINUX", "WINDOWS_SERVER_2019_FULL", "WINDOWS_SERVER_2019_CORE", "WINDOWS_SERVER_2022_FULL", "WINDOWS_SERVER_2022_CORE", "WINDOWS_SERVER_2025_FULL", "WINDOWS_SERVER_2025_CORE"], var.ecs_task_operating_system_family)
    error_message = "Operating system family must be LINUX or a valid Windows Server version."
  }
}

# ========================================
# Container configuration
# ========================================

variable "container_image" {
  description = "Container image to deploy (e.g., account.dkr.ecr.region.amazonaws.com/repo:tag)"
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9._/-]*:[a-zA-Z0-9._-]+$", var.container_image)) || can(regex("^[0-9]+\\.dkr\\.ecr\\.[a-z0-9-]+\\.amazonaws\\.com/[a-z0-9._/-]+:[a-zA-Z0-9._-]+$", var.container_image))
    error_message = "Container image must be a valid Docker image reference (e.g., repository:tag or ECR URI)."
  }
}

variable "container_port" {
  description = "Port the container exposes"
  type        = number
  default     = 8080

  validation {
    condition     = var.container_port >= 1 && var.container_port <= 65535
    error_message = "Container port must be between 1 and 65535."
  }
}

variable "container_cpu" {
  description = "CPU units reserved for the container (optional, defaults to task CPU)"
  type        = number
  default     = null

  validation {
    condition     = var.container_cpu == null || (var.container_cpu >= 0 && var.container_cpu <= 16384)
    error_message = "Container CPU must be between 0 and 16384 when specified."
  }
}

variable "container_memory_reservation" {
  description = "Soft memory limit (in MiB) for the container"
  type        = number
  default     = null

  validation {
    condition     = var.container_memory_reservation == null || (var.container_memory_reservation >= 4 && var.container_memory_reservation <= 122880)
    error_message = "Container memory reservation must be between 4 MiB and 122880 MiB when specified."
  }
}

variable "container_environment" {
  description = "Environment variables for the container"
  type = list(object({
    name  = string
    value = string
  }))
  default = []
}

variable "container_secrets" {
  description = "Secrets to inject as environment variables (from Secrets Manager or SSM Parameter Store)"
  type = list(object({
    name      = string
    valueFrom = string
  }))
  default = []
}

variable "container_health_check" {
  description = <<-EOT
    Container health check configuration. Set to null to disable.

    Note: CMD-SHELL requires a shell (/bin/sh) and the specified command
    (e.g., curl) to exist in the image. For distroless images, either:
    - Set to null and rely on ALB health checks (recommended)
    - Use CMD-SHELL with a binary that exists in your image
  EOT
  type = object({
    command     = list(string)
    interval    = optional(number, 30)
    timeout     = optional(number, 5)
    retries     = optional(number, 3)
    startPeriod = optional(number, 60)
  })
  default = null
}

variable "container_readonly_root_filesystem" {
  description = "Mount the container's root filesystem as read-only"
  type        = bool
  default     = true
}

variable "container_stop_timeout" {
  description = "Time duration (in seconds) to wait before the container is forcefully killed"
  type        = number
  default     = 30

  validation {
    condition     = var.container_stop_timeout >= 0 && var.container_stop_timeout <= 120
    error_message = "Container stop timeout must be between 0 and 120 seconds."
  }
}

# ========================================
# Networking
# ========================================

variable "vpc_id" {
  description = "VPC ID where ECS will be deployed"
  type        = string

  validation {
    condition     = can(regex("^vpc-([0-9a-f]{8}|[0-9a-f]{17})$", var.vpc_id))
    error_message = "VPC ID must be a valid AWS VPC ID ('vpc-' followed by 8 or 17 alphanumeric characters in lowercase)."
  }
}

variable "private_subnet_ids" {
  description = "List of private subnet IDs for ECS tasks"
  type        = list(string)

  validation {
    condition     = length(var.private_subnet_ids) >= 1
    error_message = "At least one private subnet ID must be provided."
  }

  validation {
    condition     = alltrue([for s in var.private_subnet_ids : can(regex("^subnet-([0-9a-f]{8}|[0-9a-f]{17})$", s))])
    error_message = "All subnet IDs must be valid AWS subnet IDs ('subnet-' followed by 8 or 17 alphanumeric characters in lowercase)."
  }
}

variable "public_subnet_ids" {
  description = "List of public subnet IDs for the Application Load Balancer (required if ALB is enabled)"
  type        = list(string)
  default     = []

  validation {
    condition     = alltrue([for s in var.public_subnet_ids : can(regex("^subnet-([0-9a-f]{8}|[0-9a-f]{17})$", s))])
    error_message = "All subnet IDs must be valid AWS subnet IDs ('subnet-' followed by 8 or 17 alphanumeric characters in lowercase)."
  }
}

variable "assign_public_ip" {
  description = "Assign public IP to ECS tasks (required for Fargate tasks in public subnets without NAT)"
  type        = bool
  default     = false
}

# ========================================
# Load Balancer
# ========================================

variable "alb_enabled" {
  description = "Whether to create an Application Load Balancer for the ECS service"
  type        = bool
  default     = true
}

variable "alb_internal" {
  description = "Whether the ALB should be internal (private) or internet-facing"
  type        = bool
  default     = true
}

variable "alb_name" {
  description = "Name for the Application Load Balancer (optional, auto-generated if not provided)"
  type        = string
  default     = null
}

variable "alb_idle_timeout" {
  description = "Idle timeout for the ALB in seconds"
  type        = number
  default     = 60

  validation {
    condition     = var.alb_idle_timeout >= 1 && var.alb_idle_timeout <= 4000
    error_message = "ALB idle timeout must be between 1 and 4000 seconds."
  }
}

variable "alb_deletion_protection" {
  description = "Enable deletion protection for the ALB"
  type        = bool
  default     = false
}

variable "alb_drop_invalid_header_fields" {
  description = "Enable dropping of invalid header fields by the ALB"
  type        = bool
  default     = true
}

variable "alb_access_logs_enabled" {
  description = "Enable access logging for the ALB"
  type        = bool
  default     = false
}

variable "alb_access_logs_bucket" {
  description = "S3 bucket name for ALB access logs (required if access logs are enabled)"
  type        = string
  default     = ""
}

variable "alb_access_logs_prefix" {
  description = "S3 prefix for ALB access logs"
  type        = string
  default     = ""
}

variable "alb_listener_port" {
  description = "Port for the ALB listener"
  type        = number
  default     = 443

  validation {
    condition     = var.alb_listener_port >= 1 && var.alb_listener_port <= 65535
    error_message = "ALB listener port must be between 1 and 65535."
  }
}

variable "alb_listener_protocol" {
  description = "Protocol for the ALB listener (HTTP or HTTPS)"
  type        = string
  default     = "HTTPS"

  validation {
    condition     = contains(["HTTP", "HTTPS"], var.alb_listener_protocol)
    error_message = "ALB listener protocol must be HTTP or HTTPS."
  }
}

variable "alb_certificate_arn" {
  description = "ARN of the ACM certificate for HTTPS listener (required if listener protocol is HTTPS)"
  type        = string
  default     = ""
}

variable "alb_ssl_policy" {
  description = "SSL policy for the HTTPS listener. Only TLS 1.2+ policies are allowed."
  type        = string
  default     = "ELBSecurityPolicy-TLS13-1-2-2021-06"

  validation {
    condition = contains([
      # TLS 1.3 only
      "ELBSecurityPolicy-TLS13-1-3-2021-06",
      "ELBSecurityPolicy-TLS13-1-3-PQ-2025-09",
      # TLS 1.3 + 1.2
      "ELBSecurityPolicy-TLS13-1-2-2021-06",
      "ELBSecurityPolicy-TLS13-1-2-PQ-2025-09",
      "ELBSecurityPolicy-TLS13-1-2-Res-2021-06",
      "ELBSecurityPolicy-TLS13-1-2-Res-PQ-2025-09",
      "ELBSecurityPolicy-TLS13-1-2-Ext1-2021-06",
      "ELBSecurityPolicy-TLS13-1-2-Ext1-PQ-2025-09",
      "ELBSecurityPolicy-TLS13-1-2-Ext2-2021-06",
      "ELBSecurityPolicy-TLS13-1-2-Ext2-PQ-2025-09",
      # TLS 1.2 only
      "ELBSecurityPolicy-TLS-1-2-2017-01",
      "ELBSecurityPolicy-TLS-1-2-Ext-2018-06",
    ], var.alb_ssl_policy)
    error_message = "SSL policy must enforce TLS 1.2 minimum. TLS 1.0 and 1.1 policies are not allowed."
  }
}

variable "alb_health_check_path" {
  description = "Path for ALB target group health checks"
  type        = string
  default     = "/health"
}

variable "alb_health_check_interval" {
  description = "Interval between ALB health checks (seconds)"
  type        = number
  default     = 30

  validation {
    condition     = var.alb_health_check_interval >= 5 && var.alb_health_check_interval <= 300
    error_message = "Health check interval must be between 5 and 300 seconds."
  }
}

variable "alb_health_check_timeout" {
  description = "Timeout for ALB health checks (seconds)"
  type        = number
  default     = 5

  validation {
    condition     = var.alb_health_check_timeout >= 2 && var.alb_health_check_timeout <= 120
    error_message = "Health check timeout must be between 2 and 120 seconds."
  }
}

variable "alb_health_check_healthy_threshold" {
  description = "Number of consecutive health checks before marking healthy"
  type        = number
  default     = 2

  validation {
    condition     = var.alb_health_check_healthy_threshold >= 2 && var.alb_health_check_healthy_threshold <= 10
    error_message = "Healthy threshold must be between 2 and 10."
  }
}

variable "alb_health_check_unhealthy_threshold" {
  description = "Number of consecutive health checks before marking unhealthy"
  type        = number
  default     = 3

  validation {
    condition     = var.alb_health_check_unhealthy_threshold >= 2 && var.alb_health_check_unhealthy_threshold <= 10
    error_message = "Unhealthy threshold must be between 2 and 10."
  }
}

variable "alb_deregistration_delay" {
  description = "Time to wait for in-flight requests to complete when deregistering targets (seconds)"
  type        = number
  default     = 30

  validation {
    condition     = var.alb_deregistration_delay >= 0 && var.alb_deregistration_delay <= 3600
    error_message = "Deregistration delay must be between 0 and 3600 seconds."
  }
}

variable "alb_ingress_cidr_blocks" {
  description = "CIDR blocks allowed to access the ALB (use ['0.0.0.0/0'] for public access)"
  type        = list(string)
  default     = []

  validation {
    condition     = alltrue([for cidr in var.alb_ingress_cidr_blocks : can(cidrhost(cidr, 0))])
    error_message = "All values must be valid CIDR blocks."
  }
}

variable "alb_ingress_security_group_ids" {
  description = "Security group IDs allowed to access the ALB"
  type        = list(string)
  default     = []

  validation {
    condition     = alltrue([for sg in var.alb_ingress_security_group_ids : can(regex("^sg-([0-9a-f]{8}|[0-9a-f]{17})$", sg))])
    error_message = "All security group IDs must be valid AWS security group IDs ('sg-' followed by 8 or 17 alphanumeric characters in lowercase)."
  }
}

# ========================================
# IAM
# ========================================

variable "iam_task_role_name" {
  description = "IAM task role name (optional, auto-generated if not provided)"
  type        = string
  default     = null
}

variable "iam_execution_role_name" {
  description = "IAM task execution role name (optional, auto-generated if not provided)"
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
# Auto scaling
# ========================================

variable "autoscaling_enabled" {
  description = "Enable auto scaling for the ECS service"
  type        = bool
  default     = true
}

variable "autoscaling_min_capacity" {
  description = "Minimum number of tasks"
  type        = number
  default     = 2

  validation {
    condition     = var.autoscaling_min_capacity >= 0 && var.autoscaling_min_capacity <= 1000
    error_message = "Minimum capacity must be between 0 and 1000."
  }
}

variable "autoscaling_max_capacity" {
  description = "Maximum number of tasks"
  type        = number
  default     = 10

  validation {
    condition     = var.autoscaling_max_capacity >= 1 && var.autoscaling_max_capacity <= 1000
    error_message = "Maximum capacity must be between 1 and 1000."
  }
}

variable "autoscaling_cpu_target" {
  description = "Target CPU utilization percentage for auto scaling"
  type        = number
  default     = 70

  validation {
    condition     = var.autoscaling_cpu_target >= 1 && var.autoscaling_cpu_target <= 100
    error_message = "CPU target must be between 1 and 100."
  }
}

variable "autoscaling_memory_target" {
  description = "Target memory utilization percentage for auto scaling (optional, disabled if null)"
  type        = number
  default     = null

  validation {
    condition     = var.autoscaling_memory_target == null || (var.autoscaling_memory_target >= 1 && var.autoscaling_memory_target <= 100)
    error_message = "Memory target must be between 1 and 100 when specified."
  }
}

variable "autoscaling_scale_in_cooldown" {
  description = "Cooldown period in seconds after a scale-in activity completes"
  type        = number
  default     = 300

  validation {
    condition     = var.autoscaling_scale_in_cooldown >= 0 && var.autoscaling_scale_in_cooldown <= 3600
    error_message = "Scale-in cooldown must be between 0 and 3600 seconds."
  }
}

variable "autoscaling_scale_out_cooldown" {
  description = "Cooldown period in seconds after a scale-out activity completes"
  type        = number
  default     = 60

  validation {
    condition     = var.autoscaling_scale_out_cooldown >= 0 && var.autoscaling_scale_out_cooldown <= 3600
    error_message = "Scale-out cooldown must be between 0 and 3600 seconds."
  }
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

    When provided, IAM permissions are granted for the task role to read this secret,
    allowing the application to validate the X-Origin-Verify header.

    The secret should be created separately (e.g., via a dedicated Terraform module
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
    looks up the secret ARN by name and grants IAM permissions for the task role
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
  description = "List of subnet IDs for ElastiCache subnet group (defaults to private_subnet_ids if not specified)"
  type        = list(string)
  default     = []
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

variable "alarm_ecs_cpu_threshold" {
  description = "Threshold for ECS service CPU utilization alarm (percentage)"
  type        = number
  default     = 85

  validation {
    condition     = var.alarm_ecs_cpu_threshold >= 1 && var.alarm_ecs_cpu_threshold <= 100
    error_message = "CPU threshold must be between 1 and 100."
  }
}

variable "alarm_ecs_memory_threshold" {
  description = "Threshold for ECS service memory utilization alarm (percentage)"
  type        = number
  default     = 85

  validation {
    condition     = var.alarm_ecs_memory_threshold >= 1 && var.alarm_ecs_memory_threshold <= 100
    error_message = "Memory threshold must be between 1 and 100."
  }
}

variable "alarm_alb_5xx_threshold" {
  description = "Threshold for ALB 5XX error alarm (number of errors per evaluation period)"
  type        = number
  default     = 10

  validation {
    condition     = var.alarm_alb_5xx_threshold >= 1
    error_message = "5XX threshold must be at least 1."
  }
}

variable "alarm_alb_target_response_time_threshold" {
  description = "Threshold for ALB target response time alarm (seconds)"
  type        = number
  default     = 5

  validation {
    condition     = var.alarm_alb_target_response_time_threshold >= 0.001
    error_message = "Response time threshold must be at least 0.001 seconds."
  }
}
