#!/usr/bin/env bash
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


set -euo pipefail

# ============================================================================
# GitHub Apps: AWS Secrets Manager update script
# ============================================================================
# This script updates GitHub Apps configuration in AWS Secrets Manager.
# Use this after Terraform has created the secret to update its value.
#
# Compatible with Bash 3.2+ (macOS) and Bash 4+ (Linux)
# ============================================================================

readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly CYAN='\033[0;36m'
readonly NC='\033[0m' # No Color

function log_info() {
    printf "${BLUE}%-9s${NC} %s - %s\n" "[INFO]" "$(date '+%Y-%m-%d %H:%M:%S')" "$1"
}

function log_success() {
    printf "${GREEN}%-9s${NC} %s - %s\n" "[SUCCESS]" "$(date '+%Y-%m-%d %H:%M:%S')" "$1"
}

function log_warning() {
    printf "${YELLOW}%-9s${NC} %s - %s\n" "[WARNING]" "$(date '+%Y-%m-%d %H:%M:%S')" "$1"
}

function log_error() {
    printf "${RED}%-9s${NC} %s - %s\n" "[ERROR]" "$(date '+%Y-%m-%d %H:%M:%S')" "$1" >&2
}

function log_detail() {
    printf "                                 %s\n" "$1"
}

function show_help() {
    cat << EOF
Usage: $0 [OPTIONS]

GitHub Apps: AWS Secrets Manager update script
==============================================

This script updates the GitHub Apps configuration in AWS Secrets Manager.
Use this script to update secrets created by Terraform modules without
triggering Terraform state changes (when lifecycle.ignore_changes is configured).

OPTIONS:
    -f, --file FILE              Path to GitHub Apps JSON file (required)
    -s, --secret-name NAME       AWS Secrets Manager secret name (required)
    -r, --region REGION          AWS region (default: from AWS_REGION or us-east-1)
    --profile PROFILE            AWS CLI profile to use
    --dry-run                    Show what would be done without making changes
    -v, --verbose                Enable verbose output
    -h, --help                   Display this help message

ENVIRONMENT VARIABLES:
    AWS_REGION                   Default AWS region
    AWS_PROFILE                  Default AWS CLI profile
    AWS_ACCESS_KEY_ID            AWS credentials (if not using profile)
    AWS_SECRET_ACCESS_KEY        AWS credentials (if not using profile)

EXAMPLES:
    # Update secret in default region
    $0 --file github-apps.json \\
       --secret-name gate-github-apps

    # Update secret in specific region
    $0 --file github-apps.json \\
       --secret-name gate-github-apps \\
       --region us-east-1

    # Update using AWS profile
    $0 --file github-apps.json \\
       --secret-name gate-github-apps \\
       --profile production

    # Dry run to preview changes
    $0 --file github-apps.json \\
       --secret-name gate-github-apps \\
       --dry-run

REQUIREMENTS:
    - aws-cli (configured with appropriate credentials)
    - jq (for JSON validation)
    - IAM permissions: secretsmanager:PutSecretValue on the target secret
    - Bash 3.2+ (compatible with macOS and Linux)

SECURITY NOTES:
    - This script bypasses Terraform lifecycle.ignore_changes protection
    - Use carefully in production environments
    - Always validate JSON before uploading
    - Consider using --dry-run first to preview changes
    - Ensure AWS credentials have minimal required permissions

TERRAFORM INTEGRATION:
    When Terraform modules use lifecycle.ignore_changes for secrets,
    you should use this script to update secret values without forcing
    Terraform to recreate the secret version.

    Alternative: terraform apply -replace with new secret value
    (See Terraform module README for details)

GITHUB APPS JSON FORMAT:
    {
      "apps": [
        {
          "client_id": "Iv1...",
          "private_key_path": "/path/to/key.pem",
          "organization": "my-org"
        }
      ]
    }

EOF
}

JSON_FILE=""
SECRET_NAME=""
AWS_REGION="${AWS_REGION:-us-east-1}"
AWS_PROFILE=""
DRY_RUN=false
VERBOSE=false

function require_arg() {
    if [[ $# -lt 3 ]] || [[ "$3" == -* ]]; then
        log_error "Option $1 requires a value"
        exit 1
    fi
}

while [[ $# -gt 0 ]]; do
    case $1 in
        -f|--file)
            require_arg "$1" "$#" "${2:-}"
            JSON_FILE="$2"
            shift 2
            ;;
        -s|--secret-name)
            require_arg "$1" "$#" "${2:-}"
            SECRET_NAME="$2"
            shift 2
            ;;
        -r|--region)
            require_arg "$1" "$#" "${2:-}"
            AWS_REGION="$2"
            shift 2
            ;;
        --profile)
            require_arg "$1" "$#" "${2:-}"
            AWS_PROFILE="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            echo ""
            show_help
            exit 1
            ;;
    esac
done

# ============================================================================
# Validation
# ============================================================================

log_info "Starting AWS Secrets Manager update process"

