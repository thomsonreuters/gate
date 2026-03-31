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
# ECR Repository
# =============================================================================

resource "aws_ecr_repository" "this" {
  name                 = var.repository_name
  image_tag_mutability = var.image_tag_mutability
  force_delete         = var.force_delete

  image_scanning_configuration {
    scan_on_push = var.scan_on_push
  }

  encryption_configuration {
    encryption_type = var.encryption_type
    kms_key         = var.encryption_type == "KMS" ? var.kms_key_arn : null
  }

  tags = merge(
    local.default_tags,
    {
      Name = var.repository_name
    }
  )
}

# =============================================================================
# Repository policy (optional)
# =============================================================================

resource "aws_ecr_repository_policy" "this" {
  count      = var.repository_policy_file != null ? 1 : 0
  repository = aws_ecr_repository.this.name
  policy     = file(var.repository_policy_file)
}

# =============================================================================
# Lifecycle policy (optional)
# =============================================================================

resource "aws_ecr_lifecycle_policy" "this" {
  count      = var.lifecycle_policy_file != null ? 1 : 0
  repository = aws_ecr_repository.this.name
  policy     = file(var.lifecycle_policy_file)
}
