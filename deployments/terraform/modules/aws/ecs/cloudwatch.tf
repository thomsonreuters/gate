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
# CloudWatch logs
# =============================================================================

# CloudWatch log group for application logs
# ECS tasks will write logs here using the awslogs driver
resource "aws_cloudwatch_log_group" "application" {
  name              = local.cloudwatch_log_group_name
  retention_in_days = var.cloudwatch_log_retention_days

  tags = merge(
    local.default_tags,
    {
      Name = local.cloudwatch_log_group_name
    }
  )
}

# =============================================================================
# CloudWatch alarms - DynamoDB
# =============================================================================

resource "aws_cloudwatch_metric_alarm" "dynamodb_read_throttle" {
  count = var.audit_dynamodb_enabled && length(local.alarm_actions) > 0 ? 1 : 0

  alarm_name          = "${local.audit_table_name}-read-throttle"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "ReadThrottledRequests"
  namespace           = "AWS/DynamoDB"
  period              = 300 # 5 minutes
  statistic           = "Sum"
  threshold           = var.alarm_dynamodb_read_throttle_threshold
  alarm_description   = "DynamoDB table ${local.audit_table_name} read requests are being throttled"
  treat_missing_data  = "notBreaching"

  dimensions = {
    TableName = aws_dynamodb_table.audit[0].name
  }

  alarm_actions = local.alarm_actions

  tags = merge(
    local.default_tags,
    {
      Name = "${local.audit_table_name}-read-throttle-alarm"
    }
  )
}

resource "aws_cloudwatch_metric_alarm" "dynamodb_write_throttle" {
  count = var.audit_dynamodb_enabled && length(local.alarm_actions) > 0 ? 1 : 0

  alarm_name          = "${local.audit_table_name}-write-throttle"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "WriteThrottledRequests"
  namespace           = "AWS/DynamoDB"
  period              = 300 # 5 minutes
  statistic           = "Sum"
  threshold           = var.alarm_dynamodb_write_throttle_threshold
  alarm_description   = "DynamoDB table ${local.audit_table_name} write requests are being throttled"
  treat_missing_data  = "notBreaching"

  dimensions = {
    TableName = aws_dynamodb_table.audit[0].name
  }

  alarm_actions = local.alarm_actions

  tags = merge(
    local.default_tags,
    {
      Name = "${local.audit_table_name}-write-throttle-alarm"
    }
  )
}

resource "aws_cloudwatch_metric_alarm" "dynamodb_system_errors" {
  count = var.audit_dynamodb_enabled && length(local.alarm_actions) > 0 ? 1 : 0

  alarm_name          = "${local.audit_table_name}-system-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "SystemErrors"
  namespace           = "AWS/DynamoDB"
  period              = 300 # 5 minutes
  statistic           = "Sum"
  threshold           = var.alarm_dynamodb_system_errors_threshold
  alarm_description   = "DynamoDB table ${local.audit_table_name} is experiencing system errors"
  treat_missing_data  = "notBreaching"

  dimensions = {
    TableName = aws_dynamodb_table.audit[0].name
  }

  alarm_actions = local.alarm_actions

  tags = merge(
    local.default_tags,
    {
      Name = "${local.audit_table_name}-system-errors-alarm"
    }
  )
}

# =============================================================================
# CloudWatch alarms - ECS Service
# =============================================================================

resource "aws_cloudwatch_metric_alarm" "ecs_cpu_utilization" {
  count = length(local.alarm_actions) > 0 ? 1 : 0

  alarm_name          = "${local.ecs_service_name}-cpu-utilization"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  metric_name         = "CPUUtilization"
  namespace           = "AWS/ECS"
  period              = 300 # 5 minutes
  statistic           = "Average"
  threshold           = var.alarm_ecs_cpu_threshold
  alarm_description   = "ECS service ${local.ecs_service_name}: CPU utilization is above ${var.alarm_ecs_cpu_threshold}%"
  treat_missing_data  = "notBreaching"

  dimensions = {
    ClusterName = local.ecs_cluster_name
    ServiceName = local.ecs_service_name
  }

  alarm_actions = local.alarm_actions

  tags = merge(
    local.default_tags,
    {
      Name = "${local.ecs_service_name}-cpu-utilization-alarm"
    }
  )

  depends_on = [aws_ecs_service.main]
}

