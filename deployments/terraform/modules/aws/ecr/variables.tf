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

# ========================================
# Tagging
# ========================================

variable "tags" {
  description = "Additional tags to apply to all resources. Merged with module defaults (managed-by)."
  type        = map(string)
  default     = {}

  validation {
    condition     = length(var.tags) <= 49
    error_message = "Too many tags: ${length(var.tags)} user tags + 1 module default tag would exceed AWS limit of 50. Maximum allowed: 49 user tags."
  }

  validation {
    condition = alltrue([
      for key in keys(var.tags) : length(key) > 0 && length(key) <= 128
    ])
    error_message = "All tag keys must be non-empty and not exceed 128 characters."
  }

  validation {
    condition = alltrue([
      for value in values(var.tags) : length(value) <= 256
    ])
    error_message = "All tag values must not exceed 256 characters."
  }
}

# ========================================
# Repository
# ========================================

variable "repository_name" {
  description = "Name of the ECR repository"
  type        = string
  default     = "gate"

  validation {
    condition     = can(regex("^[a-z][a-z0-9._/-]{1,255}$", var.repository_name))
    error_message = "Repository name must start with a lowercase letter and contain only lowercase letters, digits, dots, hyphens, underscores, and forward slashes (max 256 characters)."
  }
}

variable "image_tag_mutability" {
  description = "Tag mutability setting for the repository. IMMUTABLE prevents image tags from being overwritten (recommended for production)."
  type        = string
  default     = "IMMUTABLE"

  validation {
    condition     = contains(["MUTABLE", "IMMUTABLE"], var.image_tag_mutability)
    error_message = "image_tag_mutability must be MUTABLE or IMMUTABLE."
  }
}

variable "force_delete" {
  description = "Whether to delete the repository even if it contains images. Use with caution."
  type        = bool
  default     = false
}

# ========================================
# Image scanning
# ========================================

variable "scan_on_push" {
  description = "Whether images are scanned for vulnerabilities after being pushed to the repository"
  type        = bool
  default     = true
}

# ========================================
# Encryption
# ========================================

variable "encryption_type" {
  description = "Encryption type for the repository. AES256 uses AWS-managed key, KMS uses a customer-managed or AWS-managed KMS key."
  type        = string
  default     = "AES256"

  validation {
    condition     = contains(["AES256", "KMS"], var.encryption_type)
    error_message = "encryption_type must be AES256 or KMS."
  }
}

variable "kms_key_arn" {
  description = "ARN of the KMS key for repository encryption (required when encryption_type is KMS, omit to use the AWS-managed aws/ecr key)"
  type        = string
  default     = null
}

# ========================================
# Lifecycle policy
# ========================================

variable "lifecycle_policy_file" {
  description = <<-EOT
    Path to a local JSON file containing an ECR lifecycle policy.

    Optional: If not provided, no lifecycle policy is attached.

    See: https://docs.aws.amazon.com/AmazonECR/latest/userguide/lifecycle_policy_examples.html
  EOT
  type        = string
  default     = null

  validation {
    condition     = var.lifecycle_policy_file == null || can(file(var.lifecycle_policy_file))
    error_message = "The file specified in lifecycle_policy_file does not exist or cannot be read. Please provide a valid file path."
  }

  validation {
    condition     = var.lifecycle_policy_file == null || can(jsondecode(file(var.lifecycle_policy_file)))
    error_message = "The file specified in lifecycle_policy_file does not contain valid JSON. Please ensure the file contains a properly formatted ECR lifecycle policy."
  }
}

# ========================================
# Repository policy
# ========================================

variable "repository_policy_file" {
  description = <<-EOT
    Path to a local JSON file containing a repository policy for cross-account or organizational access.

    Optional: If not provided, no repository policy is attached.
  EOT
  type        = string
  default     = null

  validation {
    condition     = var.repository_policy_file == null || can(file(var.repository_policy_file))
    error_message = "The file specified in repository_policy_file does not exist or cannot be read. Please provide a valid file path."
  }

  validation {
    condition     = var.repository_policy_file == null || can(jsondecode(file(var.repository_policy_file)))
    error_message = "The file specified in repository_policy_file does not contain valid JSON. Please ensure the file contains a properly formatted IAM policy document."
  }
}
