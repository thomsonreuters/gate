# GATE Helm chart

This Helm chart deploys GATE (GitHub Authenticated Token Exchange) to Kubernetes.

GATE is a Security Token Service that exchanges OIDC tokens for short-lived GitHub App installation tokens with fine-grained permissions.

The chart creates the following Kubernetes resources:

- **Deployment** with configurable replicas, security contexts, and health probes
- **Service** for internal cluster access
- **ServiceAccount** with optional annotations for cloud provider IAM integration
- **ConfigMaps** for application settings and the central authorization policy
- **Ingress** (optional) for external access with TLS support
- **HorizontalPodAutoscaler** (optional) for automatic scaling based on CPU/memory
- **PodDisruptionBudget** (optional) for high availability during cluster maintenance
- **NetworkPolicy** (optional) for restricting pod network traffic
- **SecretStore and ExternalSecret** (optional) for External Secrets Operator integration

## Prerequisites

The chart requires Kubernetes 1.23 or later and Helm 3.8 or later. Before installing, you must have one or more GitHub Apps installed on the target GitHub organization, with the client ID, organization name, and private key for each.

If you manage secrets manually, you must create the Kubernetes Secret containing the GitHub Apps credentials before installing the chart. If you use External Secrets Operator, the chart can create the necessary resources to provision the secret automatically.

Optional prerequisites depending on your configuration:

- **External Secrets Operator** if you want to pull GitHub Apps credentials from a secret management service
- **A network policy controller** (such as Calico or Cilium) if you enable NetworkPolicy
- **An ingress controller** (such as NGINX or AWS ALB) if you enable Ingress
- **Metrics Server** if you enable HorizontalPodAutoscaler

## Quick start

The fastest way to deploy GATE is to create the GitHub Apps secret manually and install with default settings.

First, create a JSON file describing your GitHub Apps. Each entry must include `client_id`, `organization`, and `private_key_path` pointing to the app's PEM file on disk:

```json
[
  {
    "client_id": "Iv1...",
    "organization": "your-org",
    "private_key_path": "/path/to/private-key.pem"
  }
]
```

Note that the JSON file uses snake_case field names (`client_id`, `private_key_path`) while Helm values use camelCase (`clientId`).

Use the helper script to create the Kubernetes secret. The script reads each `private_key_path`, validates the PEM files exist, and stores them as `app-0.pem`, `app-1.pem`, etc. in the secret:

```bash
./scripts/create-kubernetes-secret.sh \
  --file github-apps.json \
  --namespace gate \
  --secret-name github-apps
```

Alternatively, create the secret directly with `kubectl`:

```bash
kubectl create secret generic github-apps \
  --from-file=app-0.pem=/path/to/private-key.pem \
  -n gate
```

Install the chart, providing the app metadata that corresponds to each PEM file index:

```bash
helm install gate ./gate \
  --namespace gate \
  --create-namespace \
  --set config.oidc.audience="your-gate-instance" \
  --set config.githubApps.apps[0].clientId="Iv1..." \
  --set config.githubApps.apps[0].organization="your-org"
```

Verify the deployment:

```bash
kubectl rollout status deployment/gate -n gate
kubectl get pods -l app.kubernetes.io/name=gate -n gate
```

Test the health endpoint:

```bash
kubectl exec -n gate deploy/gate -- wget -qO- http://localhost:8080/health
```

## Configuration

All configuration is provided through Helm values. The chart includes a JSON schema (`values.schema.json`) that validates your configuration and provides IDE autocompletion.

### GitHub Apps credentials

GATE needs access to GitHub App credentials to generate installation tokens. The chart supports two strategies for providing these credentials.

**Kubernetes Secret (default)**

Create the secret manually before installing the chart. The secret must contain one PEM file per app, named `app-0.pem`, `app-1.pem`, etc., matching the index in the `config.githubApps.apps` array. The helper script automates this from a JSON file:

```bash
./scripts/create-kubernetes-secret.sh \
  --file github-apps.json \
  --namespace gate \
  --secret-name github-apps
```

Reference the secret in your values:

```yaml
config:
  githubApps:
    strategy: "kubernetesSecret"
    apps:
      - clientId: "Iv1..."
        organization: "your-org"
    kubernetesSecret:
      name: "github-apps"
```

**External Secrets Operator**

