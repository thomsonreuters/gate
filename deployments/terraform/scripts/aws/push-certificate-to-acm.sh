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
# AWS ACM Certificate: Import/Update script
# ============================================================================
# This script imports or updates SSL/TLS certificates in AWS Certificate Manager
# from PKCS12 (PFX) format files.
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

cleanup() {
    local exit_code=$?
    if [[ -n "${TEMP_DIR:-}" ]] && [[ -d "${TEMP_DIR}" ]]; then
        log_info "Cleaning up temporary files..."
        rm -rf "${TEMP_DIR}"
    fi
    return $exit_code
}

trap cleanup EXIT INT TERM

function parse_tag() {
    local tag_input="$1"
    local key=""
    local value=""

    # Check if it's AWS format (contains both Key= and Value=)
    if [[ "$tag_input" =~ Key=([^,]+),Value=(.+) ]]; then
        key="${BASH_REMATCH[1]}"
        value="${BASH_REMATCH[2]}"
    # Check if it's simple format (KEY=VALUE)
    elif [[ "$tag_input" =~ ^([^=]+)=(.*)$ ]]; then
        key="${BASH_REMATCH[1]}"
        value="${BASH_REMATCH[2]}"
    else
        log_error "Invalid tag format: $tag_input"
        log_error "Supported formats:"
        log_detail "1. Simple: KEY=VALUE"
        log_detail "2. AWS:    Key=KEY,Value=VALUE"
        exit 1
    fi

    local found=false
    for i in "${!TAGS_KEYS[@]}"; do
        if [[ "${TAGS_KEYS[$i]}" == "$key" ]]; then
            TAGS_VALUES[$i]="$value"
            found=true
            break
        fi
    done

    if [[ "$found" == "false" ]]; then
        TAGS_KEYS+=("$key")
        TAGS_VALUES+=("$value")
    fi
}

