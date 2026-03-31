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
# IAM role - ECS task execution role
# =============================================================================
#
# This role is assumed by the ECS agent to:
# - Pull container images from ECR
# - Write logs to CloudWatch
# - Retrieve secrets from Secrets Manager
#
# =============================================================================

resource "aws_iam_role" "execution" {
  name                 = local.iam_execution_role_name
  path                 = local.iam_role_prefix
  permissions_boundary = local.permissions_boundary

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
        Action = "sts:AssumeRole"
        Condition = {
          ArnLike = {
            "aws:SourceArn" = "arn:aws:ecs:${var.aws_region}:${data.aws_caller_identity.current.account_id}:*"
          }
          StringEquals = {
            "aws:SourceAccount" = data.aws_caller_identity.current.account_id
          }
        }
      }
    ]
  })

  tags = merge(
    local.default_tags,
    {
      Name = local.iam_execution_role_name
    }
  )
}

resource "aws_iam_role_policy_attachment" "execution_managed" {
  role       = aws_iam_role.execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# Additional policy for Secrets Manager access (for container secrets)
resource "aws_iam_role_policy" "execution_secrets" {
  name = "secrets-manager"
  role = aws_iam_role.execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue"
        ]
        Resource = aws_secretsmanager_secret.github_apps.arn
      }
    ]
  })
}

# KMS - Secrets Manager encryption for execution role (conditional, only if CMK is used)
resource "aws_iam_role_policy" "execution_kms_secrets_manager" {
  count = var.github_apps_secret_create_kms_key || var.github_apps_secret_kms_key_arn != null ? 1 : 0

  name = "kms-secrets-manager"
  role = aws_iam_role.execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "kms:Decrypt",
          "kms:DescribeKey"
        ]
        Resource = local.github_apps_secret_kms_key_arn
      }
    ]
  })
}

# =============================================================================
# IAM role - ECS task role
# =============================================================================
#
# This role is assumed by the application running in the container.
# It provides permissions for the application to access AWS services.
#
# =============================================================================

resource "aws_iam_role" "task" {
  name                 = local.iam_task_role_name
  path                 = local.iam_role_prefix
  permissions_boundary = local.permissions_boundary

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
        Action = "sts:AssumeRole"
        Condition = {
          ArnLike = {
            "aws:SourceArn" = "arn:aws:ecs:${var.aws_region}:${data.aws_caller_identity.current.account_id}:*"
          }
          StringEquals = {
            "aws:SourceAccount" = data.aws_caller_identity.current.account_id
          }
        }
      }
    ]
  })

  tags = merge(
    local.default_tags,
    {
      Name = local.iam_task_role_name
    }
  )
}

# Secrets Manager - GitHub Apps credentials
resource "aws_iam_role_policy" "secrets_manager" {
  name = "secrets-manager"
  role = aws_iam_role.task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue",
          "secretsmanager:DescribeSecret"
        ]
        Resource = aws_secretsmanager_secret.github_apps.arn
      }
    ]
  })
}

# Secrets Manager - Origin verification secret (conditional)
resource "aws_iam_role_policy" "origin_verify_secret" {
  count = local.origin_verify_enabled ? 1 : 0

  name = "origin-verify-secret"
  role = aws_iam_role.task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue",
          "secretsmanager:DescribeSecret"
        ]
        Resource = local.origin_verify_secret_arn
      }
    ]
  })
}

# DynamoDB - Audit backend (conditional)
resource "aws_iam_role_policy" "dynamodb_audit" {
  count = var.audit_dynamodb_enabled ? 1 : 0

  name = "dynamodb-audit"
  role = aws_iam_role.task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "dynamodb:PutItem"
        ]
        Resource = aws_dynamodb_table.audit[0].arn
      }
    ]
  })
}

# DynamoDB - Selector store (conditional)
resource "aws_iam_role_policy" "dynamodb_selector" {
  count = var.selector_dynamodb_enabled ? 1 : 0

  name = "dynamodb-selector"
  role = aws_iam_role.task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "dynamodb:PutItem",
          "dynamodb:GetItem",
          "dynamodb:Scan"
        ]
        Resource = aws_dynamodb_table.selector[0].arn
      }
    ]
  })
}

# CloudWatch logs - Application logging
resource "aws_iam_role_policy" "cloudwatch_logs" {
  name = "cloudwatch-logs"
  role = aws_iam_role.task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ]
        Resource = [
          aws_cloudwatch_log_group.application.arn,
          "${aws_cloudwatch_log_group.application.arn}:*"
        ]
      }
    ]
  })
}

# KMS - DynamoDB encryption for audit table (conditional, only if CMK is used)
resource "aws_iam_role_policy" "kms_dynamodb_audit" {
  count = var.audit_dynamodb_enabled && var.audit_table_kms_key_arn != null ? 1 : 0

  name = "kms-dynamodb-audit"
  role = aws_iam_role.task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "kms:Encrypt",
          "kms:Decrypt",
          "kms:GenerateDataKey"
        ]
        Resource = var.audit_table_kms_key_arn
      }
    ]
  })
}

# KMS - Secrets Manager encryption for task role (conditional, only if CMK is used)
resource "aws_iam_role_policy" "kms_secrets_manager" {
  count = var.github_apps_secret_create_kms_key || var.github_apps_secret_kms_key_arn != null ? 1 : 0

  name = "kms-secrets-manager"
  role = aws_iam_role.task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "kms:Decrypt",
          "kms:DescribeKey"
        ]
        Resource = local.github_apps_secret_kms_key_arn
      }
    ]
  })
}

# KMS - DynamoDB encryption for selector table (conditional, only if CMK is used)
resource "aws_iam_role_policy" "kms_dynamodb_selector" {
  count = var.selector_dynamodb_enabled && var.selector_table_kms_key_arn != null ? 1 : 0

  name = "kms-dynamodb-selector"
  role = aws_iam_role.task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "kms:Encrypt",
          "kms:Decrypt",
          "kms:GenerateDataKey"
        ]
        Resource = var.selector_table_kms_key_arn
      }
    ]
  })
}

# ECS Exec - SSM permissions for interactive debugging (conditional)
resource "aws_iam_role_policy" "ecs_exec" {
  count = var.ecs_service_enable_execute_command ? 1 : 0

  name = "ecs-exec"
  role = aws_iam_role.task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ssmmessages:CreateControlChannel",
          "ssmmessages:CreateDataChannel",
          "ssmmessages:OpenControlChannel",
          "ssmmessages:OpenDataChannel"
        ]
        Resource = "*"
      }
    ]
  })
}
