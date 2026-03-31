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
# Cross-variable validation
# =============================================================================

resource "terraform_data" "kms_validation" {
  count = var.encryption_type == "KMS" ? 1 : 0

  lifecycle {
    precondition {
      condition     = var.kms_key_arn == null || can(regex("^arn:aws:kms:", var.kms_key_arn))
      error_message = "kms_key_arn must be a valid KMS key ARN (starting with arn:aws:kms:) when provided."
    }
  }
}
