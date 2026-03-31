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
# GATE (GitHub Authenticated Token Exchange)
# Module: AWS EKS
# =============================================================================
#
# This module provisions AWS resources required for deploying GATE on Amazon EKS
# using Helm.
#
# Core Resources:
# - IAM role with IRSA (IAM Roles for Service Accounts) for Kubernetes pods
# - IAM policies with least-privilege access
# - Secrets Manager for GitHub Apps credentials
# - CloudWatch log group for application logging
#
# Optional Resources (conditional based on variables):
# - DynamoDB table for audit logs (when audit_dynamodb_enabled=true)
# - DynamoDB table for selector rate limiting (when selector_dynamodb_enabled=true)
# - ElastiCache Redis for selector rate limiting (when selector_elasticache_enabled=true)
# - CloudWatch alarms for DynamoDB throttling and errors
# - KMS encryption policies for DynamoDB tables (when using customer-managed keys)
#
# The actual application deployment is handled by Helm chart.
#
# Prerequisites:
# - Existing EKS cluster
# - EKS OIDC provider configured on the cluster
# - External Secrets Operator (ESO) installed (for Secrets Manager sync)
#
# =============================================================================

terraform {
  required_version = ">= 1.6"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}
