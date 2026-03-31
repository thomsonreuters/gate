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
# The application running in Kubernetes will write logs here
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
# CloudWatch alarms
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