If you use External Secrets Operator to manage secrets, the chart can create an ExternalSecret that pulls credentials from your secret management service. See the [External Secrets Operator](#external-secrets-operator) section for detailed configuration.

### Central policy

The central authorization policy is defined directly in your Helm values under `config.policy`. The chart renders this as a ConfigMap and mounts it into the container.

```yaml
config:
  policy:
    version: "1.0"
    trustPolicyPath: ".github/gate/trust-policy.yaml"
    defaultTokenTTL: 900
    maxTokenTTL: 3600
    requireExplicitPolicy: false

    providers:
      - issuer: "https://token.actions.githubusercontent.com"
        name: "GitHub Actions"
        requiredClaims:
          repository_owner: "^your-org$"

    maxPermissions:
      contents: write
      issues: write
      pull_requests: write
      packages: write
      metadata: read
```

Changes to the policy trigger an automatic rollout because the Deployment includes a checksum annotation of the ConfigMap content.

### Audit backends

GATE logs every token request for compliance and debugging. Three audit backends are available.

**Console (default)**

Writes audit entries to stdout as structured JSON. Suitable for development or when logs are collected by a log aggregation system.

```yaml
config:
  audit:
    backend: ""
```

**PostgreSQL**

Writes audit entries to a PostgreSQL table. For production, provide the connection string via an existing secret rather than inline:

```yaml
config:
  audit:
    backend: "sql"
    sql:
      existingSecret:
        name: "postgres-credentials"
        key: "dsn"
```

**DynamoDB**

Writes audit entries to a DynamoDB table. Requires AWS credentials, typically provided via IAM Roles for Service Accounts (IRSA):

```yaml
config:
  awsRegion: "us-east-1"
  audit:
    backend: "dynamodb"
    dynamodb:
      tableName: "audit_logs"
      ttlDays: 90

serviceAccount:
  annotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::123456789012:role/gate-role"
```

### Selector stores

When GATE is configured with multiple GitHub Apps, the selector tracks rate limit state to distribute requests across apps with available capacity. Three storage backends are available.

**Memory (default)**

Stores state in memory. Suitable for single-replica deployments or development. State is lost when pods restart and is not shared across replicas.

```yaml
config:
  selector:
    type: "memory"
```

**Redis**

Stores state in Redis. Recommended for multi-replica deployments. Enable TLS for production connections to managed Redis services:

```yaml
config:
  selector:
    type: "redis"
    redis:
      address: "redis.gate.svc.cluster.local:6379"
      tls: true
      existingSecret:
        name: "redis-credentials"
        key: "password"
```

**DynamoDB**

Stores state in DynamoDB. Recommended for serverless deployments on AWS:

```yaml
config:
  awsRegion: "us-east-1"
  selector:
    type: "dynamodb"
    dynamodb:
      tableName: "gate-selector"
      ttlMinutes: 120
```

## Production deployment

For production environments, enable high availability features and tune resources appropriately.

### Replicas and autoscaling

Run at least two replicas and enable horizontal pod autoscaling:

```yaml
replicaCount: 2

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 80
  targetMemoryUtilizationPercentage: 80
```

### Pod disruption budget

Enable a PodDisruptionBudget to maintain availability during node drains and cluster upgrades:

```yaml
podDisruptionBudget:
  enabled: true
  minAvailable: 1
```

### Network policy

Enable NetworkPolicy to restrict traffic to and from GATE pods. The default policy allows ingress on port 8080 and egress to port 443 (for GitHub API and OIDC provider calls):

```yaml
networkPolicy:
  enabled: true
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              gate-client: "true"
      ports:
        - protocol: TCP
          port: 8080
  egress:
    - to: []
      ports:
        - protocol: TCP
          port: 443
```

### Ingress with TLS

Expose GATE externally with TLS termination:

```yaml
ingress:
  enabled: true
  className: "nginx"
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
  hosts:
    - host: gate.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: gate-tls
      hosts:
        - gate.example.com
```

### Direct TLS termination

Most deployments terminate TLS at the load balancer or ingress controller. For environments where the application must terminate TLS directly, configure the server TLS settings and mount the certificate and key into the pod:

```yaml
config:
  server:
    tls:
      certFilePath: "/certs/tls.crt"
      keyFilePath: "/certs/tls.key"

extraVolumes:
  - name: tls-certs
    secret:
      secretName: gate-tls

extraVolumeMounts:
  - name: tls-certs
    mountPath: /certs
    readOnly: true
```

### FIPS 140-3 compliance

For FedRAMP and regulated environments, GATE supports FIPS 140-3 mode. This requires the container image to be built with FIPS support (see the root project README for build instructions).

Enable FIPS mode in your values:

```yaml
config:
  fips:
    enabled: true
    mode: "on"
```

When enabled, the chart sets `GODEBUG=fips140=on` (or `fips140=only`) in the pod environment. This activates the Go Cryptographic Module's FIPS mode, which enforces FIPS-approved algorithms for all cryptographic operations including TLS, hashing, signing, and random number generation.

Two modes are available:

- `on` enables FIPS mode while keeping non-FIPS algorithms available. This is the recommended setting for most deployments.
- `only` additionally causes non-FIPS algorithms to return errors or panic. Use this for strict compliance, but note that OIDC providers using non-FIPS algorithms (such as EdDSA or secp256k1) will fail token validation. Ensure all configured OIDC providers use FIPS-approved algorithms (RS256, ES256, PS256, and their SHA-384/SHA-512 variants) before enabling this mode.

GATE exposes an `/api/v1/info` endpoint that reports the current FIPS 140-3 status. This is useful for compliance verification and monitoring:

```bash
kubectl exec -n gate deploy/gate -- wget -qO- http://localhost:8080/api/v1/info
# {"fips_enabled":true}
```

GATE also logs the FIPS status at startup, which can be verified in pod logs:

```bash
kubectl logs -l app.kubernetes.io/name=gate -n gate | grep "FIPS 140-3"
```

### Resource limits

Adjust CPU and memory based on your expected load. The defaults are conservative starting points:

```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

### Security contexts

The chart configures secure defaults that should not need modification. Pods run as a non-root user (UID 65532), use a read-only root filesystem, drop all Linux capabilities, and enable seccomp with the RuntimeDefault profile.

## External Secrets Operator

External Secrets Operator (ESO) synchronizes secrets from external secret management services into Kubernetes. The chart can create an ExternalSecret to pull GitHub Apps credentials automatically.

### AWS Secrets Manager

Store your GitHub Apps JSON in AWS Secrets Manager, then configure the chart to pull it. The pod must authenticate to AWS, typically via IAM Roles for Service Accounts (IRSA).

```yaml
config:
  githubApps:
    strategy: "externalSecret"
    externalSecret:
      enabled: true
      secretStoreName: "aws-secretsmanager"
      remoteSecretName: "gate/github-apps"
      refreshInterval: "1h"

externalSecrets:
  secretStore:
    create: true
    name: "aws-secretsmanager"
    provider:
      type: "aws"
      aws:
        service: "SecretsManager"
        region: "us-east-1"
```

### Azure Key Vault

Store your GitHub Apps JSON in Azure Key Vault, then configure the chart to pull it. The pod must authenticate to Azure, typically via Azure Workload Identity or AAD Pod Identity.

```yaml
config:
  githubApps:
    strategy: "externalSecret"
    externalSecret:
      enabled: true
      secretStoreName: "azure-keyvault"
      remoteSecretName: "gate-github-apps"
      refreshInterval: "1h"

externalSecrets:
  secretStore:
    create: true
    name: "azure-keyvault"
    provider:
      type: "azure"
      azure:
        vaultUrl: "https://your-vault.vault.azure.net"
```

### Google Cloud Secret Manager

Store your GitHub Apps JSON in Google Cloud Secret Manager, then configure the chart to pull it. The pod must authenticate to GCP, typically via GKE Workload Identity.

```yaml
config:
  githubApps:
    strategy: "externalSecret"
    externalSecret:
      enabled: true
      secretStoreName: "gcp-secretmanager"
      remoteSecretName: "gate-github-apps"
      refreshInterval: "1h"

externalSecrets:
  secretStore:
    create: true
    name: "gcp-secretmanager"
    provider:
      type: "gcp"
      gcp:
        projectID: "your-project-id"
```

### HashiCorp Vault

Store your GitHub Apps JSON in HashiCorp Vault, then configure the chart to pull it. Vault must have the Kubernetes auth method enabled and configured to allow the GATE service account.

```yaml
config:
  githubApps:
    strategy: "externalSecret"
    externalSecret:
      enabled: true
      secretStoreName: "vault"
      remoteSecretName: "gate/github-apps"
      refreshInterval: "1h"

externalSecrets:
  secretStore:
    create: true
    name: "vault"
    provider:
      type: "vault"
      vault:
        server: "https://vault.example.com"
        path: "secret"
        version: "v2"
        mountPath: "kubernetes"
        role: "gate"
```

### Using an existing SecretStore

If your platform team manages a shared SecretStore, reference it instead of creating one:

```yaml
config:
  githubApps:
    strategy: "externalSecret"
    externalSecret:
      enabled: true
      secretStoreName: "shared-secretstore"
      remoteSecretName: "gate/github-apps"
      refreshInterval: "1h"

externalSecrets:
  secretStore:
    create: false
```

## Parameters

### Global parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `nameOverride` | Override the chart name used in resource names | `""` |
| `fullnameOverride` | Override the full resource name | `""` |

### Image parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.registry` | Container registry | `""` |
| `image.repository` | Image repository | `gate` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `image.tag` | Image tag (defaults to Chart.appVersion) | `""` |
| `imagePullSecrets` | Pull secrets for private registries | `[]` |

### Service account parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceAccount.create` | Create a ServiceAccount | `true` |
| `serviceAccount.automount` | Automount the ServiceAccount token | `true` |
| `serviceAccount.annotations` | ServiceAccount annotations (e.g., for IRSA) | `{}` |
| `serviceAccount.name` | ServiceAccount name (auto-generated if empty) | `""` |

### Deployment parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas (ignored if autoscaling enabled) | `2` |
| `podAnnotations` | Annotations to add to pods | `{}` |
| `podLabels` | Labels to add to pods | `{}` |
| `podSecurityContext.runAsNonRoot` | Run containers as non-root | `true` |
| `podSecurityContext.runAsUser` | User ID for containers | `65532` |
| `podSecurityContext.runAsGroup` | Group ID for containers | `65532` |
| `podSecurityContext.fsGroup` | Filesystem group ID | `65532` |
| `podSecurityContext.seccompProfile.type` | Seccomp profile type | `RuntimeDefault` |
| `securityContext.allowPrivilegeEscalation` | Allow privilege escalation | `false` |
| `securityContext.capabilities.drop` | Linux capabilities to drop | `["ALL"]` |
| `securityContext.readOnlyRootFilesystem` | Read-only root filesystem | `true` |
| `securityContext.runAsNonRoot` | Run as non-root user | `true` |
| `securityContext.runAsUser` | Container user ID | `65532` |

### Resource parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.requests.memory` | Memory request | `128Mi` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `512Mi` |

### Autoscaling parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `autoscaling.enabled` | Enable HorizontalPodAutoscaler | `false` |
| `autoscaling.minReplicas` | Minimum replicas | `2` |
| `autoscaling.maxReplicas` | Maximum replicas | `10` |
| `autoscaling.targetCPUUtilizationPercentage` | Target CPU utilization | `80` |
| `autoscaling.targetMemoryUtilizationPercentage` | Target memory utilization | `80` |

### Pod disruption budget parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `podDisruptionBudget.enabled` | Enable PodDisruptionBudget | `false` |
| `podDisruptionBudget.minAvailable` | Minimum available pods | `1` |
| `podDisruptionBudget.maxUnavailable` | Maximum unavailable pods | `nil` |

### Scheduling parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `nodeSelector` | Node selector for pod scheduling | `{}` |
| `tolerations` | Tolerations for pod scheduling | `[]` |
| `affinity` | Affinity rules (includes pod anti-affinity by default) | See values.yaml |

### Service parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Service port | `8080` |
| `service.targetPort` | Container port | `8080` |
| `service.annotations` | Service annotations | `{}` |

### Ingress parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ingress.enabled` | Enable Ingress | `false` |
| `ingress.className` | Ingress class name | `""` |
| `ingress.annotations` | Ingress annotations | `{}` |
| `ingress.hosts` | Ingress hosts configuration | See values.yaml |
| `ingress.tls` | TLS configuration | `[]` |

### Network policy parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `networkPolicy.enabled` | Enable NetworkPolicy | `false` |
| `networkPolicy.policyTypes` | Policy types to enforce | `["Ingress", "Egress"]` |
| `networkPolicy.ingress` | Ingress rules | See values.yaml |
| `networkPolicy.egress` | Egress rules | See values.yaml |

### Health probe parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `livenessProbe.httpGet.path` | Liveness probe path | `/health` |
| `livenessProbe.httpGet.port` | Liveness probe port | `http` |
| `livenessProbe.initialDelaySeconds` | Initial delay before probing | `10` |
| `livenessProbe.periodSeconds` | Probe interval | `10` |
| `livenessProbe.timeoutSeconds` | Probe timeout | `5` |
| `livenessProbe.failureThreshold` | Failures before marking unhealthy | `3` |
| `readinessProbe.httpGet.path` | Readiness probe path | `/health` |
| `readinessProbe.httpGet.port` | Readiness probe port | `http` |
| `readinessProbe.initialDelaySeconds` | Initial delay before probing | `5` |
| `readinessProbe.periodSeconds` | Probe interval | `5` |
| `readinessProbe.timeoutSeconds` | Probe timeout | `3` |
| `readinessProbe.failureThreshold` | Failures before marking not ready | `3` |

### Application configuration parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.logLevel` | Log level (debug, info, warn, error) | `info` |
| `config.logFormat` | Log format (json, text) | `json` |
| `config.server.port` | HTTP server port | `8080` |
| `config.server.readTimeout` | HTTP read timeout | `30s` |
| `config.server.writeTimeout` | HTTP write timeout | `30s` |
| `config.server.shutdownTimeout` | Graceful shutdown timeout | `10s` |
| `config.server.requestTimeout` | Per-request timeout | `30s` |
| `config.server.idleTimeout` | Idle connection timeout | `10s` |
| `config.server.waitTimeout` | Wait timeout for graceful connection draining | `10s` |
| `config.server.tls.certFilePath` | Path to TLS certificate file (for direct TLS termination) | `""` |
| `config.server.tls.keyFilePath` | Path to TLS private key file (for direct TLS termination) | `""` |
| `config.oidc.audience` | Expected OIDC audience claim (required) | `gate` |
| `config.awsRegion` | AWS region for DynamoDB backends | `us-east-1` |

### FIPS 140-3 parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.fips.enabled` | Enable FIPS 140-3 mode (requires FIPS-compiled image) | `false` |
| `config.fips.mode` | FIPS mode: `on` or `only` | `on` |

### GitHub Apps parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.githubApps.strategy` | Secret strategy: kubernetesSecret or externalSecret | `kubernetesSecret` |
| `config.githubApps.apps` | GitHub App metadata array (`clientId`, `organization` per entry) | See values.yaml |
| `config.githubApps.kubernetesSecret.name` | Kubernetes Secret name (must contain `app-N.pem` files) | `github-apps` |
| `config.githubApps.externalSecret.enabled` | Enable ExternalSecret | `false` |
| `config.githubApps.externalSecret.secretStoreName` | SecretStore to reference | `aws-secretsmanager` |
| `config.githubApps.externalSecret.remoteSecretName` | Remote secret name | `""` |
| `config.githubApps.externalSecret.refreshInterval` | Secret refresh interval | `1h` |

### Audit backend parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.audit.backend` | Audit backend: `""` (console), `sql` (PostgreSQL), `dynamodb` | `""` |
| `config.audit.dynamodb.tableName` | DynamoDB table name | `""` |
| `config.audit.dynamodb.ttlDays` | TTL in days (0 to disable, 1-365) | `90` |
| `config.audit.sql.dsn` | PostgreSQL connection string (prefer existingSecret) | `""` |
| `config.audit.sql.existingSecret.name` | Secret containing PostgreSQL DSN | `""` |
| `config.audit.sql.existingSecret.key` | Key within the Secret | `dsn` |

### Selector store parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.selector.type` | Selector store: memory, redis, dynamodb | `memory` |
| `config.selector.redis.address` | Redis address (host:port) | `""` |
| `config.selector.redis.password` | Redis password (prefer existingSecret) | `""` |
| `config.selector.redis.db` | Redis database number (0-15) | `0` |
| `config.selector.redis.tls` | Enable TLS for Redis | `false` |
| `config.selector.redis.existingSecret.name` | Secret containing Redis password | `""` |
| `config.selector.redis.existingSecret.key` | Key within the Secret | `password` |
| `config.selector.dynamodb.tableName` | DynamoDB table name | `""` |
| `config.selector.dynamodb.ttlMinutes` | TTL in minutes (0 to disable, recommended: 60-180) | `120` |

### Policy parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.policy.version` | Policy schema version | `1.0` |
| `config.policy.trustPolicyPath` | Path to trust policy in repositories | `.github/gate/trust-policy.yaml` |
| `config.policy.defaultTokenTTL` | Default token TTL in seconds | `900` |
| `config.policy.maxTokenTTL` | Maximum token TTL in seconds | `3600` |
| `config.policy.requireExplicitPolicy` | Require policy_name in requests | `false` |
| `config.policy.githubAPIBaseURL` | GitHub API URL (for GitHub Enterprise) | `""` |
| `config.policy.githubRawBaseURL` | GitHub raw content URL (for GitHub Enterprise) | `""` |
| `config.policy.providers` | Trusted OIDC providers (required) | See values.yaml |
| `config.policy.providers[].issuer` | OIDC issuer URL | `""` |
| `config.policy.providers[].name` | Human-readable provider name | `""` |
| `config.policy.providers[].requiredClaims` | Claims that must be present and match patterns (claim name to regex) | `{}` |
| `config.policy.providers[].forbiddenClaims` | Claims that must NOT match patterns (claim name to regex) | `{}` |
| `config.policy.providers[].timeRestrictions.allowedDays` | Days of the week when access is permitted | `[]` |
| `config.policy.providers[].timeRestrictions.allowedHours.start` | Start hour (0-23) of the allowed window | `nil` |
| `config.policy.providers[].timeRestrictions.allowedHours.end` | End hour (0-23) of the allowed window | `nil` |
| `config.policy.maxPermissions` | Organization-wide maximum permissions | See values.yaml |

### Origin verification parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.originVerification.enabled` | Enable origin verification (shared secret header) | `false` |
| `config.originVerification.headerName` | Header name that the proxy/CDN sends | `X-Origin-Verify` |
| `config.originVerification.strategy` | Secret strategy: kubernetesSecret or externalSecret | `kubernetesSecret` |
| `config.originVerification.kubernetesSecret.name` | Kubernetes Secret name | `cloudfront-origin-verify` |
| `config.originVerification.kubernetesSecret.key` | Key within the Secret | `value` |
| `config.originVerification.externalSecret.secretStoreName` | SecretStore to reference for origin verification secret | `aws-secretsmanager` |
| `config.originVerification.externalSecret.remoteSecretName` | Remote secret name for origin verification | `""` |
| `config.originVerification.externalSecret.refreshInterval` | Secret refresh interval for origin verification | `1h` |

### External Secrets Operator parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `externalSecrets.secretStore.create` | Create a SecretStore | `false` |
| `externalSecrets.secretStore.name` | SecretStore name | `""` |
| `externalSecrets.secretStore.annotations` | SecretStore annotations | `{}` |
| `externalSecrets.secretStore.provider.type` | Provider: aws, gcp, azure, vault | `aws` |
| `externalSecrets.secretStore.provider.aws.service` | AWS service name | `SecretsManager` |
| `externalSecrets.secretStore.provider.aws.region` | AWS region | `us-east-1` |
| `externalSecrets.secretStore.provider.gcp.projectID` | GCP project ID | `""` |
| `externalSecrets.secretStore.provider.azure.vaultUrl` | Azure Key Vault URL | `""` |
| `externalSecrets.secretStore.provider.vault.server` | Vault server URL | `""` |
| `externalSecrets.secretStore.provider.vault.path` | Vault secret engine path | `secret` |
| `externalSecrets.secretStore.provider.vault.version` | Vault KV engine version | `v2` |
| `externalSecrets.secretStore.provider.vault.mountPath` | Vault Kubernetes auth mount path | `kubernetes` |
| `externalSecrets.secretStore.provider.vault.role` | Vault Kubernetes auth role | `""` |

### Extension parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `extraEnv` | Extra environment variables | `[]` |
| `extraEnvFrom` | Extra environment variable sources | `[]` |
| `extraVolumes` | Extra volumes | `[]` |
| `extraVolumeMounts` | Extra volume mounts | `[]` |
| `initContainers` | Init containers | `[]` |
| `sidecars` | Sidecar containers | `[]` |

## Upgrading

To upgrade an existing release:

```bash
helm upgrade gate ./gate \
  --namespace gate \
  --values your-values.yaml
```

The Deployment includes checksum annotations for both ConfigMaps. When you change the application configuration or policy, Helm detects the checksum change and triggers a rolling update automatically.

If you are upgrading across major chart versions, review the changelog for breaking changes that may require value migration.

## Uninstalling

To remove the release and all associated resources:

```bash
helm uninstall gate --namespace gate
```

This removes all Kubernetes resources created by the chart. It does not remove:

- The namespace (if you created it separately or with `--create-namespace`)
- Persistent data in external systems (DynamoDB tables, PostgreSQL databases, Redis)
- Secrets created manually (such as the GitHub Apps secret)

To remove the namespace as well:

```bash
kubectl delete namespace gate
```