for cmd in aws jq; do
    if ! command -v "$cmd" &> /dev/null; then
        log_error "Required command '$cmd' not found. Please install it and try again."
        exit 1
    fi
done

if [[ -z "$JSON_FILE" ]]; then
    log_error "GitHub Apps JSON file is required. Use --file option."
    exit 1
fi

if [[ ! -f "$JSON_FILE" ]]; then
    log_error "File not found: $JSON_FILE"
    exit 1
fi

if ! jq empty "$JSON_FILE" 2>/dev/null; then
    log_error "Invalid JSON in $JSON_FILE"
    exit 1
fi

if [[ -z "$SECRET_NAME" ]]; then
    log_error "Secret name is required. Use --secret-name option."
    exit 1
fi

log_info "Secret name: $SECRET_NAME"
log_info "AWS region: $AWS_REGION"
log_info "Source file: $JSON_FILE"

if [[ -n "$AWS_PROFILE" ]]; then
    log_info "AWS profile: $AWS_PROFILE"
fi

if [[ "$DRY_RUN" == "true" ]]; then
    log_warning "DRY RUN MODE: No changes will be made"
fi

# ============================================================================
# Verify AWS access
# ============================================================================

log_info "Verifying AWS credentials..."

AWS_BASE=(aws)
if [[ -n "$AWS_PROFILE" ]]; then
    AWS_BASE+=(--profile "$AWS_PROFILE")
fi
AWS_BASE+=(--region "$AWS_REGION")

if ! CALLER_IDENTITY=$("${AWS_BASE[@]}" sts get-caller-identity 2>&1); then
    log_error "Failed to verify AWS credentials"
    log_error "AWS CLI output: $CALLER_IDENTITY"
    log_error "Please check your AWS credentials and try again"
    exit 1
fi

AWS_ACCOUNT=$(echo "$CALLER_IDENTITY" | jq -r '.Account')
AWS_USER_ARN=$(echo "$CALLER_IDENTITY" | jq -r '.Arn')

log_success "AWS credentials verified"
log_detail "Account: $AWS_ACCOUNT"
log_detail "Identity: $AWS_USER_ARN"

log_info "Verifying secret exists in Secrets Manager..."

if ! SECRET_INFO=$("${AWS_BASE[@]}" secretsmanager describe-secret --secret-id "$SECRET_NAME" 2>&1); then
    log_error "Secret not found: $SECRET_NAME"
    log_error "AWS CLI output: $SECRET_INFO"
    log_error "Please check the secret name and ensure it exists"
    exit 1
fi

SECRET_ARN=$(echo "$SECRET_INFO" | jq -r '.ARN')
log_success "Secret found: $SECRET_NAME"
log_detail "ARN: $SECRET_ARN"

# ============================================================================
# Update secret in AWS Secrets Manager
# ============================================================================

log_info "Preparing to update secret in AWS Secrets Manager..."

JSON_CONTENT=$(jq -c '.' "$JSON_FILE")

if [[ "$DRY_RUN" == "true" ]]; then
    echo ""
    log_success "DRY RUN: Would execute the following command:"
    echo ""
    echo "  ${AWS_BASE[*]} secretsmanager put-secret-value \\"
    echo "    --secret-id $SECRET_NAME \\"
    echo "    --secret-string file:///dev/stdin < <json-content>"
    echo ""
    log_info "JSON content (first 100 chars):"
    echo "$JSON_CONTENT" | head -c 100
    echo "..."
    echo ""
    log_info "To apply this change, run without --dry-run flag"
    exit 0
fi

log_info "Updating secret value..."

if $VERBOSE; then
    log_info "Executing: aws secretsmanager put-secret-value --secret-id $SECRET_NAME"
fi

if ! UPDATE_RESULT=$(echo "$JSON_CONTENT" | "${AWS_BASE[@]}" secretsmanager put-secret-value \
    --secret-id "$SECRET_NAME" \
    --secret-string file:///dev/stdin 2>&1); then
    log_error "Failed to update secret"
    log_error "AWS CLI output:"
    echo "$UPDATE_RESULT" | while IFS= read -r line; do
        log_detail "$line"
    done
    exit 1
fi

VERSION_ID=$(echo "$UPDATE_RESULT" | jq -r '.VersionId')

echo ""
log_success "Secret updated successfully!"
echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║                    SECRET UPDATED                              ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
printf "${CYAN}Secret details:${NC}\n"
printf "  Name:       %s\n" "$SECRET_NAME"
printf "  ARN:        %s\n" "$SECRET_ARN"
printf "  Region:     %s\n" "$AWS_REGION"
printf "  Version ID: %s\n" "$VERSION_ID"
echo ""

log_success "Operation completed successfully!"

echo ""
printf "${GREEN}Next steps:${NC}\n"
printf "  1. Verify secret: aws secretsmanager get-secret-value --secret-id %s --region %s\n" "$SECRET_NAME" "$AWS_REGION"
printf "  2. If using ESO, the secret will sync automatically to Kubernetes\n"
printf "  3. Restart pods to pick up new configuration (if needed)\n"
echo ""

exit 0
