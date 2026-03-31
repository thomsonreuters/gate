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
# GitHub Apps: Kubernetes secret creation script
# ============================================================================
# This script creates a Kubernetes Secret containing GitHub App private keys
# from a JSON configuration file. Each app's private_key_path is read and
# stored as app-0.pem, app-1.pem, etc. in the secret, matching the Helm
# chart's expected format.
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

GitHub Apps: Kubernetes secret creation script
==============================================

This script creates a Kubernetes Secret containing GitHub App private keys.
It reads a JSON file describing each app (client_id, organization, and
private_key_path), extracts the PEM files, and stores them in the secret
as app-0.pem, app-1.pem, etc. — matching the Helm chart's expected format.

OPTIONS:
    -f, --file FILE              Path to GitHub Apps JSON file (required)
    -s, --secret-name NAME       Kubernetes secret name (default: github-apps)
    -n, --namespace NAMESPACE    Kubernetes namespace (default: default)
    -c, --context CONTEXT        Kubernetes context to use (default: current context)
    -l, --labels LABELS          Labels to apply to the secret (KEY=VALUE format)
    --update                     Update existing secret (default: create new)
    --dry-run                    Show what would be done without making changes
    -v, --verbose                Enable verbose output
    -h, --help                   Display this help message

LABEL FORMAT:
    Labels must follow Kubernetes label conventions:

    --labels app=gate --labels environment=production
    --labels "app=gate environment=production"

ENVIRONMENT VARIABLES:
    KUBECONFIG                   Path to kubeconfig file

EXAMPLES:
    # Create a new secret (minimal)
    $0 --file github-apps.json

    # Create secret in specific namespace
    $0 --file github-apps.json \\
       --namespace gate

    # Create secret with custom name and labels
    $0 --file github-apps.json \\
       --secret-name my-github-apps \\
       --namespace gate \\
       --labels app=gate \\
       --labels environment=production

    # Update an existing secret
    $0 --file github-apps.json \\
       --secret-name github-apps \\
       --namespace gate \\
       --update

    # Dry run to preview changes
    $0 --file github-apps.json \\
       --namespace gate \\
       --dry-run

REQUIREMENTS:
    - jq (for JSON validation and parsing)
    - kubectl (configured with appropriate cluster access)
    - Kubernetes RBAC permissions: secrets create/update in target namespace
    - Bash 3.2+ (compatible with macOS and Linux)

SECURITY NOTES:
    - Private key files should have restrictive permissions (chmod 600)
    - Secret will be created in Kubernetes and accessible by pods in the namespace
    - Consider using External Secrets Operator for automated sync from a
      secret management service

GITHUB APPS JSON FORMAT:
    [
      {
        "client_id": "Iv1...",
        "private_key_path": "/path/to/key.pem",
        "organization": "my-org"
      }
    ]

    Each entry's private_key_path must point to an existing PEM file.
    The script stores them as app-0.pem, app-1.pem, etc. in the secret.

EOF
}

LABELS_KEYS=()
LABELS_VALUES=()

function parse_label() {
    local label_input="$1"
    local key=""
    local value=""

    if [[ "$label_input" =~ ^([^=]+)=(.*)$ ]]; then
        key="${BASH_REMATCH[1]}"
        value="${BASH_REMATCH[2]}"
    else
        log_error "Invalid label format: $label_input"
        log_error "Expected format: KEY=VALUE"
        exit 1
    fi

    if [[ ! "$key" =~ ^[a-zA-Z0-9]([-a-zA-Z0-9_.]*[a-zA-Z0-9])?(/[a-zA-Z0-9]([-a-zA-Z0-9_.]*[a-zA-Z0-9])?)?$ ]]; then
        log_error "Invalid Kubernetes label key: $key"
        log_error "Label keys must consist of alphanumeric characters, '-', '_' or '.'"
        exit 1
    fi

    LABELS_KEYS+=("$key")
    LABELS_VALUES+=("$value")
}

function parse_labels_argument() {
    local labels_arg="$1"
    if [[ -z "$labels_arg" ]]; then
        return
    fi

    local IFS=' '
    read -ra LABELS <<< "$labels_arg"
    for label in "${LABELS[@]}"; do
        if [[ -n "$label" ]]; then
            parse_label "$label"
        fi
    done
}

JSON_FILE=""
SECRET_NAME="github-apps"
NAMESPACE="default"
KUBE_CONTEXT=""
UPDATE_MODE=false
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
        -n|--namespace)
            require_arg "$1" "$#" "${2:-}"
            NAMESPACE="$2"
            shift 2
            ;;
        -c|--context)
            require_arg "$1" "$#" "${2:-}"
            KUBE_CONTEXT="$2"
            shift 2
            ;;
        -l|--labels)
            require_arg "$1" "$#" "${2:-}"
            parse_labels_argument "$2"
            shift 2
            ;;
        --update)
            UPDATE_MODE=true
            shift
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

