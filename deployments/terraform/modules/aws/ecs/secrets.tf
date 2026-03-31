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
# AWS Secrets Manager - GitHub Apps configuration
# =============================================================================

resource "aws_secretsmanager_secret" "github_apps" {
  name        = local.github_apps_secret_name
  description = "GitHub Apps configuration for GATE (GitHub Authenticated Token Exchange)"

  recovery_window_in_days = var.github_apps_secret_recovery_window
  kms_key_id              = local.github_apps_secret_kms_key_arn

  tags = merge(
    local.default_tags,
    {
      Name = local.github_apps_secret_name
    }
  )
}

resource "aws_secretsmanager_secret_policy" "github_apps" {
  count      = var.github_apps_secret_policy_file != null ? 1 : 0
  secret_arn = aws_secretsmanager_secret.github_apps.arn
  policy     = file(var.github_apps_secret_policy_file)
}

resource "aws_secretsmanager_secret_version" "github_apps" {
  secret_id     = aws_secretsmanager_secret.github_apps.id
  secret_string = local.github_apps_secret_value

  # Prevent accidental secret updates during normal Terraform operations.
  # This ensures that changes to other resources don't inadvertently create new secret versions.
  #
  # Initial setup:
  #   Option 1: Let Terraform create with placeholder, then update with script (recommended)
  #     terraform apply
  #     ./deployments/terraform/scripts/aws/push-github-apps-to-secrets-manager.sh \
  #         --file github-apps.json \
  #         --secret-name <secret-name>
  #
  #   Option 2: Provide initial file in terraform.tfvars (for one-time setup)
  #     github_apps_secret_file = "./github-apps.json"
  #     terraform apply
  #
  # Subsequent updates:
  #   Always use the script (Option 1). Terraform will ignore changes due to lifecycle policy below.
  lifecycle {
    ignore_changes = [secret_string]
  }
}
