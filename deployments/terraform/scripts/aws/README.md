# AWS scripts

Production-grade helper scripts for managing AWS resources provisioned by Terraform modules (EKS, ECS, etc.).

## Scripts

### `push-github-apps-to-aws-secrets-manager.sh`

Updates GitHub Apps configuration in AWS Secrets Manager.

**Usage:**
```bash
# Update secret
./push-github-apps-to-aws-secrets-manager.sh \
  --file github-apps.json \
  --secret-name gate-github-apps \
  --region us-east-1

# Update using AWS profile
./push-github-apps-to-aws-secrets-manager.sh \
  --file github-apps.json \
  --secret-name gate-github-apps \
  --profile production

# Dry run
./push-github-apps-to-aws-secrets-manager.sh \
  --file github-apps.json \
  --secret-name gate-github-apps \
  --dry-run
```

**Features:**
- JSON validation
- AWS credentials verification
- Secret existence checking
- Dry-run mode
- Verbose logging
- AWS profile support
- Bash 3.2+ compatible (macOS + Linux)

**Requirements:**
- `aws-cli` (configured with credentials)
- `jq` (for JSON validation)
- IAM permissions: `secretsmanager:PutSecretValue`

**See also:** `./push-github-apps-to-aws-secrets-manager.sh --help`

---

### `push-certificate-to-acm.sh`

Imports or updates SSL/TLS certificates in AWS Certificate Manager (ACM) from PKCS12/PFX files.

**Usage:**
```bash
# Import a new certificate
export CERT_PASSWORD="my-secure-password"
./push-certificate-to-acm.sh \
  --pfx-file certificate.pfx \
  --region us-east-1

# Update an existing certificate
export CERT_PASSWORD="my-secure-password"
./push-certificate-to-acm.sh \
  --pfx-file certificate.pfx \
  --certificate-arn arn:aws:acm:us-east-1:123456789012:certificate/abc123... \
  --profile production

# Import with tags
export CERT_PASSWORD="my-secure-password"
./push-certificate-to-acm.sh \
  --pfx-file certificate.pfx \
  --tags Environment=Production \
  --tags Application=WebAPI \
  --region us-east-1
```

**Features:**
- PKCS12/PFX extraction and validation
- Certificate chain handling
- Private key / certificate match verification
- Import new or update existing certificates
- Tagging support (simple and AWS CLI formats)
- AWS credentials verification
- Secure temporary file handling with automatic cleanup
- Verbose logging
- AWS profile support
- Bash 3.2+ compatible (macOS + Linux)

**Requirements:**
- `openssl` (for certificate extraction)
- `aws-cli` (configured with credentials)
- `jq` (for JSON parsing)
- IAM permissions: `acm:ImportCertificate`, `acm:DescribeCertificate` (and `acm:AddTagsToCertificate` for tagging)

**See also:** `./push-certificate-to-acm.sh --help`

## When to use

These scripts are specifically designed for **managing AWS resources after Terraform provisioning**.

### push-github-apps-to-aws-secrets-manager.sh

Use this script for updating secrets when using `lifecycle.ignore_changes` in Terraform modules.

#### Background: Terraform secrets management

The Terraform modules use `lifecycle.ignore_changes` on secret values to prevent accidental updates during normal operations. This means:

1. **Terraform creates the secret** (initial provisioning)
2. **lifecycle.ignore_changes protects the value** (normal operations)
3. **This script updates the value** (credential rotation)

#### Use cases

**1. Initial Secret Population (After Terraform Apply)**
```bash
# Terraform creates empty/placeholder secret
terraform apply

# Populate with actual credentials
./push-github-apps-to-aws-secrets-manager.sh \
  --file github-apps.json \
  --secret-name $(terraform output -raw github_apps_secret_name)
```

**2. Credential Rotation**
```bash
# Update GitHub App credentials without Terraform
./push-github-apps-to-aws-secrets-manager.sh \
  --file github-apps-rotated.json \
  --secret-name gate-github-apps
```

**3. Multiple Environments**
```bash
# Update dev environment
./push-github-apps-to-aws-secrets-manager.sh \
  --file github-apps-dev.json \
  --secret-name dev-github-apps \
  --profile dev

# Update prod environment
./push-github-apps-to-aws-secrets-manager.sh \
  --file github-apps-prod.json \
  --secret-name prod-github-apps \
  --profile production
```

### push-certificate-to-acm.sh

Use this script for importing or rotating SSL/TLS certificates in ACM from PKCS12/PFX files.

#### Use cases

**1. Initial Certificate Import**
```bash
# Import certificate after Terraform provisions the infrastructure
export CERT_PASSWORD="my-secure-password"
./push-certificate-to-acm.sh \
  --pfx-file certificate.pfx \
  --tags Environment=Production \
  --region us-east-1
```

**2. Certificate Renewal/Rotation**
```bash
# Update an existing certificate with a renewed one
export CERT_PASSWORD="my-secure-password"
./push-certificate-to-acm.sh \
  --pfx-file renewed-certificate.pfx \
  --certificate-arn arn:aws:acm:us-east-1:123456789012:certificate/abc123...
```

**3. Multiple Environments**
```bash
# Import to dev
export CERT_PASSWORD="dev-password"
./push-certificate-to-acm.sh \
  --pfx-file dev-certificate.pfx \
  --profile dev \
  --region us-east-1

# Import to prod
export CERT_PASSWORD="prod-password"
./push-certificate-to-acm.sh \
  --pfx-file prod-certificate.pfx \
  --profile production \
  --region us-east-1
```

## Integration with Terraform

### Recommended workflow

```bash
# 1. Import certificates (required before Terraform if referenced by CloudFront, ALB, etc.)
export CERT_PASSWORD="my-secure-password"
./scripts/aws/push-certificate-to-acm.sh \
  --pfx-file certificate.pfx \
  --tags Environment=Production \
  --region us-east-1

# 2. Provision infrastructure (references the ACM certificate ARN)
cd deployments/terraform/environments/prod
terraform apply

# 3. Get secret name from Terraform
SECRET_NAME=$(terraform output -raw github_apps_secret_name)

# 4. Update secret value (bypasses Terraform state)
cd ../../
./scripts/aws/push-github-apps-to-aws-secrets-manager.sh \
  --file github-apps.json \
  --secret-name "$SECRET_NAME"

# 5. If using EKS + ESO, secrets sync automatically to Kubernetes
# 6. Restart pods if needed
kubectl rollout restart deployment/gate -n gate
```

### Alternative: terraform replace

Instead of using the secrets script, you can force Terraform to update the secret:

```bash
terraform apply \
  -replace="module.gate.aws_secretsmanager_secret_version.github_apps" \
  -var="github_apps_secret_file=./github-apps.json"
```

**Trade-offs:**
- ✅ Script: Direct, no state changes, faster, recommended
- ✅ Terraform: Tracked in state, audit trail
- ⚠️ Script: Bypasses Terraform (but intended design)
- ⚠️ Terraform: File path exposed in command history

See Terraform module READMEs for detailed explanation of `lifecycle.ignore_changes` approach.

## Script quality

These scripts follow production-grade standards:
- Comprehensive error handling
- Structured logging with timestamps
- Input validation (JSON, certificates, AWS credentials)
- Dry-run mode for safety (secrets script)
- Verbose mode for debugging
- Compatible with Bash 3.2+ (macOS + Linux)
- Clear help messages with examples
- Exit codes for automation
- AWS profile support
