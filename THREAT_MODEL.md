# GATE: Threat model

This document provides a structured threat analysis of GATE (GitHub Authenticated Token Exchange) using the STRIDE methodology. It identifies trust boundaries, assets, threat scenarios, and actionable recommendations for operators and contributors.

## Table of contents

- [1. System overview](#1-system-overview)
- [2. Trust boundaries](#2-trust-boundaries)
- [3. Assets](#3-assets)
- [4. STRIDE threat analysis](#4-stride-threat-analysis)
- [5. Critical threat scenarios](#5-critical-threat-scenarios)
- [6. Deployment security](#6-deployment-security)

## 1. System overview

GATE is a Security Token Service (STS) that exchanges OIDC tokens from trusted identity providers for short-lived GitHub App installation tokens. It serves as a security-critical gateway: any weakness translates directly into unauthorized access to GitHub repositories.

```
                                   ┌──────────────────────────────────────────────────────────┐
┌──────────────┐    OIDC Token     │                     GATE                                 │
│   Client     │──────────────────>│                                                          │
│  (Workflow)  │<──────────────────│  ┌─────────┐  ┌──────────┐  ┌────────────┐               │
│              │   GitHub Token    │  │  API    │─>│ Service  │─>│ OIDC       │               │
└──────────────┘                   │  │  Layer  │  │ Exchange │  │ Validator  │               │
                                   │  └─────────┘  │          │  └─────┬──────┘               │
                                   │               │          │        │                      │
                                   │               │          │  ┌─────v──────┐               │
                                   │               │          │  │ JWKS Cache │───────────────┼──> OIDC Provider
                                   │               │          │  └────────────┘               │
                                   │               │          │                               │
                                   │               │          │──> AuthZ Engine               │
                                   │               │          │    ├─ Layer 1: Central Policy |
                                   │               │          │    └─ Layer 2: Repo Policy ───┼──> GitHub API
                                   │               │          │                               │     (contents)
                                   │               │          │──> App Selector               │
                                   │               │          │──> GitHub Client──────────────┼──> GitHub API
                                   │               │          │──> Audit Backend──────────────┼──> DB/Console
                                   │               └──────────┘                               │
                                   └──────────────────────────────────────────────────────────┘
```

## 2. Trust boundaries

| ID | Boundary | Description |
|----|----------|-------------|
| TB1 | Client to GATE API | External untrusted callers presenting OIDC tokens |
| TB2 | GATE to OIDC Provider | Outbound JWKS/discovery fetching (trusts the provider's TLS certificate and key material) |
| TB3 | GATE to GitHub API | Outbound calls for policy fetching and token generation (trusts GitHub API responses) |
| TB4 | GATE to Audit Backend | Outbound writes to postgres/dynamodb |
| TB5 | GATE to Selector Store | Outbound reads/writes to redis/dynamodb for rate limit state |
| TB6 | Repository Owners to Trust Policy Files | Developers authoring trust policy files (default `.github/gate/trust-policy.yaml`, configurable via `trust_policy_path` with `{org}` substitution) in their repos |
| TB7 | Deployment Environment to GATE | Secrets injection (GitHub App private keys, origin verify secret, DB credentials) |
| TB8 | Central Config Authors to GATE | Whoever manages the central policy YAML defines the security ceiling |

## 3. Assets

| Asset | Sensitivity | Location |
|-------|-------------|----------|
| GitHub App private keys | CRITICAL<br><br>Allows generating any token the App can issue. | Environment variables, Kubernetes secrets, or external secret stores |
| Issued GitHub installation tokens | HIGH<br><br>Grants repository access. | In-memory during exchange, returned to caller |
| Central policy configuration | HIGH<br><br>Defines the security ceiling. | Loaded at startup from YAML config file or environment variables |
| Trust policy files | HIGH<br><br>Defines per-repo access rules. | Stored in target repositories on GitHub |
| OIDC tokens | HIGH<br><br>Represent caller identity. | In-flight only, received in requests |
| Audit logs | MEDIUM<br><br>Forensic evidence. | DynamoDB, SQL, or console (stdout) |
| JWKS key material | MEDIUM<br><br>Cached public keys for signature verification. | In-memory provider cache (managed by `coreos/go-oidc`, re-fetches on unknown key IDs) |
| Origin verification secret | MEDIUM<br><br>Prevents direct-to-origin bypass. | Environment variable / Kubernetes secret |

## 4. STRIDE threat analysis

### 4.1 Spoofing

| ID | Threat | Severity | Mitigations in place | Residual risk |
|----|--------|----------|----------------------|---------------|
| S1 | **Forged OIDC tokens**<br><br>Attacker crafts a JWT to impersonate a legitimate workflow. | CRITICAL | - JWKS signature verification<br><br>- Issuer allowlist checked before JWKS fetch<br><br>- Audience validation<br><br>- Time claims validation | LOW<br><br>Requires compromising the OIDC provider's signing keys. |
| S2 | **SSRF via crafted issuer URL**<br><br>Attacker supplies a token with a malicious issuer to force GATE to fetch from internal endpoints. | HIGH | - Issuer allowlist checked before any network request (exact string match)<br><br>- Only pre-configured issuer URLs trigger OIDC discovery | LOW<br><br>Allowlist is the sole defense. No additional URL sanitization is performed — issuer URLs are trusted as configured by the operator. Misconfigured allowlists (e.g. including internal URLs) would not be caught. |
| S3 | **OIDC provider impersonation**<br><br>Attacker stands up a lookalike OIDC provider. | HIGH | - Issuer allowlist restricts to configured providers<br><br>- HTTPS prevents MitM during discovery | LOW<br><br>Depends on allowlist being correctly configured. |
| S4 | **Token replay**<br><br>Stolen OIDC token reused before expiry. | MEDIUM | - Short token lifetimes from OIDC providers (typically 5-10 min)<br><br>- Validation of: `exp`, `nbf`, `iat` | MEDIUM<br><br>Tokens valid until expiry, no replay detection (nonce/jti). |

### 4.2 Tampering

| ID | Threat | Severity | Mitigations in place | Residual risk |
|----|--------|----------|----------------------|---------------|
| T1 | **Trust policy tampering by developers**<br><br>Developer modifies trust policy to grant elevated permissions, including potential circular escalation via `administration: write` to disable branch protections. | CRITICAL | - `max_permissions` ceiling in central config<br><br>- org/user/enterprise scopes are blocked by design.<br><br>- Policy are validated.<br><br>- `administration: none` can be set in `max_permissions` to prevent circular escalation | HIGH<br><br>See section 5.1. |
| T3 | **Central config tampering**<br><br>Attacker with access to config source relaxes `max_permissions` or adds a rogue OIDC provider. | CRITICAL | - Operational controls (access to config source should be tightly restricted) | MEDIUM<br><br>Depends on deployment security. |
| T4 | **GitHub App private key theft**<br><br>Attacker extracts private key to generate tokens independently. | CRITICAL | - Keys in Kubernetes secrets or external secret stores<br><br>- Distroless container with no shell | MEDIUM<br><br>Depends on secret management hygiene. |
| T5 | **Audit log tampering**<br><br>Attacker deletes or modifies audit entries to cover tracks. | HIGH | - Separate DB credentials for audit<br><br>- DynamoDB with IAM-scoped writes | MEDIUM<br><br>No write-once enforcement in current design. |
| T6 | **Policy cache poisoning**<br><br>Stale or incorrect policy served from cache. | MEDIUM | - In-process memory cache (not externally accessible)<br><br>- 5-minute TTL | LOW<br><br>Requires code-level compromise. |

### 4.3 Repudiation

| ID | Threat | Severity | Mitigations in place | Residual risk |
|----|--------|----------|----------------------|---------------|
| R1 | **Lost audit entries**<br><br>Denied-request audit is best-effort: if the backend is slow or the process crashes, entries can be lost. Granted-request audit is blocking: failures prevent token issuance. | MEDIUM | - Granted audit is synchronous and blocking — failure returns an error to the caller, ensuring no token is issued without a record<br><br>- Denied audit logs a `slog.Warn` on failure and continues | LOW<br><br>Denied requests could have no audit record if the backend fails. Granted requests always have a record or the token is not issued. |
| R2 | **Untraceable policy changes**<br><br>GATE logs which policy matched but not who authored the policy or when it was last changed. | MEDIUM | - Git history provides authorship, but GATE audit logs don't capture policy version/commit | MEDIUM<br><br>Forensic reconstruction requires cross-referencing git history. |

### 4.4 Information Disclosure

| ID | Threat | Severity | Mitigations in place | Residual risk |
|----|--------|----------|----------------------|---------------|
| I1 | **Claim values in error responses**<br><br>Authorization denials include specific claim values and expected patterns. | MEDIUM | - Detailed errors are useful for debugging | MEDIUM<br><br>Reveals internal policy regex patterns to callers. |
| I2 | **Tokens in transit**<br><br>GATE listens on plain HTTP (port 8080 by default).<br>TLS must be terminated externally. | HIGH | - Helm chart supports ingress with TLS<br><br>- HSTS header when TLS detected | MEDIUM<br><br>Misconfigured deployment could expose tokens. |
| I3 | **Private keys in environment variables**<br><br>`GATE_GITHUB_APPS` env var contains PEM-encoded private keys visible in `/proc/*/environ`. | HIGH | - External Secrets Operator integration<br><br>- Kubernetes secret references | MEDIUM<br><br>Depends on operational practices. |
| I4 | **GitHub token in response body**<br><br>The actual `ghs_*` token is returned in JSON response. | MEDIUM | - HTTPS expected in production<br><br>- SHA256 hash logged instead of raw token | LOW<br><br>This is by design (the token is the service output). |
| I5 | **OIDC claims in audit logs**<br><br>All claims (including potentially sensitive custom claims) are persisted in audit entries. | LOW | - Claims are needed for forensic analysis | LOW<br><br>Acceptable risk for audit purposes. |

### 4.5 Denial of Service

| ID | Threat | Severity | Mitigations in place | Residual risk |
|----|--------|----------|----------------------|---------------|
| D1 | **GitHub API rate limit exhaustion**<br><br>Attacker sends many valid requests to drain all GitHub App rate limits. | HIGH | - Multi-app selector with rate tracking<br><br>- `RetryAfterSeconds` in response | MEDIUM<br><br>Determined attacker with valid OIDC tokens could still exhaust limits. |
| D2 | **JWKS cache eviction flood**<br><br>Attacker sends tokens from many different issuers to evict cached JWKS. | MEDIUM | - Issuer allowlist limits to configured providers<br><br>- Cache entries bounded by the number of configured providers (typically 1–5) | LOW<br><br>Allowlist is the primary defense; only pre-approved issuers trigger discovery and caching. |
| D3 | **Large request body**<br><br>Oversized POST body consumes memory. | MEDIUM | - `http.MaxBytesReader` limits request body size | LOW |
| D4 | **Policy fetch amplification**<br><br>Requests targeting many distinct repositories force GitHub API calls. | MEDIUM | - 500-entry policy cache<br><br>- 5-minute TTL<br><br>- Individual file size capped at ~1 MB by GitHub's Contents API | MEDIUM<br><br>Cache helps, but cold-start requests are expensive. |
| D5 | **Slow audit backend blocking**<br><br>Slow audit backend could block token issuance. | MEDIUM | - Denied audit is non-blocking (logs warning and continues)<br><br>- Granted audit is synchronous — a slow backend delays the response but ensures the audit record exists before the token is returned | MEDIUM<br><br>Granted requests are blocked by a slow audit backend. Denied requests continue normally. Backend latency directly impacts grant response times. |

### 4.6 Elevation of Privilege

| ID | Threat | Severity | Mitigations in place | Residual risk |
|----|--------|----------|----------------------|---------------|
| E1 | **Trust policy privilege escalation**<br><br>Developer grants themselves permissions up to `max_permissions` ceiling by modifying their repo's trust policy, with circular escalation risk if `administration` is not blocked. | CRITICAL | - `max_permissions` in central config<br><br>- org/user/enterprise scopes are blocked by design<br><br>- `administration: none` can be set to prevent self-reinforcing escalation | HIGH<br><br>See section 5.1. |
| E2 | **Collusion between contributors**<br><br>Two contributors cooperate to bypass code review on trust policy changes. | HIGH | - Branch rulesets with multi-reviewer requirements<br><br>- CODEOWNERS<br><br>- Push rulesets (GitHub Team plan, private repositories) | HIGH<br><br>See section 5.2. |
| E3 | **Cross-organization token issuance**<br><br>If `required_claims` doesn't restrict `repository_owner`, tokens can be issued for repos in other organizations where the GitHub App is installed. | HIGH | - `required_claims.repository_owner` pattern in central config | MEDIUM<br><br>Depends on correct configuration. |

## 5. Critical threat scenarios

### 5.1 Trust policy privilege escalation (T1, E1)

**Scenario:**

A contributor with write access to a repository modifies the trust policy file (default `.github/gate/trust-policy.yaml`) to add or expand a trust policy that grants elevated permissions (for example `contents: write`, `pull_requests: write`, or `administration: write` if not blocked).

**Attack path:**

1. Contributor pushes a commit to the trust policy file (directly or via unreviewed PR).
2. After the 5-minute policy cache expires, GATE fetches the modified policy.
3. The contributor's next workflow run matches the permissive policy.
4. GATE issues a token with the elevated permissions.

**Impact:**

The contributor gains GitHub API access beyond what was intended for their workflow. Impact severity increases dramatically if the `administration` permission is not blocked in `max_permissions`: an attacker can use an admin token to disable branch rulesets on the repository, creating a self-reinforcing escalation loop. With branch protections removed, they can compromise the repository's security posture in a way that may be difficult to detect or reverse quickly.

**Severity:**

Critical. Undermines the entire authorization model.

**Required mitigations (by repository type):**

For **private repositories** (with GitHub Team plan):

- **Push rulesets** should be configured to restrict direct pushes to the trust policy path. Push rulesets are the strongest protection available because they are enforced server-side by GitHub and cannot be overridden by repository administrators if the ruleset is owned at the organization level.

- **CODEOWNERS** designating a security team or repository administrators as required reviewers for trust policy files.

For **public repositories**:

- The primary defense is **strict write access control**: only trusted individuals should have write access to the repository.

- **Branch protection rules** requiring pull request reviews before merging should be enabled for branches containing trust policy files. This ensures changes cannot be pushed directly without review.

- **CODEOWNERS** designating a security team or repository administrators as required reviewers for trust policy files provides mandatory cross-team review. Combined with branch protection requiring reviews, this significantly raises the bar for unauthorized policy changes.

For **all repositories**:

- **`max_permissions`** in central config must be restrictive, listing only the permissions actually needed across the organization. Since `max_permissions` operates as an allowlist (unlisted permissions are denied), this is the hardcoded security ceiling regardless of what trust policies request.

- **`administration`** should not be listed in `max_permissions` (denied by default as an unlisted permission), or explicitly set to `none`, to prevent circular escalation where an attacker uses admin tokens to disable branch protections and further escalate privileges.

### 5.2 Collusion of contributors (E2)

**Scenario:**

Even with branch rulesets requiring reviews, two colluding contributors can escalate privileges.

**Attack path:**

1. Contributor A creates a PR adding a permissive trust policy.
2. Contributor B approves the PR (perhaps without proper scrutiny).
3. The expanded policy takes effect after merge and cache expiry.
4. Contributors A or B can now request elevated tokens.

**Impact:**

Same as section 5.1 (including the circular escalation risk if `administration` is not blocked).

**Severity:**

High.

**Mitigations and residual risk:**

This threat represents a residual organizational risk that is largely outside GATE's technical control. Organizations that do not trust their developers with repository-level access decisions should not use GATE's delegated trust policy model, or should implement additional organizational controls:

- **CODEOWNERS** requiring security team approval or repository admins (not just peer developers) provides a mandatory cross-team review.
- **Minimum 2+ reviewers** raises the collusion threshold but does not eliminate it.
- **Audit monitoring** for trust policy changes that expand permissions enables detective controls.
- **Required status checks** with automated policy analysis can catch suspicious patterns.

However, if developers are not trusted to manage access to their own repositories within the guardrails established by `max_permissions`, the fundamental assumption of the two-layer authorization model may not hold for that organization. In such cases, consider centralizing all trust policy management or implementing manual approval workflows for policy changes.


## 6. Deployment security

| Aspect | Status | Notes |
|--------|--------|-------|
| Container image | STRONG | - Distroless base image<br><br>- Non-root user (65532)<br><br>- Static binary<br><br>- Base images pinned by SHA256 digest |
| Pod security | STRONG | - `runAsNonRoot`<br><br>- `readOnlyRootFilesystem`<br><br>- `drop ALL` capabilities<br><br>- `seccompProfile: RuntimeDefault` |
| TLS | EXTERNAL | - GATE itself is HTTP-only<br><br>- TLS must be terminated at ingress/ALB<br><br>- Misconfiguration is a risk |
| Origin verification | OPTIONAL | - Shared secret header to verify requests come through trusted proxy/CDN<br><br>- Disabled by default |
| Network policy | OPTIONAL | - When enabled, allows only ingress on 8080 and egress on 443<br><br>- Disabled by default |
| Secrets management | FLEXIBLE | - Supports Kubernetes secrets and External Secrets Operator<br><br>- Plain environment variables are a risk |
| Health endpoint | UNAUTHENTICATED | - Excluded from origin verification to allow Kubernetes probes<br><br>- Could be used for probing |