resource "aws_cloudwatch_metric_alarm" "ecs_memory_utilization" {
  count = length(local.alarm_actions) > 0 ? 1 : 0

  alarm_name          = "${local.ecs_service_name}-memory-utilization"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  metric_name         = "MemoryUtilization"
  namespace           = "AWS/ECS"
  period              = 300 # 5 minutes
  statistic           = "Average"
  threshold           = var.alarm_ecs_memory_threshold
  alarm_description   = "ECS service ${local.ecs_service_name}: memory utilization is above ${var.alarm_ecs_memory_threshold}%"
  treat_missing_data  = "notBreaching"

  dimensions = {
    ClusterName = local.ecs_cluster_name
    ServiceName = local.ecs_service_name
  }

  alarm_actions = local.alarm_actions

  tags = merge(
    local.default_tags,
    {
      Name = "${local.ecs_service_name}-memory-utilization-alarm"
    }
  )

  depends_on = [aws_ecs_service.main]
}

# =============================================================================
# CloudWatch alarms - Application Load Balancer
# =============================================================================

resource "aws_cloudwatch_metric_alarm" "alb_5xx_errors" {
  count = var.alb_enabled && length(local.alarm_actions) > 0 ? 1 : 0

  alarm_name          = "${local.alb_name}-5xx-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "HTTPCode_ELB_5XX_Count"
  namespace           = "AWS/ApplicationELB"
  period              = 300 # 5 minutes
  statistic           = "Sum"
  threshold           = var.alarm_alb_5xx_threshold
  alarm_description   = "ALB ${local.alb_name} is returning 5XX errors"
  treat_missing_data  = "notBreaching"

  dimensions = {
    LoadBalancer = aws_lb.main[0].arn_suffix
  }

  alarm_actions = local.alarm_actions

  tags = merge(
    local.default_tags,
    {
      Name = "${local.alb_name}-5xx-errors-alarm"
    }
  )
}

resource "aws_cloudwatch_metric_alarm" "alb_target_5xx_errors" {
  count = var.alb_enabled && length(local.alarm_actions) > 0 ? 1 : 0

  alarm_name          = "${local.alb_name}-target-5xx-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "HTTPCode_Target_5XX_Count"
  namespace           = "AWS/ApplicationELB"
  period              = 300 # 5 minutes
  statistic           = "Sum"
  threshold           = var.alarm_alb_5xx_threshold
  alarm_description   = "ALB ${local.alb_name} targets are returning 5XX errors"
  treat_missing_data  = "notBreaching"

  dimensions = {
    LoadBalancer = aws_lb.main[0].arn_suffix
    TargetGroup  = aws_lb_target_group.main[0].arn_suffix
  }

  alarm_actions = local.alarm_actions

  tags = merge(
    local.default_tags,
    {
      Name = "${local.alb_name}-target-5xx-errors-alarm"
    }
  )
}

resource "aws_cloudwatch_metric_alarm" "alb_target_response_time" {
  count = var.alb_enabled && length(local.alarm_actions) > 0 ? 1 : 0

  alarm_name          = "${local.alb_name}-target-response-time"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  metric_name         = "TargetResponseTime"
  namespace           = "AWS/ApplicationELB"
  period              = 300 # 5 minutes
  statistic           = "Average"
  threshold           = var.alarm_alb_target_response_time_threshold
  alarm_description   = "ALB ${local.alb_name} target response time is above ${var.alarm_alb_target_response_time_threshold}s"
  treat_missing_data  = "notBreaching"

  dimensions = {
    LoadBalancer = aws_lb.main[0].arn_suffix
    TargetGroup  = aws_lb_target_group.main[0].arn_suffix
  }

  alarm_actions = local.alarm_actions

  tags = merge(
    local.default_tags,
    {
      Name = "${local.alb_name}-target-response-time-alarm"
    }
  )
}

resource "aws_cloudwatch_metric_alarm" "alb_unhealthy_hosts" {
  count = var.alb_enabled && length(local.alarm_actions) > 0 ? 1 : 0

  alarm_name          = "${local.alb_name}-unhealthy-hosts"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "UnHealthyHostCount"
  namespace           = "AWS/ApplicationELB"
  period              = 60 # 1 minute
  statistic           = "Maximum"
  threshold           = 0
  alarm_description   = "ALB ${local.alb_name} has unhealthy hosts in target group"
  treat_missing_data  = "notBreaching"

  dimensions = {
    LoadBalancer = aws_lb.main[0].arn_suffix
    TargetGroup  = aws_lb_target_group.main[0].arn_suffix
  }

  alarm_actions = local.alarm_actions

  tags = merge(
    local.default_tags,
    {
      Name = "${local.alb_name}-unhealthy-hosts-alarm"
    }
  )
}