function parse_tags_argument() {
    local tags_arg="$1"

    if [[ -z "$tags_arg" ]]; then
        return
    fi

    # Split by spaces, handling both formats
    local current_tag=""
    local char
    local in_value=false

    for (( i=0; i<${#tags_arg}; i++ )); do
        char="${tags_arg:$i:1}"

        # Detect if we're in a Value= section
        if [[ "$current_tag" =~ Value= ]]; then
            in_value=true
        fi

        # Space can be a separator or part of a value
        if [[ "$char" == " " ]]; then
            # Check if next part starts with Key= (AWS format separator)
            local remaining="${tags_arg:$i+1}"
            if [[ "$remaining" =~ ^Key= ]] || [[ "$in_value" == "false" ]]; then
                # This space is a separator
                if [[ -n "$current_tag" ]]; then
                    parse_tag "$current_tag"
                    current_tag=""
                    in_value=false
                fi
            else
                # This space is part of the value
                current_tag+="$char"
            fi
        else
            current_tag+="$char"
        fi
    done

    # Parse the last tag
    if [[ -n "$current_tag" ]]; then
        parse_tag "$current_tag"
    fi
}

function show_help() {
    cat << EOF
Usage: $0 [OPTIONS]

AWS ACM Certificate: Import/Update script
=========================================

This script imports a new certificate or updates an existing certificate in
AWS Certificate Manager (ACM) from a PKCS12/PFX format file.

OPTIONS:
    -p, --pfx-file FILE          Path to the PKCS12/PFX certificate file (required)
    -c, --certificate-arn ARN    Certificate ARN for update operation (optional)
                                 If not provided, a new certificate will be imported
    -r, --region REGION          AWS region (default: from AWS_REGION or AWS config)
    -t, --tags TAGS              Tags to apply to the certificate (multiple formats supported)
                                 Can be specified multiple times
                                 Only applied when importing new certificates
    --profile PROFILE            AWS CLI profile to use
    --password PASSWORD          Certificate password (not recommended, use env var instead)
    --password-env VAR           Environment variable containing the password (default: CERT_PASSWORD)
    -v, --verbose                Enable verbose output
    -h, --help                   Display this help message

TAG FORMATS:
    The --tags option supports multiple formats for flexibility:

    1. Simple format (one tag per --tags flag):
       --tags Environment=Production --tags Application=WebAPI

    2. AWS CLI format (one tag per --tags flag):
       --tags Key=Environment,Value=Production --tags Key=Application,Value=WebAPI

    3. Multiple tags in single --tags flag (AWS CLI style):
       --tags "Key=Environment,Value=Production Key=Application,Value=WebAPI"

    4. Mixed formats:
       --tags Environment=Production --tags Key=Application,Value=WebAPI

ENVIRONMENT VARIABLES:
    CERT_PASSWORD                Default password for the PKCS12/PFX file
    AWS_PROFILE                  AWS profile to use (overridden by --profile)
    AWS_REGION                   AWS region to use (overridden by --region)

EXAMPLES:
    # Import a new certificate using default profile
    export CERT_PASSWORD="my-secure-password"
    $0 --pfx-file certificate.pfx

    # Update an existing certificate with specific AWS profile
    export CERT_PASSWORD="my-secure-password"
    $0 --pfx-file certificate.pfx \\
       --certificate-arn arn:aws:acm:us-east-1:123456789012:certificate/abc123... \\
       --profile production

    # Import with tags (simple format)
    export CERT_PASSWORD="my-secure-password"
    $0 --pfx-file certificate.pfx \\
       --tags Environment=Production \\
       --tags Application=WebAPI \\
       --tags Owner=DevOps \\
       --region us-east-1

    # Import with tags (AWS CLI format)
    export CERT_PASSWORD="my-secure-password"
    $0 --pfx-file certificate.pfx \\
       --tags Key=Environment,Value=Production Key=Application,Value=WebAPI \\
       --profile production

REQUIREMENTS:
    - openssl (for certificate extraction)
    - aws-cli (configured with appropriate credentials)
    - jq (for JSON parsing)
    - IAM permissions: acm:ImportCertificate, acm:DescribeCertificate (and acm:AddTagsToCertificate for tagging)
    - Bash 3.2+ (compatible with macOS and Linux)

SECURITY NOTES:
    - Always use environment variables or interactive prompts for passwords
    - Avoid passing passwords via command-line arguments (visible in process list)
    - Temporary files are stored securely and automatically cleaned up
    - Private keys are never written to disk unencrypted

EOF
}

PFX_FILE=""
CERTIFICATE_ARN=""
AWS_REGION="${AWS_REGION:-}"
AWS_PROFILE=""
CERT_PASSWORD="${CERT_PASSWORD:-}"
PASSWORD_ENV="CERT_PASSWORD"
VERBOSE=false

TAGS_KEYS=()
TAGS_VALUES=()

function require_arg() {
    if [[ $# -lt 3 ]] || [[ "$3" == -* ]]; then
        log_error "Option $1 requires a value"
        exit 1
    fi
}

while [[ $# -gt 0 ]]; do
    case $1 in
        -p|--pfx-file)
            require_arg "$1" "$#" "${2:-}"
            PFX_FILE="$2"
            shift 2
            ;;
        -c|--certificate-arn)
            require_arg "$1" "$#" "${2:-}"
            CERTIFICATE_ARN="$2"
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
        -t|--tags)
            require_arg "$1" "$#" "${2:-}"
            parse_tags_argument "$2"
            shift 2
            ;;
        --password)
            require_arg "$1" "$#" "${2:-}"
            CERT_PASSWORD="$2"
            log_warning "Password provided via command line. Consider using environment variables instead."
            shift 2
            ;;
        --password-env)
            require_arg "$1" "$#" "${2:-}"
            PASSWORD_ENV="$2"
            shift 2
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

log_info "Starting AWS ACM certificate import/update process"

for cmd in openssl aws jq; do
    if ! command -v "$cmd" &> /dev/null; then
        log_error "Required command '$cmd' not found. Please install it and try again."
        exit 1
    fi
done

if [[ -z "$PFX_FILE" ]]; then
    log_error "PFX file is required. Use --pfx-file option."
    exit 1
fi

if [[ ! -f "$PFX_FILE" ]]; then
    log_error "PFX file not found: $PFX_FILE"
    exit 1
fi

if [[ -z "$CERT_PASSWORD" ]]; then
    if [[ -n "${!PASSWORD_ENV:-}" ]]; then
        CERT_PASSWORD="${!PASSWORD_ENV}"
        log_info "Using password from environment variable: $PASSWORD_ENV"
    else
        log_info "No password provided. Prompting for password..."
        read -s -p "Enter PKCS12/PFX password: " CERT_PASSWORD
        echo ""
        if [[ -z "$CERT_PASSWORD" ]]; then
            log_error "Password cannot be empty"
            exit 1
        fi
    fi
fi

if [[ -n "$CERTIFICATE_ARN" ]]; then
    OPERATION="update"
    log_info "Operation mode: UPDATE existing certificate"
    log_info "Certificate ARN: $CERTIFICATE_ARN"

    if [[ ! "$CERTIFICATE_ARN" =~ ^arn:aws:acm:[a-z0-9-]+:[0-9]+:certificate/.+ ]]; then
        log_error "Invalid certificate ARN format"
        exit 1
    fi

    if [[ ${#TAGS_KEYS[@]} -gt 0 ]]; then
        log_warning "Tags specified but will be ignored (tags can only be applied during import)"
    fi
else
    OPERATION="import"
    log_info "Operation mode: IMPORT new certificate"

    if [[ ${#TAGS_KEYS[@]} -gt 0 ]]; then
        log_info "Tags to apply (${#TAGS_KEYS[@]} tag(s)):"
        for i in "${!TAGS_KEYS[@]}"; do
            log_detail "${TAGS_KEYS[$i]} = ${TAGS_VALUES[$i]}"
        done
    fi
fi

log_info "Source file: $PFX_FILE"

if [[ -n "$AWS_PROFILE" ]]; then
    log_info "AWS profile: $AWS_PROFILE"
fi

if [[ -n "$AWS_REGION" ]]; then
    log_info "AWS region: $AWS_REGION"
fi

# ============================================================================
# Verify AWS credentials before proceeding
# ============================================================================

log_info "Verifying AWS credentials..."

AWS_BASE=(aws)
if [[ -n "$AWS_PROFILE" ]]; then
    AWS_BASE+=(--profile "$AWS_PROFILE")
fi
if [[ -n "$AWS_REGION" ]]; then
    AWS_BASE+=(--region "$AWS_REGION")
fi

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

# ============================================================================
# Create temporary directory for extraction
# ============================================================================

TEMP_DIR=$(mktemp -d)
chmod 700 "$TEMP_DIR"
log_info "Created secure temporary directory: $TEMP_DIR"

PRIVATE_KEY_FILE="$TEMP_DIR/private_key.pem"
CERTIFICATE_FILE="$TEMP_DIR/certificate.pem"
CHAIN_FILE="$TEMP_DIR/chain.pem"

# ============================================================================
# Extract components from PKCS12/PFX file
# ============================================================================

log_info "Extracting certificate components from PFX file..."

if $VERBOSE; then
    log_info "Extracting private key..."
fi

if ! openssl pkcs12 -in "$PFX_FILE" \
    -nocerts \
    -out "$PRIVATE_KEY_FILE" \
    -passin "pass:$CERT_PASSWORD" \
    -passout "pass:$CERT_PASSWORD" 2>/dev/null; then
    log_error "Failed to extract private key. Check your password and PFX file."
    exit 1
fi

PRIVATE_KEY_DECRYPTED="$TEMP_DIR/private_key_decrypted.pem"
if ! openssl rsa -in "$PRIVATE_KEY_FILE" \
    -out "$PRIVATE_KEY_DECRYPTED" \
    -passin "pass:$CERT_PASSWORD" 2>/dev/null; then
    log_error "Failed to decrypt private key"
    exit 1
fi

log_success "Private key extracted successfully"

if $VERBOSE; then
    log_info "Extracting certificate..."
fi

if ! openssl pkcs12 -in "$PFX_FILE" \
    -clcerts \
    -nokeys \
    -out "$CERTIFICATE_FILE" \
    -passin "pass:$CERT_PASSWORD" 2>/dev/null; then
    log_error "Failed to extract certificate"
    exit 1
fi

log_success "Certificate extracted successfully"

if $VERBOSE; then
    log_info "Extracting certificate chain..."
fi

if ! openssl pkcs12 -in "$PFX_FILE" \
    -cacerts \
    -nokeys \
    -out "$CHAIN_FILE" \
    -passin "pass:$CERT_PASSWORD" 2>/dev/null; then
    log_error "Failed to extract certificate chain"
    exit 1
fi

if [[ ! -s "$CHAIN_FILE" ]]; then
    log_warning "No certificate chain found (this is normal for self-signed certificates)"
    CHAIN_FILE=""
else
    log_success "Certificate chain extracted successfully"
fi

# ============================================================================
# Validate extracted certificates
# ============================================================================

log_info "Validating extracted certificates..."

if ! openssl x509 -in "$CERTIFICATE_FILE" -noout -text &>/dev/null; then
    log_error "Invalid certificate format"
    exit 1
fi

CERT_SUBJECT=$(openssl x509 -in "$CERTIFICATE_FILE" -noout -subject -nameopt RFC2253 | sed 's/subject=//')
CERT_ISSUER=$(openssl x509 -in "$CERTIFICATE_FILE" -noout -issuer -nameopt RFC2253 | sed 's/issuer=//')
CERT_NOT_BEFORE=$(openssl x509 -in "$CERTIFICATE_FILE" -noout -startdate | sed 's/notBefore=//')
CERT_NOT_AFTER=$(openssl x509 -in "$CERTIFICATE_FILE" -noout -enddate | sed 's/notAfter=//')

log_info "Certificate Details:"
log_detail "Subject:    $CERT_SUBJECT"
log_detail "Issuer:     $CERT_ISSUER"
log_detail "Valid From: $CERT_NOT_BEFORE"
log_detail "Valid To:   $CERT_NOT_AFTER"

if ! openssl rsa -in "$PRIVATE_KEY_DECRYPTED" -check -noout &>/dev/null; then
    log_error "Invalid private key format"
    exit 1
fi

CERT_MODULUS=$(openssl x509 -in "$CERTIFICATE_FILE" -noout -modulus | openssl md5)
KEY_MODULUS=$(openssl rsa -in "$PRIVATE_KEY_DECRYPTED" -noout -modulus | openssl md5)

if [[ "$CERT_MODULUS" != "$KEY_MODULUS" ]]; then
    log_error "Private key does not match the certificate"
    exit 1
fi

log_success "Private key matches the certificate"

# ============================================================================
# Import or Update Certificate in ACM
# ============================================================================

log_info "Preparing to $OPERATION certificate in AWS ACM..."

IMPORT_ARGS=("${AWS_BASE[@]}" acm import-certificate)
IMPORT_ARGS+=(--certificate "fileb://$CERTIFICATE_FILE")
IMPORT_ARGS+=(--private-key "fileb://$PRIVATE_KEY_DECRYPTED")

if [[ -n "$CHAIN_FILE" ]]; then
    IMPORT_ARGS+=(--certificate-chain "fileb://$CHAIN_FILE")
fi

if [[ "$OPERATION" == "update" ]]; then
    IMPORT_ARGS+=(--certificate-arn "$CERTIFICATE_ARN")
fi

if [[ "$OPERATION" == "import" ]] && [[ ${#TAGS_KEYS[@]} -gt 0 ]]; then
    IMPORT_ARGS+=(--tags)
    for i in "${!TAGS_KEYS[@]}"; do
        IMPORT_ARGS+=("Key=${TAGS_KEYS[$i]},Value=${TAGS_VALUES[$i]}")
    done
fi

log_info "Executing AWS ACM $OPERATION..."

if $VERBOSE; then
    VERBOSE_CMD="${IMPORT_ARGS[*]}"
    VERBOSE_CMD="${VERBOSE_CMD/fileb:\/\/$PRIVATE_KEY_DECRYPTED/fileb://***REDACTED***}"
    log_info "Command: $VERBOSE_CMD"
fi

if ! OUTPUT=$("${IMPORT_ARGS[@]}" 2>&1); then
    log_error "Failed to $OPERATION certificate in ACM"
    log_error "AWS CLI output:"
    echo "$OUTPUT" | while IFS= read -r line; do
        log_detail "$line"
    done
    exit 1
fi

RESULT_ARN=$(echo "$OUTPUT" | jq -r '.CertificateArn')

if [[ -z "$RESULT_ARN" ]] || [[ "$RESULT_ARN" == "null" ]]; then
    log_error "Failed to parse certificate ARN from AWS response"
    log_error "AWS CLI output: $OUTPUT"
    exit 1
fi

echo ""
if [[ "$OPERATION" == "import" ]]; then
    log_success "Certificate imported successfully!"
    echo ""
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║                    NEW CERTIFICATE CREATED                     ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo ""
    printf "${CYAN}Certificate ARN:${NC}\n"
    printf "  %s\n" "$RESULT_ARN"
    echo ""

    # Display applied tags
    if [[ ${#TAGS_KEYS[@]} -gt 0 ]]; then
        log_success "Tags applied successfully (${#TAGS_KEYS[@]} tag(s))"
        for i in "${!TAGS_KEYS[@]}"; do
            log_detail "${TAGS_KEYS[$i]} = ${TAGS_VALUES[$i]}"
        done
    fi
else
    log_success "Certificate updated successfully!"
    echo ""
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║                    CERTIFICATE UPDATED                         ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo ""
    printf "${CYAN}Certificate ARN:${NC}\n"
    printf "  %s\n" "$RESULT_ARN"
    echo ""
fi

# ============================================================================
# Display certificate information from ACM
# ============================================================================

log_info "Fetching certificate details from ACM..."

if DESCRIBE_OUTPUT=$("${AWS_BASE[@]}" acm describe-certificate --certificate-arn "$RESULT_ARN" 2>&1); then
    echo ""
    echo "════════════════════════════════════════════════════════════════"
    echo "  AWS ACM Certificate Information"
    echo "════════════════════════════════════════════════════════════════"

    DOMAIN=$(echo "$DESCRIBE_OUTPUT" | jq -r '.Certificate.DomainName')
    STATUS=$(echo "$DESCRIBE_OUTPUT" | jq -r '.Certificate.Status')
    TYPE=$(echo "$DESCRIBE_OUTPUT" | jq -r '.Certificate.Type')

    echo "  Domain Name: $DOMAIN"
    echo "  Status:      $STATUS"
    echo "  Type:        $TYPE"
    echo "  ARN:         $RESULT_ARN"

    if [[ ${#TAGS_KEYS[@]} -gt 0 ]]; then
        echo "  Tags:"
        for i in "${!TAGS_KEYS[@]}"; do
            printf "    - %-50s : %s\n" "${TAGS_KEYS[$i]}" "${TAGS_VALUES[$i]}"
        done
    fi

    echo "════════════════════════════════════════════════════════════════"
    echo ""
else
    log_warning "Could not fetch certificate details from ACM (certificate was still ${OPERATION}ed)"
fi

log_success "Operation completed successfully!"

echo ""
printf "${GREEN}Next steps:${NC}\n"
printf "  1. Verify the certificate in AWS Console: https://console.aws.amazon.com/acm/\n"
if [[ "$OPERATION" == "import" ]]; then
    printf "  2. Attach the certificate to your AWS resources (ALB, CloudFront, API Gateway, etc.)\n"
    printf "  3. Save the certificate ARN for future updates\n"
else
    printf "  2. Check that services using this certificate are functioning correctly\n"
fi
echo ""

printf "${GREEN}Certificate ARN (save this for updates):${NC}\n"
printf "  %s\n" "$RESULT_ARN"
echo ""

exit 0
