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
# Application
# ========================================

output "resource_prefix" {
  description = "Resource prefix used for naming"
  value       = local.resource_prefix
}

# ========================================
# ECS cluster
# ========================================

output "ecs_cluster_name" {
  description = "ECS cluster name"
  value       = local.ecs_cluster_name
}

output "ecs_cluster_arn" {
  description = "ECS cluster ARN"
  value       = var.create_ecs_cluster ? aws_ecs_cluster.main[0].arn : data.aws_ecs_cluster.existing[0].arn
}

output "ecs_cluster_id" {
  description = "ECS cluster ID"
  value       = var.create_ecs_cluster ? aws_ecs_cluster.main[0].id : data.aws_ecs_cluster.existing[0].id
}

# ========================================
# ECS service
# ========================================

output "ecs_service_name" {
  description = "ECS service name"
  value       = aws_ecs_service.main.name
}

output "ecs_service_id" {
  description = "ECS service ID"
  value       = aws_ecs_service.main.id
}

output "ecs_task_definition_arn" {
  description = "ECS task definition ARN"
  value       = aws_ecs_task_definition.main.arn
}

output "ecs_task_definition_family" {
  description = "ECS task definition family"
  value       = aws_ecs_task_definition.main.family
}

output "ecs_task_definition_revision" {
  description = "ECS task definition revision"
  value       = aws_ecs_task_definition.main.revision
}

# ========================================
# Networking
# ========================================

output "ecs_tasks_security_group_id" {
  description = "Security group ID for ECS tasks"
  value       = aws_security_group.ecs_tasks.id
}

output "alb_security_group_id" {
  description = "Security group ID for ALB (if enabled)"
  value       = var.alb_enabled ? aws_security_group.alb[0].id : null
}

# ========================================
# Load Balancer
# ========================================

output "alb_arn" {
  description = "ARN of the Application Load Balancer (if enabled)"
  value       = var.alb_enabled ? aws_lb.main[0].arn : null
}

output "alb_dns_name" {
  description = "DNS name of the Application Load Balancer (if enabled)"
  value       = var.alb_enabled ? aws_lb.main[0].dns_name : null
}

output "alb_zone_id" {
  description = "Canonical hosted zone ID of the ALB (for Route53 alias records)"
  value       = var.alb_enabled ? aws_lb.main[0].zone_id : null
}

output "alb_listener_arn" {
  description = "ARN of the ALB listener (if enabled)"
  value       = var.alb_enabled ? aws_lb_listener.main[0].arn : null
}

output "target_group_arn" {
  description = "ARN of the target group (if ALB is enabled)"
  value       = var.alb_enabled ? aws_lb_target_group.main[0].arn : null
}

output "target_group_name" {
  description = "Name of the target group (if ALB is enabled)"
  value       = var.alb_enabled ? aws_lb_target_group.main[0].name : null
}

output "service_url" {
  description = "URL to access the service (if ALB is enabled)"
  value       = var.alb_enabled ? "${var.alb_listener_protocol == "HTTPS" ? "https" : "http"}://${aws_lb.main[0].dns_name}:${var.alb_listener_port}" : null
}

# ========================================
# IAM
# ========================================

output "iam_task_role_arn" {
  description = "ARN of the IAM task role (application permissions)"
  value       = aws_iam_role.task.arn
}

output "iam_task_role_name" {
  description = "Name of the IAM task role"
  value       = aws_iam_role.task.name
}

output "iam_execution_role_arn" {
  description = "ARN of the IAM task execution role (ECS agent permissions)"
  value       = aws_iam_role.execution.arn
}

output "iam_execution_role_name" {
  description = "Name of the IAM task execution role"
  value       = aws_iam_role.execution.name
}

# ========================================
# Secrets Manager
# ========================================

output "github_apps_secret_arn" {
  description = "ARN of the GitHub Apps secret in Secrets Manager"
  value       = aws_secretsmanager_secret.github_apps.arn
}

output "github_apps_secret_name" {
  description = "Name of the GitHub Apps secret in Secrets Manager"
  value       = aws_secretsmanager_secret.github_apps.name
}

# ========================================
# Secrets Manager KMS
# ========================================

output "github_apps_secret_kms_key_arn" {
  description = "ARN of the KMS key used for Secrets Manager encryption (null if using AWS-managed key)"
  value       = local.github_apps_secret_kms_key_arn
}

output "github_apps_secret_kms_key_id" {
  description = "ID of the module-created KMS key (null if not created by module)"
  value       = var.github_apps_secret_create_kms_key ? aws_kms_key.github_apps_secret[0].key_id : null
}

output "github_apps_secret_kms_alias_arn" {
  description = "ARN of the module-created KMS key alias (null if not created by module)"
  value       = var.github_apps_secret_create_kms_key ? aws_kms_alias.github_apps_secret[0].arn : null
}

