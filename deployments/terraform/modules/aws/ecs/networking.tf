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
# Security groups
# =============================================================================

resource "aws_security_group" "ecs_tasks" {
  name_prefix = "${local.resource_prefix}-ecs-tasks-"
  description = "Security group for ${var.service_name} ECS tasks"
  vpc_id      = var.vpc_id

  tags = merge(
    local.default_tags,
    {
      Name = "${local.resource_prefix}-ecs-tasks"
    }
  )

  lifecycle {
    create_before_destroy = true
  }
}

# Allow inbound traffic from ALB to ECS tasks
resource "aws_security_group_rule" "ecs_tasks_ingress_from_alb" {
  count = var.alb_enabled ? 1 : 0

  type              = "ingress"
  from_port         = var.container_port
  to_port           = var.container_port
  protocol          = "tcp"
  security_group_id = aws_security_group.ecs_tasks.id

  source_security_group_id = aws_security_group.alb[0].id

  description = "Allow inbound traffic from ALB"
}

# Allow all outbound traffic from ECS tasks
resource "aws_security_group_rule" "ecs_tasks_egress" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.ecs_tasks.id

  description = "Allow all outbound traffic"
}

resource "aws_security_group" "alb" {
  count = var.alb_enabled ? 1 : 0

  name_prefix = "${local.resource_prefix}-alb-"
  description = "Security group for ${var.service_name} Application Load Balancer"
  vpc_id      = var.vpc_id

  tags = merge(
    local.default_tags,
    {
      Name = "${local.resource_prefix}-alb"
    }
  )

  lifecycle {
    create_before_destroy = true
  }
}

# Allow inbound traffic from CIDR blocks to ALB
resource "aws_security_group_rule" "alb_ingress_cidr" {
  count = var.alb_enabled && length(var.alb_ingress_cidr_blocks) > 0 ? 1 : 0

  type              = "ingress"
  from_port         = var.alb_listener_port
  to_port           = var.alb_listener_port
  protocol          = "tcp"
  cidr_blocks       = var.alb_ingress_cidr_blocks
  security_group_id = aws_security_group.alb[0].id

  description = "Allow inbound traffic from specified CIDR blocks"
}

# Allow inbound traffic from security groups to ALB
resource "aws_security_group_rule" "alb_ingress_sg" {
  count = var.alb_enabled ? length(var.alb_ingress_security_group_ids) : 0

  type                     = "ingress"
  from_port                = var.alb_listener_port
  to_port                  = var.alb_listener_port
  protocol                 = "tcp"
  security_group_id        = aws_security_group.alb[0].id
  source_security_group_id = var.alb_ingress_security_group_ids[count.index]

  description = "Allow inbound traffic from security group ${var.alb_ingress_security_group_ids[count.index]}"
}

# Allow outbound traffic from ALB to ECS tasks
resource "aws_security_group_rule" "alb_egress_to_ecs" {
  count = var.alb_enabled ? 1 : 0

  type              = "egress"
  from_port         = var.container_port
  to_port           = var.container_port
  protocol          = "tcp"
  security_group_id = aws_security_group.alb[0].id

  source_security_group_id = aws_security_group.ecs_tasks.id

  description = "Allow outbound traffic to ECS tasks"
}

# =============================================================================
# Application Load Balancer
# =============================================================================

resource "aws_lb" "main" {
  count = var.alb_enabled ? 1 : 0

  name               = local.alb_name
  internal           = var.alb_internal
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb[0].id]
  subnets            = var.alb_internal ? var.private_subnet_ids : var.public_subnet_ids

  idle_timeout               = var.alb_idle_timeout
  enable_deletion_protection = var.alb_deletion_protection
  drop_invalid_header_fields = var.alb_drop_invalid_header_fields

  dynamic "access_logs" {
    for_each = var.alb_access_logs_enabled ? [1] : []
    content {
      bucket  = var.alb_access_logs_bucket
      prefix  = var.alb_access_logs_prefix
      enabled = true
    }
  }

  tags = merge(
    local.default_tags,
    {
      Name = local.alb_name
    }
  )
}

resource "aws_lb_target_group" "main" {
  count = var.alb_enabled ? 1 : 0

  name        = local.target_group_name
  port        = var.container_port
  protocol    = "HTTP"
  vpc_id      = var.vpc_id
  target_type = "ip"

  deregistration_delay = var.alb_deregistration_delay

  health_check {
    enabled             = true
    healthy_threshold   = var.alb_health_check_healthy_threshold
    interval            = var.alb_health_check_interval
    matcher             = "200-299"
    path                = var.alb_health_check_path
    port                = "traffic-port"
    protocol            = "HTTP"
    timeout             = var.alb_health_check_timeout
    unhealthy_threshold = var.alb_health_check_unhealthy_threshold
  }

  tags = merge(
    local.default_tags,
    {
      Name = local.target_group_name
    }
  )

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_lb_listener" "main" {
  count = var.alb_enabled ? 1 : 0

  load_balancer_arn = aws_lb.main[0].arn
  port              = var.alb_listener_port
  protocol          = var.alb_listener_protocol

  # SSL configuration (only for HTTPS)
  ssl_policy      = var.alb_listener_protocol == "HTTPS" ? var.alb_ssl_policy : null
  certificate_arn = var.alb_listener_protocol == "HTTPS" ? var.alb_certificate_arn : null

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.main[0].arn
  }

  tags = merge(
    local.default_tags,
    {
      Name = "${local.resource_prefix}-listener"
    }
  )
}

resource "aws_lb_listener" "http_redirect" {
  count = var.alb_enabled && var.alb_listener_protocol == "HTTPS" ? 1 : 0

  load_balancer_arn = aws_lb.main[0].arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "redirect"

    redirect {
      port        = tostring(var.alb_listener_port)
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }

  tags = merge(
    local.default_tags,
    {
      Name = "${local.resource_prefix}-http-redirect"
    }
  )
}
