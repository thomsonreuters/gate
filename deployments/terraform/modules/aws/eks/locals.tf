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

locals {
  # ========================================
  # Resource naming
  # ========================================

  resource_prefix = var.resource_prefix != "" ? var.resource_prefix : var.service_name

  iam_role_name           = var.iam_role_name != null ? var.iam_role_name : "${local.resource_prefix}-eks-irsa"
  github_apps_secret_name = var.github_apps_secret_name != null ? var.github_apps_secret_name : "${local.resource_prefix}-github-apps"

  github_apps_secret_value = var.github_apps_secret_file != null ? file(var.github_apps_secret_file) : jsonencode({
    apps        = []
    placeholder = true
    note        = "This is a placeholder. Update using: ./deployments/terraform/scripts/aws/push-github-apps-to-secrets-manager.sh"
  })

  github_apps_secret_kms_key_arn    = var.github_apps_secret_create_kms_key ? aws_kms_key.github_apps_secret[0].arn : var.github_apps_secret_kms_key_arn
  github_apps_secret_kms_alias_name = "alias/${local.resource_prefix}-github-apps-secret"

  audit_table_name                 = var.audit_table_name != null ? var.audit_table_name : "${local.resource_prefix}-audit-logs"
  selector_table_name              = var.selector_table_name != null ? var.selector_table_name : "${local.resource_prefix}-rate-limits"
  elasticache_subnet_group_name    = "${local.resource_prefix}-elasticache"
  elasticache_replication_group_id = "${local.resource_prefix}-redis"
  cloudwatch_log_group_name        = var.cloudwatch_log_group_name != null ? var.cloudwatch_log_group_name : "/aws/eks/${var.eks_cluster_name}/${var.kubernetes_namespace}/${var.service_name}"

  # ========================================
  # IAM
  # ========================================

  permissions_boundary = var.permissions_boundary_arn != "" ? var.permissions_boundary_arn : null
  iam_role_prefix      = var.iam_role_prefix

  # ========================================
  # EKS OIDC provider
  # ========================================

  # Get OIDC issuer URL from cluster data
  cluster_oidc_issuer_url = replace(data.aws_eks_cluster.cluster.identity[0].oidc[0].issuer, "https://", "")

  # Use provided OIDC provider URL or derive from cluster
  oidc_provider_url = var.eks_oidc_provider_url != null ? var.eks_oidc_provider_url : local.cluster_oidc_issuer_url

  # Full OIDC provider ARN - use provided ARN or construct from URL
  oidc_provider_arn_formatted = var.eks_oidc_provider_arn != null ? var.eks_oidc_provider_arn : "arn:aws:iam::${var.aws_account_number}:oidc-provider/${local.oidc_provider_url}"

  # ========================================
  # Kubernetes Service Account
  # ========================================

  service_account_arn_pattern = "system:serviceaccount:${var.kubernetes_namespace}:${var.kubernetes_service_account_name}"

  # ========================================
  # Origin verification
  # ========================================

  origin_verify_enabled    = var.origin_verify_secret_arn != null || var.origin_verify_secret_name != null
  origin_verify_secret_arn = coalesce(var.origin_verify_secret_arn, try(data.aws_secretsmanager_secret.origin_verify[0].arn, null))

  # ========================================
  # CloudWatch alarms
  # ========================================

  alarm_actions = var.alarm_sns_topic_arn != "" ? [var.alarm_sns_topic_arn] : []

  # ========================================
  # Tags
  # ========================================

  default_tags = merge(
    {
      "managed-by"           = "terraform"
      "deployment-type"      = "eks"
      "eks-cluster"          = var.eks_cluster_name
      "kubernetes-namespace" = var.kubernetes_namespace
    },
    var.tags
  )
}
