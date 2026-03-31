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
# Data sources
# =============================================================================

data "aws_caller_identity" "current" {}

data "aws_ecs_cluster" "existing" {
  count = var.create_ecs_cluster ? 0 : 1

  cluster_name = var.ecs_cluster_name
}

data "aws_secretsmanager_secret" "origin_verify" {
  count = var.origin_verify_secret_name != null ? 1 : 0
  name  = var.origin_verify_secret_name
}
