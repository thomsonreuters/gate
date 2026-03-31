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
# ECS Cluster
# =============================================================================
#
# Creates an ECS cluster with Fargate capacity providers if create_ecs_cluster
# is true. Otherwise, uses an existing cluster referenced by ecs_cluster_name.
#
# =============================================================================

resource "aws_ecs_cluster" "main" {
  count = var.create_ecs_cluster ? 1 : 0

  name = local.ecs_cluster_name

  setting {
    name  = "containerInsights"
    value = var.ecs_cluster_container_insights ? "enabled" : "disabled"
  }

  tags = merge(
    local.default_tags,
    {
      Name = local.ecs_cluster_name
    }
  )
}

resource "aws_ecs_cluster_capacity_providers" "main" {
  count = var.create_ecs_cluster ? 1 : 0

  cluster_name = aws_ecs_cluster.main[0].name

  capacity_providers = var.ecs_cluster_capacity_providers

  default_capacity_provider_strategy {
    capacity_provider = var.ecs_cluster_default_capacity_provider.capacity_provider
    weight            = var.ecs_cluster_default_capacity_provider.weight
    base              = var.ecs_cluster_default_capacity_provider.base
  }
}

# =============================================================================
# ECS task definition
# =============================================================================

resource "aws_ecs_task_definition" "main" {
  family                   = local.ecs_service_name
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.ecs_task_cpu
  memory                   = var.ecs_task_memory
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  runtime_platform {
    operating_system_family = var.ecs_task_operating_system_family
    cpu_architecture        = var.ecs_task_architecture
  }

  container_definitions = jsonencode([
    merge(
      {
        name      = var.service_name
        image     = var.container_image
        essential = true

        cpu               = var.container_cpu
        memoryReservation = var.container_memory_reservation

        portMappings = [
          {
            containerPort = var.container_port
            hostPort      = var.container_port
            protocol      = "tcp"
          }
        ]

        environment = concat(
          var.container_environment,
          [
            {
              # Tell the application where to find GitHub Apps credentials.
              # The actual JSON is in GITHUB_APPS_CONFIG (injected as a secret below).
              name  = "GATE_GITHUB_APPS"
              value = "env://GITHUB_APPS_CONFIG"
            }
          ]
        )

        secrets = concat(
          var.container_secrets,
          [
            {
              # GitHub Apps credentials JSON from Secrets Manager.
              # Referenced by GATE_GITHUB_APPS environment variable above.
              name      = "GITHUB_APPS_CONFIG"
              valueFrom = aws_secretsmanager_secret.github_apps.arn
            }
          ]
        )

        logConfiguration = {
          logDriver = "awslogs"
          options = {
            "awslogs-group"         = aws_cloudwatch_log_group.application.name
            "awslogs-region"        = var.aws_region
            "awslogs-stream-prefix" = var.service_name
          }
        }

        readonlyRootFilesystem = var.container_readonly_root_filesystem
        stopTimeout            = var.container_stop_timeout

        # Linux parameters for security hardening
        linuxParameters = {
          initProcessEnabled = true
          capabilities = {
            drop = ["ALL"]
          }
        }

        # Run as non-root user
        user = "1000:1000"
      },
      # Container health check (optional, disabled by default for distroless compatibility)
      var.container_health_check != null ? {
        healthCheck = {
          command     = var.container_health_check.command
          interval    = var.container_health_check.interval
          timeout     = var.container_health_check.timeout
          retries     = var.container_health_check.retries
          startPeriod = var.container_health_check.startPeriod
        }
      } : {}
    )
  ])

  tags = merge(
    local.default_tags,
    {
      Name = local.ecs_service_name
    }
  )
}

# =============================================================================
# ECS service
# =============================================================================

resource "aws_ecs_service" "main" {
  name            = local.ecs_service_name
  cluster         = var.create_ecs_cluster ? aws_ecs_cluster.main[0].id : data.aws_ecs_cluster.existing[0].id
  task_definition = aws_ecs_task_definition.main.arn
  desired_count   = var.ecs_service_desired_count
  launch_type     = "FARGATE"

  deployment_minimum_healthy_percent = var.ecs_service_deployment_minimum_healthy_percent
  deployment_maximum_percent         = var.ecs_service_deployment_maximum_percent
  force_new_deployment               = var.ecs_service_force_new_deployment
  wait_for_steady_state              = var.ecs_service_wait_for_steady_state

  enable_execute_command = var.ecs_service_enable_execute_command

  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [aws_security_group.ecs_tasks.id]
    assign_public_ip = var.assign_public_ip
  }

  dynamic "load_balancer" {
    for_each = var.alb_enabled ? [1] : []
    content {
      target_group_arn = aws_lb_target_group.main[0].arn
      container_name   = var.service_name
      container_port   = var.container_port
    }
  }

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  depends_on = [
    aws_lb_listener.main
  ]

  tags = merge(
    local.default_tags,
    {
      Name = local.ecs_service_name
    }
  )
}

# =============================================================================
# Auto scaling
# =============================================================================

resource "aws_appautoscaling_target" "ecs" {
  count = var.autoscaling_enabled ? 1 : 0

  max_capacity       = var.autoscaling_max_capacity
  min_capacity       = var.autoscaling_min_capacity
  resource_id        = "service/${var.create_ecs_cluster ? aws_ecs_cluster.main[0].name : data.aws_ecs_cluster.existing[0].cluster_name}/${aws_ecs_service.main.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"

  tags = local.default_tags
}

resource "aws_appautoscaling_policy" "ecs_cpu" {
  count = var.autoscaling_enabled ? 1 : 0

  name               = "${local.ecs_service_name}-cpu-scaling"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.ecs[0].resource_id
  scalable_dimension = aws_appautoscaling_target.ecs[0].scalable_dimension
  service_namespace  = aws_appautoscaling_target.ecs[0].service_namespace

  target_tracking_scaling_policy_configuration {
    target_value       = var.autoscaling_cpu_target
    scale_in_cooldown  = var.autoscaling_scale_in_cooldown
    scale_out_cooldown = var.autoscaling_scale_out_cooldown

    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
  }
}

resource "aws_appautoscaling_policy" "ecs_memory" {
  count = var.autoscaling_enabled && var.autoscaling_memory_target != null ? 1 : 0

  name               = "${local.ecs_service_name}-memory-scaling"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.ecs[0].resource_id
  scalable_dimension = aws_appautoscaling_target.ecs[0].scalable_dimension
  service_namespace  = aws_appautoscaling_target.ecs[0].service_namespace

  target_tracking_scaling_policy_configuration {
    target_value       = var.autoscaling_memory_target
    scale_in_cooldown  = var.autoscaling_scale_in_cooldown
    scale_out_cooldown = var.autoscaling_scale_out_cooldown

    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageMemoryUtilization"
    }
  }
}
