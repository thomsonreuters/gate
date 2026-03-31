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
# KMS - Customer-managed key for Secrets Manager encryption
# =============================================================================

resource "aws_kms_key" "github_apps_secret" {
  count = var.github_apps_secret_create_kms_key ? 1 : 0

  description         = "CMK for encrypting GitHub Apps Secrets Manager secret (${local.resource_prefix})"
  enable_key_rotation = true

  deletion_window_in_days = 30

  tags = merge(
    local.default_tags,
    {
      Name = "${local.resource_prefix}-github-apps-secret"
    }
  )
}

resource "aws_kms_alias" "github_apps_secret" {
  count = var.github_apps_secret_create_kms_key ? 1 : 0

  name          = local.github_apps_secret_kms_alias_name
  target_key_id = aws_kms_key.github_apps_secret[0].key_id
}

resource "aws_kms_key_policy" "github_apps_secret" {
  count = var.github_apps_secret_create_kms_key && var.github_apps_secret_kms_key_policy_file != null ? 1 : 0

  key_id = aws_kms_key.github_apps_secret[0].id
  policy = file(var.github_apps_secret_kms_key_policy_file)
}