# ========================================
# DynamoDB
# ========================================

output "audit_table_name" {
  description = "DynamoDB audit table name (use in application configuration)"
  value       = var.audit_dynamodb_enabled ? aws_dynamodb_table.audit[0].name : null
}

output "audit_table_arn" {
  description = "DynamoDB audit table ARN"
  value       = var.audit_dynamodb_enabled ? aws_dynamodb_table.audit[0].arn : null
}

output "selector_table_name" {
  description = "DynamoDB selector table name (use in application configuration)"
  value       = var.selector_dynamodb_enabled ? aws_dynamodb_table.selector[0].name : null
}

output "selector_table_arn" {
  description = "DynamoDB selector table ARN"
  value       = var.selector_dynamodb_enabled ? aws_dynamodb_table.selector[0].arn : null
}

# ========================================
# ElastiCache
# ========================================

output "elasticache_endpoint" {
  description = "ElastiCache Redis primary endpoint address"
  value       = var.selector_elasticache_enabled ? aws_elasticache_replication_group.selector[0].primary_endpoint_address : null
}

output "elasticache_port" {
  description = "ElastiCache Redis port"
  value       = var.selector_elasticache_enabled ? 6379 : null
}

output "elasticache_reader_endpoint" {
  description = "ElastiCache Redis reader endpoint address (for read replicas)"
  value       = var.selector_elasticache_enabled ? aws_elasticache_replication_group.selector[0].reader_endpoint_address : null
}

output "elasticache_auth_token_enabled" {
  description = "Whether ElastiCache Redis authentication is enabled"
  value       = var.selector_elasticache_enabled ? var.elasticache_auth_token_enabled : null
}

output "elasticache_security_group_id" {
  description = "Security group ID for ElastiCache (if enabled)"
  value       = var.selector_elasticache_enabled ? aws_security_group.elasticache[0].id : null
}

# ========================================
# CloudWatch logs
# ========================================

output "cloudwatch_log_group_name" {
  description = "CloudWatch log group name for application logs"
  value       = aws_cloudwatch_log_group.application.name
}

output "cloudwatch_log_group_arn" {
  description = "CloudWatch log group ARN for application logs"
  value       = aws_cloudwatch_log_group.application.arn
}

# ========================================
# CloudWatch alarms
# ========================================

output "alarm_sns_topic_arn" {
  description = "SNS topic ARN for CloudWatch alarms (if configured)"
  value       = var.alarm_sns_topic_arn != "" ? var.alarm_sns_topic_arn : null
}

output "dynamodb_alarm_names" {
  description = "Names of DynamoDB CloudWatch alarms (if created)"
  value = var.audit_dynamodb_enabled && length(local.alarm_actions) > 0 ? {
    read_throttle  = aws_cloudwatch_metric_alarm.dynamodb_read_throttle[0].alarm_name
    write_throttle = aws_cloudwatch_metric_alarm.dynamodb_write_throttle[0].alarm_name
    system_errors  = aws_cloudwatch_metric_alarm.dynamodb_system_errors[0].alarm_name
  } : null
}

output "ecs_alarm_names" {
  description = "Names of ECS CloudWatch alarms (if created)"
  value = length(local.alarm_actions) > 0 ? {
    cpu_utilization    = aws_cloudwatch_metric_alarm.ecs_cpu_utilization[0].alarm_name
    memory_utilization = aws_cloudwatch_metric_alarm.ecs_memory_utilization[0].alarm_name
  } : null
}

output "alb_alarm_names" {
  description = "Names of ALB CloudWatch alarms (if created)"
  value = var.alb_enabled && length(local.alarm_actions) > 0 ? {
    alb_5xx_errors       = aws_cloudwatch_metric_alarm.alb_5xx_errors[0].alarm_name
    target_5xx_errors    = aws_cloudwatch_metric_alarm.alb_target_5xx_errors[0].alarm_name
    target_response_time = aws_cloudwatch_metric_alarm.alb_target_response_time[0].alarm_name
    unhealthy_hosts      = aws_cloudwatch_metric_alarm.alb_unhealthy_hosts[0].alarm_name
  } : null
}

# ========================================
# Auto scaling
# ========================================

output "autoscaling_target_id" {
  description = "Auto scaling target ID (if enabled)"
  value       = var.autoscaling_enabled ? aws_appautoscaling_target.ecs[0].id : null
}

output "autoscaling_policies" {
  description = "Auto scaling policy ARNs (if enabled)"
  value = var.autoscaling_enabled ? {
    cpu    = aws_appautoscaling_policy.ecs_cpu[0].arn
    memory = var.autoscaling_memory_target != null ? aws_appautoscaling_policy.ecs_memory[0].arn : null
  } : null
}