log_info "Starting secret creation process"

for cmd in jq kubectl; do
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

APP_COUNT=$(jq 'length' "$JSON_FILE")
if [[ "$APP_COUNT" -lt 1 ]]; then
    log_error "JSON file must contain at least one GitHub App entry"
    exit 1
fi

for i in $(seq 0 $((APP_COUNT - 1))); do
    CLIENT_ID=$(jq -r ".[$i].client_id // empty" "$JSON_FILE")
    ORGANIZATION=$(jq -r ".[$i].organization // empty" "$JSON_FILE")
    PEM_PATH=$(jq -r ".[$i].private_key_path // empty" "$JSON_FILE")

    if [[ -z "$CLIENT_ID" ]]; then
        log_error "App entry [$i] is missing 'client_id'"
        exit 1
    fi
    if [[ -z "$ORGANIZATION" ]]; then
        log_error "App entry [$i] is missing 'organization'"
        exit 1
    fi
    if [[ -z "$PEM_PATH" ]]; then
        log_error "App entry [$i] is missing 'private_key_path'"
        exit 1
    fi
    if [[ ! -f "$PEM_PATH" ]]; then
        log_error "Private key file not found for app [$i]: $PEM_PATH"
        exit 1
    fi
done

log_info "Validated $APP_COUNT GitHub App(s) in $JSON_FILE"

if [[ ! "$SECRET_NAME" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]]; then
    log_error "Invalid secret name: $SECRET_NAME"
    log_error "Secret names must consist of lowercase alphanumeric characters or '-'"
    exit 1
fi

if [[ ! "$NAMESPACE" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]]; then
    log_error "Invalid namespace: $NAMESPACE"
    exit 1
fi

if [[ "$UPDATE_MODE" == "true" ]]; then
    OPERATION="update"
    log_info "Operation mode: UPDATE existing secret"
else
    OPERATION="create"
    log_info "Operation mode: CREATE new secret"
fi

log_info "Secret name: $SECRET_NAME"
log_info "Namespace: $NAMESPACE"
log_info "Source file: $JSON_FILE"

for i in $(seq 0 $((APP_COUNT - 1))); do
    CLIENT_ID=$(jq -r ".[$i].client_id" "$JSON_FILE")
    ORGANIZATION=$(jq -r ".[$i].organization" "$JSON_FILE")
    PEM_PATH=$(jq -r ".[$i].private_key_path" "$JSON_FILE")
    log_detail "app-$i.pem: client_id=$CLIENT_ID org=$ORGANIZATION key=$PEM_PATH"
done

