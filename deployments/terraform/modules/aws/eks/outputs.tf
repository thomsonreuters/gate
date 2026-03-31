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
# EKS
# ========================================

output "eks_cluster_name" {
  description = "EKS cluster name"
  value       = var.eks_cluster_name
}

output "kubernetes_namespace" {
  description = "Kubernetes namespace for deployment"
  value       = var.kubernetes_namespace
}

output "kubernetes_service_account_name" {
  description = "Kubernetes service account name"
  value       = var.kubernetes_service_account_name
}

output "oidc_provider_arn" {
  description = "EKS OIDC provider ARN"
  value       = local.oidc_provider_arn_formatted
}

output "oidc_provider_url" {
  description = "EKS OIDC provider URL"
  value       = local.oidc_provider_url
}

# ========================================
# IAM
# ========================================

output "iam_role_arn" {
  description = "ARN of the IAM role for IRSA (use in Kubernetes ServiceAccount annotation: eks.amazonaws.com/role-arn)"
  value       = aws_iam_role.service_account.arn
}

output "iam_role_name" {
  description = "Name of the IAM role for IRSA"
  value       = aws_iam_role.service_account.name
}

# ========================================
# Secrets Manager
# ========================================

output "github_apps_secret_arn" {
  description = "ARN of the GitHub Apps secret in Secrets Manager (use in ExternalSecret)"
  value       = aws_secretsmanager_secret.github_apps.arn
}

output "github_apps_secret_name" {
  description = "Name of the GitHub Apps secret in Secrets Manager (use in ExternalSecret)"
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
