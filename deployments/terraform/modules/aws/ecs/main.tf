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
# Module: AWS ECS
# =============================================================================
#
# This module provisions AWS resources required for deploying GATE on Amazon ECS
# using Fargate.
#
# Core Resources:
# - ECS cluster (optional, can use existing)
# - ECS task definition with Fargate launch type
# - ECS service with rolling deployment
# - IAM task role for application permissions
# - IAM task execution role for ECS agent permissions
# - Secrets Manager for GitHub Apps credentials
# - CloudWatch log group for application logging
# - Application Load Balancer (optional) for traffic distribution
# - Security groups for network isolation
#
# Optional Resources (conditional based on variables):
# - DynamoDB table for audit logs (when audit_dynamodb_enabled=true)
# - DynamoDB table for selector rate limiting (when selector_dynamodb_enabled=true)
# - ElastiCache Redis for selector rate limiting (when selector_elasticache_enabled=true)
# - Auto scaling for the ECS service (when autoscaling_enabled=true)
# - CloudWatch alarms for monitoring
# - KMS encryption policies for DynamoDB tables (when using customer-managed keys)
#
# Prerequisites:
# - VPC with private subnets (and public subnets if using ALB)
# - ACM certificate (if using HTTPS listener on ALB)
# - Container image available in ECR or another registry
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
