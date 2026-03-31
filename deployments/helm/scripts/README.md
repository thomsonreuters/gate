# Helm deployment scripts

Production-grade helper scripts for Kubernetes deployment with Helm.

## Scripts

### `create-kubernetes-secret.sh`

Creates or updates Kubernetes Secrets for GitHub Apps private keys. The script reads a JSON file describing each app (`client_id`, `organization`, `private_key_path`), validates the PEM files exist, and stores them as `app-0.pem`, `app-1.pem`, etc. in the secret — matching the Helm chart's expected format.

**Usage:**
```bash
# Create new secret
./create-kubernetes-secret.sh \
  --file github-apps.json \
  --namespace gate

# Update existing secret
./create-kubernetes-secret.sh \
  --file github-apps.json \
  --namespace gate \
  --update

# Dry run
./create-kubernetes-secret.sh \
  --file github-apps.json \
  --namespace gate \
  --dry-run
```

**Input JSON format:**
```json
[
  {
    "client_id": "Iv1...",
    "organization": "your-org",
    "private_key_path": "/path/to/private-key.pem"
  }
]
```

**Features:**
- JSON validation and per-app field validation
- PEM file existence checking
- Kubernetes name validation
- Namespace existence checking
- Secret conflict detection
- Dry-run mode
- Verbose logging
- Labels support
- Bash 3.2+ compatible (macOS + Linux)

**Requirements:**
- `kubectl` (configured with cluster access)
- `jq` (for JSON validation and parsing)

**See also:** `./create-kubernetes-secret.sh --help`

## When to use

- **Manual Kubernetes deployments** - When not using External Secrets Operator
- **Development/testing** - Quick secret creation for local testing
- **Secret rotation** - Updating GitHub Apps credentials
- **CI/CD pipelines** - Automated secret management in deployment workflows

## Integration with Helm

These scripts complement the Helm chart deployment:

1. **Before Helm install:** Create secrets
   ```bash
   ./create-kubernetes-secret.sh --file github-apps.json --namespace gate
   helm install gate ../gate
   ```

2. **After credential rotation:** Update secrets
   ```bash
   ./create-kubernetes-secret.sh --file github-apps-new.json --namespace gate --update
   kubectl rollout restart deployment/gate -n gate
   ```

## Script quality

These scripts follow production-grade standards:
- Comprehensive error handling
- Structured logging with timestamps
- Input validation (JSON, PEM files, K8s names, namespaces)
- Dry-run mode for safety
- Verbose mode for debugging
- Compatible with Bash 3.2+ (macOS + Linux)
- Clear help messages with examples
- Exit codes for automation
