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
# GATE (GitHub Authenticated Token Exchange)
# Module: AWS ECR
# =============================================================================
#
# This module provisions an Amazon Elastic Container Registry (ECR) repository
# for storing GATE container images.
#
# Core Resources:
# - ECR repository with configurable image scanning and encryption
# - Lifecycle policy for automatic image cleanup (optional)
# - Repository policy for cross-account or organizational access (optional)
#
# This module is intentionally standalone and decoupled from the EKS/ECS
# deployment modules, since container registry management is an independent
# concern from application deployment.
#
# =============================================================================

terraform {
  required_version = ">= 1.6"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}
