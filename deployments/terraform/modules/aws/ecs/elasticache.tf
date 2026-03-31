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
# ElastiCache Redis - Selector for rate limit state
# =============================================================================
#
# Prerequisites:
#   - VPC with private subnets
#   - Network connectivity between ECS tasks and ElastiCache subnet
#
# =============================================================================

resource "aws_elasticache_subnet_group" "selector" {
  count = var.selector_elasticache_enabled ? 1 : 0

  name        = local.elasticache_subnet_group_name
  description = "Subnet group for ${var.service_name} ElastiCache Redis cluster"
  subnet_ids  = local.elasticache_subnet_ids

  tags = merge(
    local.default_tags,
    {
      Name = local.elasticache_subnet_group_name
    }
  )
}

resource "aws_security_group" "elasticache" {
  count = var.selector_elasticache_enabled ? 1 : 0

  name_prefix = "${local.resource_prefix}-elasticache-"
  description = "Security group for ${var.service_name} ElastiCache Redis cluster"
  vpc_id      = var.vpc_id

  tags = merge(
    local.default_tags,
    {
      Name = "${local.resource_prefix}-elasticache"
    }
  )

  lifecycle {
    create_before_destroy = true
  }
}

# Allows inbound Redis traffic from ECS tasks
resource "aws_security_group_rule" "elasticache_ingress_from_ecs" {
  count = var.selector_elasticache_enabled ? 1 : 0

  type              = "ingress"
  from_port         = 6379
  to_port           = 6379
  protocol          = "tcp"
  security_group_id = aws_security_group.elasticache[0].id

  source_security_group_id = aws_security_group.ecs_tasks.id

  description = "Allow Redis traffic from ECS tasks"
}

# Allows outbound traffic (for cluster mode, health checks)
resource "aws_security_group_rule" "elasticache_egress" {
  count = var.selector_elasticache_enabled ? 1 : 0

  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.elasticache[0].id

  description = "Allow all outbound traffic"
}

resource "aws_elasticache_replication_group" "selector" {
  count = var.selector_elasticache_enabled ? 1 : 0

  replication_group_id = local.elasticache_replication_group_id
  description          = "Redis cluster for ${var.service_name} selector rate limit state"

  # Engine configuration
  engine               = "redis"
  engine_version       = var.elasticache_engine_version
  port                 = 6379
  parameter_group_name = var.elasticache_parameter_group_name

  # Node configuration
  node_type                  = var.elasticache_node_type
  num_cache_clusters         = var.elasticache_num_cache_nodes
  multi_az_enabled           = var.elasticache_multi_az_enabled
  automatic_failover_enabled = var.elasticache_automatic_failover_enabled

  # Network configuration
  subnet_group_name  = aws_elasticache_subnet_group.selector[0].name
  security_group_ids = [aws_security_group.elasticache[0].id]

  # Security configuration
  at_rest_encryption_enabled = var.elasticache_at_rest_encryption_enabled
  transit_encryption_enabled = var.elasticache_transit_encryption_enabled
  auth_token                 = var.elasticache_auth_token_enabled ? var.elasticache_auth_token : null

  # Maintenance configuration
  maintenance_window         = var.elasticache_maintenance_window
  snapshot_retention_limit   = var.elasticache_snapshot_retention_limit
  snapshot_window            = var.elasticache_snapshot_window
  auto_minor_version_upgrade = var.elasticache_auto_minor_version_upgrade

  # Logging
  log_delivery_configuration {
    destination      = aws_cloudwatch_log_group.elasticache_slow_log[0].name
    destination_type = "cloudwatch-logs"
    log_format       = "json"
    log_type         = "slow-log"
  }

  log_delivery_configuration {
    destination      = aws_cloudwatch_log_group.elasticache_engine_log[0].name
    destination_type = "cloudwatch-logs"
    log_format       = "json"
    log_type         = "engine-log"
  }

  tags = merge(
    local.default_tags,
    {
      Name = local.elasticache_replication_group_id
    }
  )
}

resource "aws_cloudwatch_log_group" "elasticache_slow_log" {
  count = var.selector_elasticache_enabled ? 1 : 0

  name              = "/aws/elasticache/${local.elasticache_replication_group_id}/slow-log"
  retention_in_days = var.cloudwatch_log_retention_days

  tags = merge(
    local.default_tags,
    {
      Name = "${local.elasticache_replication_group_id}-slow-log"
    }
  )
}

resource "aws_cloudwatch_log_group" "elasticache_engine_log" {
  count = var.selector_elasticache_enabled ? 1 : 0

  name              = "/aws/elasticache/${local.elasticache_replication_group_id}/engine-log"
  retention_in_days = var.cloudwatch_log_retention_days

  tags = merge(
    local.default_tags,
    {
      Name = "${local.elasticache_replication_group_id}-engine-log"
    }
  )
}