if [[ ${#LABELS_KEYS[@]} -gt 0 ]]; then
    log_info "Labels to apply (${#LABELS_KEYS[@]} label(s)):"
    for i in "${!LABELS_KEYS[@]}"; do
        log_detail "${LABELS_KEYS[$i]} = ${LABELS_VALUES[$i]}"
    done
fi

if [[ "$DRY_RUN" == "true" ]]; then
    log_warning "DRY RUN MODE: No changes will be made"
fi

# ============================================================================
# Verify Kubernetes access
# ============================================================================

log_info "Verifying Kubernetes cluster access..."

KUBECTL_BASE=(kubectl)

if [[ -n "$KUBE_CONTEXT" ]]; then
    KUBECTL_BASE+=(--context "$KUBE_CONTEXT")
    log_info "Using Kubernetes context: $KUBE_CONTEXT"
fi

if ! CURRENT_CONTEXT=$("${KUBECTL_BASE[@]}" config current-context 2>&1); then
    log_error "Failed to get current Kubernetes context"
    log_error "Please check your kubeconfig and cluster access"
    exit 1
fi

log_info "Current context: $CURRENT_CONTEXT"

if ! CLUSTER_INFO=$("${KUBECTL_BASE[@]}" cluster-info 2>&1 | head -1); then
    log_error "Failed to connect to Kubernetes cluster"
    exit 1
fi

log_success "Kubernetes cluster access verified"
log_detail "$CLUSTER_INFO"

if ! "${KUBECTL_BASE[@]}" get namespace "$NAMESPACE" &>/dev/null; then
    log_error "Namespace '$NAMESPACE' does not exist"
    log_error "Please create the namespace first or use an existing one"
    exit 1
fi

log_success "Namespace '$NAMESPACE' exists"

if "${KUBECTL_BASE[@]}" get secret "$SECRET_NAME" -n "$NAMESPACE" &>/dev/null; then
    if [[ "$UPDATE_MODE" == "false" ]]; then
        log_error "Secret '$SECRET_NAME' already exists in namespace '$NAMESPACE'"
        log_error "Use --update flag to update the existing secret"
        exit 1
    fi
    log_info "Secret '$SECRET_NAME' exists and will be updated"
else
    if [[ "$UPDATE_MODE" == "true" ]]; then
        log_warning "Secret '$SECRET_NAME' does not exist, will create instead"
        OPERATION="create"
    fi
fi

# ============================================================================
# Build --from-file arguments (one per app PEM)
# ============================================================================

FROM_FILE_ARGS=()
for i in $(seq 0 $((APP_COUNT - 1))); do
    PEM_PATH=$(jq -r ".[$i].private_key_path" "$JSON_FILE")
    FROM_FILE_ARGS+=("--from-file=app-${i}.pem=${PEM_PATH}")
done

# ============================================================================
# Create or update secret in Kubernetes
# ============================================================================

log_info "Preparing to $OPERATION secret in Kubernetes..."

CREATE_CMD=("${KUBECTL_BASE[@]}" create secret generic "$SECRET_NAME")
CREATE_CMD+=("${FROM_FILE_ARGS[@]}")
CREATE_CMD+=("--namespace=$NAMESPACE")

if [[ "$DRY_RUN" == "true" ]]; then
    CREATE_CMD+=(--dry-run=client -o yaml)
else
    if [[ ${#LABELS_KEYS[@]} -gt 0 ]]; then
        for i in "${!LABELS_KEYS[@]}"; do
            CREATE_CMD+=("--labels=${LABELS_KEYS[$i]}=${LABELS_VALUES[$i]}")
        done
    fi
fi

log_info "Executing Kubernetes secret $OPERATION..."

if $VERBOSE; then
    log_info "Command: ${CREATE_CMD[*]}"
fi

if [[ "$DRY_RUN" != "true" && "$OPERATION" == "update" ]]; then
    if ! "${KUBECTL_BASE[@]}" delete secret "$SECRET_NAME" \
            --namespace="$NAMESPACE" --ignore-not-found=true &>/dev/null; then
        log_error "Failed to delete existing secret"
        exit 1
    fi
fi

if ! OUTPUT=$("${CREATE_CMD[@]}" 2>&1); then
    log_error "Failed to $OPERATION secret in Kubernetes"
    log_error "kubectl output:"
    echo "$OUTPUT" | while IFS= read -r line; do
        log_detail "$line"
    done
    exit 1
fi

if [[ "$DRY_RUN" == "true" ]]; then
    echo ""
    log_success "DRY RUN: Secret definition generated successfully"
    echo ""
    echo "════════════════════════════════════════════════════════════════"
    echo "  Generated secret YAML (dry-run, values redacted)"
    echo "════════════════════════════════════════════════════════════════"
    echo "$OUTPUT" | awk '
        /^(data|stringData):/  { in_data=1; print; next }
        /^[a-zA-Z]/            { in_data=0 }
        in_data && /^  [a-zA-Z0-9_.-]+:/ {
            sub(/:[[:space:]].*$/, ": <REDACTED>")
            sub(/:$/, ": <REDACTED>")
        }
        { print }
    '
    echo "════════════════════════════════════════════════════════════════"
    echo ""
    log_info "To apply this secret, run without --dry-run flag"
    exit 0
fi

echo ""
log_success "Secret ${OPERATION}d successfully!"
echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
if [[ "$OPERATION" == "create" ]]; then
    echo "║               NEW SECRET CREATED                               ║"
else
    echo "║               SECRET UPDATED                                   ║"
fi
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
printf "${CYAN}Secret details:${NC}\n"
printf "  Name:      %s\n" "$SECRET_NAME"
printf "  Namespace: %s\n" "$NAMESPACE"
printf "  Keys:      "
for i in $(seq 0 $((APP_COUNT - 1))); do
    if [[ $i -gt 0 ]]; then
        printf ", "
    fi
    printf "app-%d.pem" "$i"
done
printf "\n"
echo ""

if [[ ${#LABELS_KEYS[@]} -gt 0 ]]; then
    log_success "Labels applied (${#LABELS_KEYS[@]} label(s))"
    for i in "${!LABELS_KEYS[@]}"; do
        log_detail "${LABELS_KEYS[$i]} = ${LABELS_VALUES[$i]}"
    done
    echo ""
fi

# ============================================================================
# Display secret information from Kubernetes
# ============================================================================

log_info "Fetching secret details from Kubernetes..."

if SECRET_INFO=$("${KUBECTL_BASE[@]}" get secret "$SECRET_NAME" -n "$NAMESPACE" -o wide 2>&1); then
    echo ""
    echo "════════════════════════════════════════════════════════════════"
    echo "  Kubernetes secret information"
    echo "════════════════════════════════════════════════════════════════"
    echo "$SECRET_INFO" | while IFS= read -r line; do
        echo "  $line"
    done
    echo "════════════════════════════════════════════════════════════════"
    echo ""
fi

log_success "Operation completed successfully!"

echo ""
printf "${GREEN}Next steps:${NC}\n"
printf "  1. Verify secret: kubectl describe secret %s -n %s\n" "$SECRET_NAME" "$NAMESPACE"
printf "  2. Deploy Helm chart with: helm install gate ../gate\n"
echo ""

exit 0
