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
# DynamoDB table – Audit logs
# =============================================================================

resource "aws_dynamodb_table" "audit" {
  count = var.audit_dynamodb_enabled ? 1 : 0

  name         = local.audit_table_name
  billing_mode = var.audit_table_billing_mode
  hash_key     = "request_id"
  range_key    = "timestamp"

  read_capacity  = var.audit_table_billing_mode == "PROVISIONED" ? var.audit_table_read_capacity : null
  write_capacity = var.audit_table_billing_mode == "PROVISIONED" ? var.audit_table_write_capacity : null

  # Primary key attributes
  attribute {
    name = "request_id"
    type = "S"
  }

  attribute {
    name = "timestamp"
    type = "N"
  }

  attribute {
    name = "caller"
    type = "S"
  }

  attribute {
    name = "target_repository"
    type = "S"
  }

  global_secondary_index {
    name = "CallerIndex"

    key_schema {
      attribute_name = "caller"
      key_type       = "HASH"
    }

    key_schema {
      attribute_name = "timestamp"
      key_type       = "RANGE"
    }

    projection_type = "ALL"

    read_capacity  = var.audit_table_billing_mode == "PROVISIONED" ? var.audit_table_read_capacity : null
    write_capacity = var.audit_table_billing_mode == "PROVISIONED" ? var.audit_table_write_capacity : null
  }

  global_secondary_index {
    name = "RepositoryIndex"

    key_schema {
      attribute_name = "target_repository"
      key_type       = "HASH"
    }

    key_schema {
      attribute_name = "timestamp"
      key_type       = "RANGE"
    }

    projection_type = "ALL"

    read_capacity  = var.audit_table_billing_mode == "PROVISIONED" ? var.audit_table_read_capacity : null
    write_capacity = var.audit_table_billing_mode == "PROVISIONED" ? var.audit_table_write_capacity : null
  }

  # TTL configuration for automatic deletion of old audit logs
  ttl {
    enabled        = var.audit_table_ttl_enabled
    attribute_name = "expires_at"
  }

  # Point-in-time recovery for data protection
  point_in_time_recovery {
    enabled = var.audit_table_point_in_time_recovery
  }

  # Server-side encryption
  # Uses customer-managed KMS key if provided, otherwise AWS-managed keys
  server_side_encryption {
    enabled     = true
    kms_key_arn = var.audit_table_kms_key_arn
  }

  tags = merge(
    local.default_tags,
    {
      Name = local.audit_table_name
    }
  )
}

# =============================================================================
# DynamoDB table - Rate limit state (selector)
# =============================================================================

resource "aws_dynamodb_table" "selector" {
  count = var.selector_dynamodb_enabled ? 1 : 0

  name         = local.selector_table_name
  billing_mode = var.selector_table_billing_mode
  hash_key     = "client_id"

  read_capacity  = var.selector_table_billing_mode == "PROVISIONED" ? var.selector_table_read_capacity : null
  write_capacity = var.selector_table_billing_mode == "PROVISIONED" ? var.selector_table_write_capacity : null

  # Primary key attribute
  attribute {
    name = "client_id"
    type = "S"
  }

  # TTL configuration for automatic deletion of stale rate limit state
  ttl {
    enabled        = var.selector_table_ttl_enabled
    attribute_name = "expires_at"
  }

  # Point-in-time recovery for data protection
  point_in_time_recovery {
    enabled = var.selector_table_point_in_time_recovery
  }

  # Server-side encryption
  # Uses customer-managed KMS key if provided, otherwise AWS-managed keys
  server_side_encryption {
    enabled     = true
    kms_key_arn = var.selector_table_kms_key_arn
  }

  tags = merge(
    local.default_tags,
    {
      Name = local.selector_table_name
    }
  )
}
