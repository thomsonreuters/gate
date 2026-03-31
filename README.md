# GATE: GitHub Authenticated Token Exchange

GATE is a Security Token Service (STS) that exchanges OIDC tokens from trusted identity providers for short-lived GitHub App installation tokens with fine-grained permissions. It enables organizations to eliminate long-lived Personal Access Tokens (PATs) by providing a centralized, auditable mechanism for issuing repository-scoped credentials based on workload identity.

Modern CI/CD pipelines and automation workflows frequently require access to GitHub repositories, but managing static credentials at scale introduces significant security risks. GATE addresses this challenge by allowing workloads to authenticate using their native identity (such as a GitHub Actions workflow or a Kubernetes service account) and receive ephemeral tokens scoped to exactly the permissions they need.

## Table of contents

- [1. How it works](#how-it-works)
- [2. Key capabilities](#key-capabilities)
- [3. Getting started](#getting-started)
- [4. Configuration](#configuration)
- [5. API reference](#api-reference)
- [6. Development](#development)
- [7. Deployment](#deployment)
- [8. FIPS 140-3 compliance](#fips-140-3-compliance)
- [9. Security considerations](#security-considerations)
- [10. Contributing](#contributing)
- [11. License](#license)

## How it works

GATE implements a two-layer authorization model that balances centralized governance with repository self-service. When a client presents an OIDC token, GATE validates it against the configured identity providers, then evaluates the request against both organizational policies and repository-specific trust policies before issuing a GitHub installation token.

```
┌─────────────────┐                           ┌─────────────────┐
│     Client      │                           │      GATE       │
│   (Workflow)    │                           │                 │
└────────┬────────┘                           └────────┬────────┘
         │                                             │
         │  1. Request token with OIDC credential      │
         │────────────────────────────────────────────►│
         │                                             │
         │                            2. Validate OIDC token against
         │                               configured providers
         │                                             │
         │                            3. Evaluate central policy
         │                               (org-wide rules)
         │                                             │
         │                            4. Fetch and evaluate trust
         │                               policy from target repo
         │                                             │
         │                            5. Generate scoped installation
         │                               token via GitHub App
         │                                             │
         │                            6. Log audit entry
         │                                             │
         │  7. Return short-lived token                │
         │◄────────────────────────────────────────────│
         │                                             │
```

The central configuration establishes organization-wide guardrails: which OIDC issuers are trusted, what claims must be present, which permissions can never be granted, and global token lifetime limits. Repository trust policies, maintained by repository owners in their own repositories, define the specific conditions under which tokens may be issued for that repository.

This separation allows platform teams to enforce security baselines while enabling development teams to manage their own access policies without requiring central approval for routine changes.

## Key capabilities

GATE validates OIDC tokens by fetching issuer metadata and JSON Web Key Sets, verifying signatures, and checking standard claims such as audience, expiration, and issuer. Beyond standard validation, GATE supports requiring specific claims with pattern matching (for example, ensuring that only workflows from a particular GitHub organization can request tokens) and forbidding claims that indicate untrusted contexts.

The authorization engine evaluates requests against configurable rules that can match any combination of OIDC claims. Rules support both AND and OR logic, enabling policies that express complex requirements such as "must be from the main branch AND triggered by a push event" or "must be from either the release branch OR tagged as a production deployment." Time-based restrictions can limit token issuance to specific hours or days.

Tokens issued by GATE are scoped to a single repository and carry only the permissions explicitly granted by both the central policy and the matching trust policy. The effective permissions are always the intersection of what was requested, what the trust policy allows, and what the central policy permits. Token lifetimes default to 15 minutes and cannot exceed the organization's configured maximum, ensuring that even if a token is compromised, its utility is limited.

Every token request generates an audit entry capturing the requester's identity, the target repository, the requested and granted permissions, the matched policy, and the outcome. GATE supports multiple audit backends including console output for development, PostgreSQL for queryable storage, and DynamoDB for serverless deployments.

For organizations with high token issuance rates, GATE can distribute load across multiple GitHub Apps using pluggable selection strategies. The selector tracks rate limit consumption across apps and routes requests to apps with available capacity, with state stored in memory, Redis, or DynamoDB depending on deployment topology.

## Getting started

GATE requires Go 1.26 or later for building from source, one or more GitHub Apps installed on the target organization, and an OIDC identity provider that your workloads can authenticate against.

Clone the repository and build the binary:

```bash
git clone https://github.com/thomsonreuters/gate.git
cd gate
make build
```

Create a minimal configuration file. GATE uses a YAML config file and supports environment variable overrides with the `GATE_` prefix. Create a `config.yaml` in the working directory:

```yaml
logger:
  level: info
  format: json

oidc:
  audience: your-gate-instance  # defaults to "gate" if omitted

policy:
  version: "1.0"
  trust_policy_path: ".github/gate/trust-policy.yaml"
  default_token_ttl: 900
  max_token_ttl: 3600
  providers:
    - issuer: https://token.actions.githubusercontent.com
      name: GitHub Actions
  max_permissions:
    contents: read
    metadata: read

github_apps:
  - client_id: "Iv1..."
    organization: your-org
    private_key_path: /path/to/private-key.pem
```

Any config value can be overridden via environment variables using the `GATE_` prefix with underscores replacing dots (e.g., `GATE_LOGGER_LEVEL=debug`, `GATE_OIDC_AUDIENCE=my-gate`). If no config file is found, GATE falls back to environment variables and built-in defaults.

You can also specify a config file path explicitly using the `--config` flag:

```bash
./bin/gate server --config /etc/gate/config.yaml
```

Start the service:

```bash
./bin/gate server
```

GATE will listen on port 8080 by default. Verify it's running by checking the health endpoint:

```bash
curl http://localhost:8080/health
```

## Configuration

### Policy configuration

The policy section of the config file defines organization-wide rules that apply to all token requests. It specifies which identity providers are trusted, what claims are required or forbidden, permission ceilings, and default token behavior.

```yaml
policy:
  version: "1.0"

  # Path pattern for locating trust policies in target repositories.
  trust_policy_path: ".github/gate/trust-policy.yaml"

  # Default and maximum token lifetimes in seconds.
  default_token_ttl: 900
  max_token_ttl: 3600

  # When true, requests fail if no trust policy exists in the target repository.
  require_explicit_policy: true

  # Trusted OIDC identity providers.
  providers:
    - issuer: "https://token.actions.githubusercontent.com"
      name: "GitHub Actions"

      # Claims that must be present with values matching the given patterns.
      required_claims:
        repository_owner:
          pattern: "^your-org$"
          description: "Only workflows from your-org may request tokens"

      # Claims that must not be present, or must not match if present.
      forbidden_claims:
        actor:
          pattern: "^dependabot"
          description: "Dependabot workflows are not permitted"

  # Maximum permission levels that can ever be granted.
  # Requests exceeding these levels are denied regardless of trust policy.
  # Permissions not listed are implicitly forbidden.
  max_permissions:
    contents: write
    issues: write
    packages: write
    metadata: read
```

### Trust policies

Trust policies are YAML files stored in target repositories that define who can obtain tokens for that repository and with what permissions. Repository owners create and maintain these policies, giving them control over their repository's security posture within the guardrails established by the central configuration.

```yaml
version: "1.0"

trust_policies:
  # Policy for CI workflows that need read access
  - name: "ci-read"
    description: "Allow CI workflows to read repository contents"
    issuer: "https://token.actions.githubusercontent.com"

    rules:
      - name: "main-branch"
        logic: AND
        conditions:
          - field: "repository"
            pattern: "^your-org/your-repo$"
          - field: "ref"
            pattern: "^refs/heads/main$"
          - field: "event_name"
            pattern: "^(push|pull_request)$"

    permissions:
      contents: read
      packages: read

    token_ttl: 600

  # Policy for release workflows that need write access
  - name: "release-publish"
    description: "Allow release workflows to publish packages"
    issuer: "https://token.actions.githubusercontent.com"

    rules:
      - name: "tagged-release"
        logic: AND
        conditions:
          - field: "repository"
            pattern: "^your-org/your-repo$"
          - field: "ref_type"
            pattern: "^tag$"
          - field: "ref"
            pattern: "^refs/tags/v[0-9]+\\.[0-9]+\\.[0-9]+$"

    permissions:
      contents: write
      packages: write

    token_ttl: 900
```

When GATE receives a token request, it fetches the trust policy from the target repository, evaluates each policy in order, and uses the first policy whose rules match the request's OIDC claims. If no policy matches and `require_explicit_policy` is enabled, the request is denied.

### Origin verification

When GATE is fronted by a CDN or reverse proxy such as CloudFront, origin verification ensures that requests reach the application only through the intended path. GATE validates a shared secret header on every request and rejects traffic that bypasses the proxy.

```yaml
origin:
  enabled: true
  header_name: "X-Origin-Verify"
  header_value: "${GATE_ORIGIN_HEADER_VALUE}"
```

The shared secret is injected via the `GATE_ORIGIN_HEADER_VALUE` environment variable. See the [Helm chart documentation](deployments/helm/gate/README.md) for Kubernetes configuration and the [Terraform modules](deployments/terraform/modules/aws/README.md) for CloudFront setup.

## API reference

### Exchange endpoint

The exchange endpoint accepts an OIDC token and returns a GitHub installation token scoped to the specified repository.

**Request**

```
POST /api/v1/exchange
Content-Type: application/json
```

```json
{
  "oidc_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
  "target_repository": "your-org/your-repo",
  "policy_name": "ci-read",
  "requested_permissions": {
    "contents": "read",
    "packages": "read"
  },
  "requested_ttl": 600
}
```

The `policy_name` field is optional; if omitted, GATE evaluates all policies in the trust policy file and uses the first match. The `requested_permissions` and `requested_ttl` fields are also optional and default to the values specified in the matched policy.

**Response**

```json
{
  "token": "ghs_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "expires_at": "2025-01-15T10:30:00Z",
  "matched_policy": "ci-read",
  "permissions": {
    "contents": "read",
    "packages": "read"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Error response**

```json
{
  "error": "authorization denied: no matching trust policy",
  "error_code": "NO_RULES_MATCHED",
  "request_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

Error codes returned by the API:

**400 Bad Request:**
- `INVALID_REQUEST` - Invalid request format or missing required fields

**401 Unauthorized:**
- `INVALID_TOKEN` - Malformed or expired OIDC token

**403 Forbidden (authorization denial codes):**
- `ISSUER_NOT_ALLOWED` - OIDC token issuer is not in the allowed providers list
- `REQUIRED_CLAIM_MISMATCH` - Required claim is missing or doesn't match the configured pattern
- `FORBIDDEN_CLAIM_MATCHED` - OIDC token claim matches a forbidden pattern
- `TIME_RESTRICTION` - Request made outside allowed time windows
- `TRUST_POLICY_NOT_FOUND` - No trust policy file found in the target repository
- `NO_RULES_MATCHED` - No policy rules matched the OIDC token claims
- `POLICY_NAME_REQUIRED` - Policy name must be specified when require_explicit_policy is enabled
- `PERMISSION_NOT_IN_POLICY` - Requested permission is not granted by the matched policy
- `PERMISSION_EXCEEDS_POLICY` - Permission level exceeds the policy allowance
- `PERMISSION_EXCEEDS_ORG_MAX` - Permission level exceeds the organization-wide maximum
- `PERMISSION_DENIED` - Requested permission is explicitly denied
- `PERMISSION_NOT_IN_MAX_PERMISSIONS` - Requested permission is not in the allowed permissions list
- `NON_REPOSITORY_PERMISSION` - Requested permission is not a repository-scoped permission

**404 Not Found:**
- `POLICY_NOT_FOUND` - Explicitly named policy doesn't exist in the repository
- `REPOSITORY_NOT_FOUND` - Target repository not found or not accessible

**429 Too Many Requests:**
- `RATE_LIMITED` - Rate limit exceeded (includes Retry-After header)

**500 Internal Server Error:**
- `POLICY_LOAD_FAILED` - Failed to load trust policy from the repository
- `INTERNAL_ERROR` - Unexpected internal server error (includes app selection failures and missing GitHub client configurations)

**502 Bad Gateway:**
- `GITHUB_API_ERROR` - GitHub API request failed

### Health endpoint

GATE exposes a health endpoint for orchestration systems:

- `GET /health` returns 200 if the service is running

### Info endpoint

GATE exposes an info endpoint that reports service metadata including FIPS 140-3 status:

- `GET /api/v1/info` returns 200 with `{"fips_enabled":true}` or `{"fips_enabled":false}`

## Development

### Running tests

GATE separates unit tests from integration tests using Go build tags. Unit tests validate individual components in isolation and run quickly without external dependencies. Integration tests exercise the complete request flow through the service using an in-memory test harness with mock GitHub API and OIDC provider servers.

```bash
make test                 # Run all tests (unit + integration)
make test-unit            # Run unit tests with coverage
make test-integration     # Run integration tests
```

Unit tests generate a coverage report in the `coverage/` directory and display the total coverage percentage. All test commands include race detection to identify potential concurrency issues. Integration tests verify end-to-end behavior including OIDC token validation, policy evaluation, permission handling, and GitHub token issuance. They require no external services as all dependencies are mocked within the test harness.

### Code quality

The project enforces consistent code style and catches common issues through automated tooling:

```bash
make fmt      # Format code with gofmt
make vet      # Run go vet for static analysis
make lint     # Run golangci-lint for comprehensive linting
make check    # Run all quality checks (fmt, vet, lint, test)
```

Before submitting changes, run `make check` to ensure all quality gates pass.

### Pre-commit hooks

The repository includes a pre-commit configuration that automates quality checks before each commit. Pre-commit hooks catch issues early, reducing CI failures and maintaining consistent code quality across contributors.

To install the hooks:

```bash
pip install pre-commit
pre-commit install
```

Once installed, the hooks run automatically on every commit. They perform general hygiene checks such as detecting large files, validating JSON and YAML syntax, fixing trailing whitespace, and scanning for accidentally committed private keys. Go-specific hooks run `go mod tidy`, execute tests, verify the build, and apply golangci-lint with automatic fixes. For infrastructure code, the hooks validate Dockerfiles with hadolint, format Terraform files, and lint Helm charts.

To run all hooks manually against the entire codebase:

```bash
pre-commit run --all-files
```

If a hook fails, fix the reported issues and retry the commit. Some hooks like `go-mod-tidy` and `golangci-lint --fix` automatically correct issues, so you may only need to stage the changes and commit again.

## Deployment

### Building from source

The Makefile provides targets for building and packaging:

```bash
make build                                      # Build the binary
make image-build                                # Build container image
make image-push REGISTRY=your-registry.com     # Push to registry
make help                                       # See all available targets
```

Container images are built using a multi-stage Dockerfile with a distroless runtime image for minimal attack surface. Base images are pinned by SHA256 digest for reproducibility.

### Kubernetes with Helm

The [Helm chart](deployments/helm/gate/README.md) provides a production-ready Kubernetes deployment with configurable replicas, resource limits, pod disruption budgets, horizontal pod autoscaling, network policies, and ingress.

```bash
helm install gate deployments/helm/gate \
  --namespace gate \
  --create-namespace \
  --set image.repository=your-registry.com/your-org/gate \
  --set image.tag=1.0.0
```

The chart supports External Secrets Operator for retrieving GitHub App credentials from secret management services like AWS Secrets Manager, HashiCorp Vault, or Azure Key Vault. See `values.yaml` for the full set of configuration options.

### AWS with Terraform

[Terraform modules](deployments/terraform/modules/aws/README.md) provide infrastructure-as-code for deploying GATE to AWS. The EKS module provisions the supporting infrastructure (IAM roles, DynamoDB tables for audit and rate limiting, ElastiCache for caching, CloudWatch for logging) and outputs values for use with the Helm chart. The ECS module provides an alternative deployment target using Fargate.

```bash
cd deployments/terraform/environments/<your-env>
terraform init
terraform plan -var-file=terraform.tfvars
terraform apply -var-file=terraform.tfvars
```

## FIPS 140-3 compliance

GATE supports optional FIPS 140-3 mode for deployment in FedRAMP and other regulated environments. Go 1.24 introduced a native Go Cryptographic Module that implements FIPS 140-3 approved algorithms. When FIPS mode is enabled, all cryptographic operations (TLS, SHA-256, RSA, ECDSA, random number generation) are performed by this module, with integrity self-checks and known-answer self-tests at startup.

FIPS 140-3 support requires two steps: a build-time flag to link the Go Cryptographic Module, and a runtime setting to activate FIPS mode.

### Building with FIPS support

Set the `GOFIPS140` variable when building. The value `latest` uses the cryptographic module from your Go toolchain. A specific version such as `v1.0.0` uses a frozen module suitable for CMVP validation.

```bash
# Build binary with FIPS support
make build GOFIPS140=latest

# Build container image with FIPS support
make image-build GOFIPS140=latest

# Build with a specific frozen module version (for CMVP validation)
make build GOFIPS140=v1.0.0
```

Without `GOFIPS140` (or with `GOFIPS140=off`), the binary is built without FIPS support and behaves identically to a standard Go build.

### Runtime activation

When built with FIPS support, FIPS mode is enabled by default. You can explicitly control it via the `GODEBUG` environment variable:

```bash
# Enable FIPS mode (default when built with GOFIPS140)
export GODEBUG=fips140=on

# Strict mode: non-FIPS algorithms return errors
export GODEBUG=fips140=only
```

GATE logs the FIPS 140-3 status at startup for auditability:

```json
{"level":"INFO","msg":"FIPS 140-3 mode","enabled":true}
```

### Verification

GATE exposes FIPS 140-3 status via the `/api/v1/info` endpoint, which is intended for compliance verification and monitoring:

```bash
curl http://localhost:8080/api/v1/info
# {"fips_enabled":true}
```

### Kubernetes deployment

The Helm chart supports configuring FIPS mode. See the [Helm chart documentation](deployments/helm/gate/README.md) for details.

### Considerations

When FIPS 140-3 mode is active, `crypto/tls` only negotiates FIPS-approved protocol versions, cipher suites, and signature algorithms. This is transparent for GATE's primary use case: GitHub Actions OIDC tokens use RS256 (RSA with SHA-256), which is FIPS-approved.

OIDC providers that use non-FIPS algorithms (such as EdDSA or secp256k1) will not work in FIPS mode. Configure your identity providers to use FIPS-approved algorithms: RSA (RS256, RS384, RS512), ECDSA with NIST curves (ES256, ES384, ES512), or RSA-PSS (PS256, PS384, PS512).

> [!WARNING]
> In `fips140=only` mode, cryptographic operations using non-FIPS algorithms return errors or panic. If any of your configured OIDC providers sign tokens with non-FIPS algorithms, token validation will fail at runtime. Verify that all providers use FIPS-approved JWS algorithms before enabling `only` mode. The `on` mode is recommended unless strict compliance requires rejecting all non-FIPS algorithms.

Key generation with pairwise consistency tests may be up to 2x slower in FIPS mode. This primarily affects ephemeral keys and is not significant for GATE's workload, which does not generate keys at runtime.

For details on the Go Cryptographic Module and CMVP validation status, see the [Go FIPS 140-3 documentation](https://go.dev/doc/security/fips140).

## Security considerations

GATE is designed with security as the primary concern. The architecture follows several principles intended to minimize risk even in the event of partial compromise.

All access is denied by default. A token request succeeds only when the OIDC token passes validation, the central policy permits the issuer and claims, a trust policy exists in the target repository, the trust policy's rules match the token's claims, and the requested permissions fall within all configured limits. Any failure at any layer results in denial.

Tokens are short-lived by design. The default lifetime is 15 minutes, and the maximum configurable lifetime is limited by the central policy. Short lifetimes bound the damage from token theft: an attacker who obtains a token has minutes, not months, to exploit it.

The principle of least privilege is enforced at multiple layers. Clients should request only the permissions they need. Trust policies should grant only the permissions the workflow requires. The central policy should cap permissions at the maximum any legitimate workflow might need. The effective permissions are always the most restrictive intersection of these layers.

Audit logging captures every request with sufficient detail for forensic analysis and compliance reporting. Logs include the requester's full OIDC claims, the target repository, requested and granted permissions, the policy that matched (or why none matched), and the outcome. Sensitive values such as tokens and private keys are never logged.

The service itself runs as a non-root user in a minimal container image with no shell or package manager, reducing the attack surface available to an attacker who achieves code execution.

A detailed threat model using the STRIDE methodology is available in [THREAT_MODEL.md](THREAT_MODEL.md). The threat model provides a comprehensive analysis of trust boundaries, potential attack vectors, and specific countermeasures for each threat category.

> [!WARNING]
> Several controls documented in the threat model are critical to the security of the system and must be implemented in production deployments. Consult the threat model to understand and implement these security-critical controls.

## Contributing

Contributions are welcome. Please open an issue to discuss proposed changes before submitting a pull request for significant modifications. Ensure that new code includes appropriate tests and that all existing tests pass.

## License

Copyright 2026 Thomson Reuters. Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license text.
