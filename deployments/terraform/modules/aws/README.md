# GATE Terraform modules for AWS

This directory contains Terraform modules for deploying GATE (GitHub Authenticated Token Exchange) on AWS. Two deployment models are supported: Amazon EKS with Helm for Kubernetes-native deployments, and Amazon ECS with Fargate for serverless container deployments. A standalone ECR module is provided for container image registry management.

## Module overview

### EKS module

The EKS module provisions the AWS infrastructure required to run GATE on an existing Amazon EKS cluster. It creates IAM roles, secrets, storage backends, and monitoring resources. The actual application deployment is handled separately using the Helm chart.

This module is appropriate when you already have an EKS cluster and want to deploy GATE alongside other Kubernetes workloads, or when you need the flexibility of Kubernetes for scheduling, scaling, and service mesh integration.

**Resources created:**

- IAM role with IRSA (IAM Roles for Service Accounts) trust policy
- IAM policies with least-privilege permissions for all enabled backends
- Secrets Manager secret for GitHub Apps credentials
- CloudWatch log group for application logs
- DynamoDB table for audit logs (optional)
- DynamoDB table for selector rate limit state (optional)
- ElastiCache Redis replication group for selector state (optional)
- CloudWatch alarms for DynamoDB throttling and errors (optional)
- KMS key for Secrets Manager encryption (optional)

### ECS module

The ECS module provisions a complete deployment on Amazon ECS using Fargate. It creates the ECS cluster (or uses an existing one), task definitions, services, load balancers, and all supporting infrastructure. No additional deployment tooling is required.

This module is appropriate when you want a serverless container deployment without managing Kubernetes infrastructure, or when ECS Fargate aligns better with your organization's operational model.

**Resources created:**

- ECS cluster (optional, can use existing)
- ECS task definition with Fargate launch type
- ECS service with rolling deployment configuration
- Application Load Balancer with health checks (optional)
- IAM task role and task execution role
- Security groups for network isolation
- Secrets Manager secret for GitHub Apps credentials
- CloudWatch log group for application logs
- DynamoDB tables for audit and selector (optional)
- ElastiCache Redis for selector state (optional)
- Auto scaling policies for the ECS service (optional)
- KMS key for Secrets Manager encryption (optional)

### ECR module

The ECR module provisions an Amazon Elastic Container Registry repository for storing GATE container images. It is intentionally standalone and decoupled from the EKS and ECS deployment modules, since container registry management is an independent concern from application deployment.

This module is appropriate when your organization hosts its own container images on AWS ECR and needs a consistent, governed repository with image scanning, encryption, lifecycle policies, and cross-account access controls.

**Resources created:**

- ECR repository with configurable image scanning and tag mutability
- Repository policy for cross-account or organizational access (optional)
- Lifecycle policy for automatic image cleanup (optional)

### ECR module

The ECR module provisions an Amazon Elastic Container Registry repository for storing GATE container images. It is intentionally standalone and decoupled from the EKS and ECS deployment modules, since container registry management is an independent concern from application deployment.

This module is appropriate when your organization hosts its own container images on AWS ECR and needs a consistent, governed repository with image scanning, encryption, lifecycle policies, and cross-account access controls.

**Resources created:**

- ECR repository with configurable image scanning and tag mutability
- Repository policy for cross-account or organizational access (optional)
- Lifecycle policy for automatic image cleanup (optional)

### CloudFront module

The CloudFront module provisions an AWS CloudFront distribution with optional WAF protection and Route53 DNS records. It is designed to work with any HTTP/HTTPS origin, including ALB endpoints from the ECS module or Kubernetes Ingress endpoints from EKS deployments.

This module is appropriate when you need a global CDN layer for caching, DDoS protection, or custom domain support in front of your GATE deployment, regardless of whether the backend runs on EKS or ECS.

**Resources created:**

- CloudFront distribution with configurable caching and security settings
- Origin configuration for ALB or custom endpoints
- WAF Web ACL with AWS Managed Rules (optional)
- WAF rate limiting rule (optional)
- WAF IP allowlist and blocklist (optional)
- Route53 alias records for custom domains (optional)

## Prerequisites

Both modules require Terraform 1.6 or later and the AWS provider version 6.0 or later. You must have AWS credentials configured with permissions to create the resources listed above.

**EKS module prerequisites:**

- An existing EKS cluster with the OIDC provider enabled
- External Secrets Operator installed in the cluster (for syncing GitHub Apps credentials from Secrets Manager)
- VPC and subnet information if enabling ElastiCache

**ECS module prerequisites:**

- A VPC with private subnets for the ECS tasks
- Public subnets if using an internet-facing Application Load Balancer
- An ACM certificate if terminating HTTPS on the load balancer
- A container image available in ECR or another accessible registry

**CloudFront module prerequisites:**

- An origin endpoint (ALB DNS name from ECS module, Ingress endpoint from EKS, or custom domain)
- An ACM certificate in us-east-1 if using custom domain aliases (CloudFront requirement)
- A Route53 hosted zone if creating DNS records

## Directory structure

The Terraform configuration is organized as follows:

```
deployments/terraform/
├── modules/
│   └── aws/
│       ├── cloudfront/   # Reusable module for CloudFront CDN
│       ├── ecr/          # Reusable module for ECR container registry
│       ├── ecs/          # Reusable module for ECS deployments
│       └── eks/          # Reusable module for EKS deployments
├── environments/         # Deployment configurations
│   └── prod-eks/         # Example: production EKS deployment
└── scripts/
    └── aws/              # Helper scripts for AWS operations
```

The `modules` directory contains reusable Terraform modules. The `environments` directory is where you create configurations for actual deployments by instantiating these modules with environment-specific values. You can host your deployment configurations elsewhere if preferred.

## Quick start

### EKS deployment

Create a Terraform configuration that calls the EKS module. The provider is configured in the root module and passed implicitly to child modules:

```hcl
terraform {
  required_version = ">= 1.6"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }

  backend "s3" {}
}

provider "aws" {
  region = "us-east-1"
}

module "gate" {
  source = "path/to/deployments/terraform/modules/aws/eks"

  aws_region       = "us-east-1"
  eks_cluster_name = "my-cluster"

  kubernetes_namespace            = "gate"
  kubernetes_service_account_name = "gate"

  # Enable DynamoDB for audit logs
  audit_dynamodb_enabled = true

  tags = {
    Environment = "production"
  }
}

output "iam_role_arn" {
  value = module.gate.iam_role_arn
}

output "github_apps_secret_name" {
  value = module.gate.github_apps_secret_name
}
```

Create a backend configuration file (e.g., `prod.s3.tfbackend`):

```hcl
bucket         = "my-org-terraform-states"
key            = "gate/production.tfstate"
region         = "us-east-1"
dynamodb_table = "my-org-terraform-locks"
encrypt        = true
```

Initialize and apply:

```bash
terraform init -backend-config=prod.s3.tfbackend
terraform apply
```

After Terraform completes, populate the GitHub Apps secret and deploy with Helm:

```bash
# Update the secret with your GitHub Apps credentials
./scripts/aws/push-github-apps-to-aws-secrets-manager.sh \
  --file github-apps.json \
  --secret-name "$(terraform output -raw github_apps_secret_name)"

# Deploy with Helm, referencing the IAM role and secret
helm install gate ../../helm/gate \
  --namespace gate \
  --create-namespace \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="$(terraform output -raw iam_role_arn)" \
  --set config.githubApps.strategy=externalSecret \
  --set config.githubApps.externalSecret.enabled=true \
  --set config.githubApps.externalSecret.remoteSecretName="$(terraform output -raw github_apps_secret_name)"
```

### ECS deployment

Create a Terraform configuration that calls the ECS module:

```hcl
terraform {
  required_version = ">= 1.6"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }

  backend "s3" {}
}

provider "aws" {
  region = "us-east-1"
}

module "gate" {
  source = "path/to/deployments/terraform/modules/aws/ecs"

  aws_region = "us-east-1"

  vpc_id             = "vpc-0123456789abcdef0"
  private_subnet_ids = ["subnet-aaa", "subnet-bbb"]
  public_subnet_ids  = ["subnet-ccc", "subnet-ddd"]

  container_image = "123456789012.dkr.ecr.us-east-1.amazonaws.com/gate:1.0.0"

  # Enable Application Load Balancer
  alb_enabled           = true
  alb_certificate_arn   = "arn:aws:acm:us-east-1:123456789012:certificate/abc-123"

  # Enable DynamoDB for audit logs
  audit_dynamodb_enabled = true

  tags = {
    Environment = "production"
  }
}

output "alb_dns_name" {
  value = module.gate.alb_dns_name
}

output "github_apps_secret_name" {
  value = module.gate.github_apps_secret_name
}
```

Create a backend configuration file (e.g., `prod.s3.tfbackend`):

```hcl
bucket         = "my-org-terraform-states"
key            = "gate/production.tfstate"
region         = "us-east-1"
dynamodb_table = "my-org-terraform-locks"
encrypt        = true
```

Initialize and apply:

```bash
terraform init -backend-config=prod.s3.tfbackend
terraform apply

# Update the secret with your GitHub Apps credentials
./scripts/aws/push-github-apps-to-aws-secrets-manager.sh \
  --file github-apps.json \
  --secret-name "$(terraform output -raw github_apps_secret_name)"

# Force ECS to pick up the new secret value
aws ecs update-service \
  --cluster gate \
  --service gate \
  --force-new-deployment
```

### CloudFront deployment

The CloudFront module can be added to either EKS or ECS deployments. It requires an aliased provider (`aws.us_east_1`) because WAF resources for CloudFront must be created in us-east-1 (AWS requirement).

The providers block is always required when calling this module, even if WAF is disabled. When WAF is disabled, simply map the alias to your default provider.

**With ECS (single terraform apply):**

```hcl
provider "aws" {
  region = "us-west-2"
}

provider "aws" {
  alias  = "us_east_1"
  region = "us-east-1"
}

module "ecs" {
  source = "path/to/deployments/terraform/modules/aws/ecs"
  # ... ECS configuration
}

module "cloudfront" {
  source = "path/to/deployments/terraform/modules/aws/cloudfront"

  providers = {
    aws.us_east_1 = aws.us_east_1
  }

  origin_domain = module.ecs.alb_dns_name
  waf_enabled   = true

  # Optional: custom domain
  domain_aliases      = ["gate.example.com"]
  acm_certificate_arn = "arn:aws:acm:us-east-1:123456789012:certificate/abc-123"
  route53_zone_id     = "Z1234567890ABC"
  route53_record_name = "gate"
}

output "service_url" {
  value = module.cloudfront.service_url
}
```

**With EKS (after Helm deployment):**

```hcl
provider "aws" {
  region = "us-west-2"
}

provider "aws" {
  alias  = "us_east_1"
  region = "us-east-1"
}

module "cloudfront" {
  source = "path/to/deployments/terraform/modules/aws/cloudfront"

  providers = {
    aws.us_east_1 = aws.us_east_1
  }

  origin_domain = var.ingress_endpoint  # from kubectl or Helm output
  waf_enabled   = true
}
```

**WAF disabled or already in us-east-1:**

When WAF is disabled or you're already deploying from us-east-1, you can pass your default provider to the alias:

```hcl
provider "aws" {
  region = "us-east-1"
}

module "cloudfront" {
  source = "path/to/deployments/terraform/modules/aws/cloudfront"

  providers = {
    aws.us_east_1 = aws  # map alias to default provider
  }

  origin_domain = module.ecs.alb_dns_name
  waf_enabled   = false  # or true, both work from us-east-1
}
```

## Architecture

Both modules follow a similar pattern for provisioning supporting infrastructure while differing in how the application runs.

### Storage backends

GATE requires storage for audit logs and optionally for selector rate limit state. Both modules support the same backend options:

**Audit backend:** DynamoDB is the recommended audit backend for AWS deployments. The modules create a table with configurable billing mode (on-demand or provisioned), TTL for automatic log expiration, point-in-time recovery, and optional encryption with customer-managed KMS keys.

**Selector backend:** For single-instance deployments, in-memory storage is sufficient. For multi-instance deployments, choose between DynamoDB (serverless, higher latency) or ElastiCache Redis (lower latency, requires VPC configuration). The modules can provision either option.

### IAM and secrets

Both modules create least-privilege IAM policies that grant access only to the specific resources provisioned by the module. Permissions are scoped to exact resource ARNs rather than wildcards.

GitHub Apps credentials are stored in AWS Secrets Manager. For EKS deployments, External Secrets Operator syncs the secret into Kubernetes. For ECS deployments, the task definition references the secret directly, and ECS injects it at runtime.

### Monitoring

Both the EKS and ECS modules create a CloudWatch log group for application logs with configurable retention. When DynamoDB tables are provisioned and an SNS topic ARN is provided, the modules create CloudWatch alarms for read throttling, write throttling, and system errors.

## Secrets management

The modules use Terraform's `lifecycle.ignore_changes` on the Secrets Manager secret value. This design prevents Terraform from overwriting credentials during normal operations while still allowing Terraform to manage the secret resource itself.

The recommended workflow separates infrastructure provisioning from credential management:

1. **Terraform creates the infrastructure** including a Secrets Manager secret with a placeholder value
2. **You populate the secret** using the helper script or AWS CLI
3. **Terraform ignores the secret value** on subsequent applies, preserving your credentials

To update credentials after initial deployment:

```bash
./scripts/aws/push-github-apps-to-aws-secrets-manager.sh \
  --file github-apps.json \
  --secret-name "$(terraform output -raw github_apps_secret_name)" \
  --region us-east-1
```

The script validates the JSON structure, verifies AWS credentials, and updates the secret value. Use `--dry-run` to preview changes without applying them.

If you prefer Terraform to manage the secret value directly, you can force an update using the `-replace` flag:

```bash
terraform apply \
  -replace="module.gate.aws_secretsmanager_secret_version.github_apps" \
  -var="github_apps_secret_file=./github-apps.json"
```

This approach records the change in Terraform state but requires passing the file path as a variable.

## Encryption at rest

### Secrets Manager

By default, Secrets Manager encrypts secrets using the AWS-managed key (`aws/secretsmanager`). Both the EKS and ECS modules support optional customer-managed KMS keys (CMKs) for the GitHub Apps secret, which is useful for regulatory compliance, key rotation control, or cross-account access patterns.

Three modes are available:

**AWS-managed key (default):** No configuration needed. Secrets Manager uses `aws/secretsmanager`.

**Existing CMK:** Reference an existing KMS key by ARN:

```hcl
module "gate" {
  source = "path/to/deployments/terraform/modules/aws/eks"
  # ...

  github_apps_secret_kms_key_arn = "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012"
}
```

**Module-created CMK:** Let the module create and manage a KMS key with automatic rotation:

```hcl
module "gate" {
  source = "path/to/deployments/terraform/modules/aws/eks"
  # ...

  github_apps_secret_create_kms_key = true

  # Optional: attach a custom key policy
  github_apps_secret_kms_key_policy_file = "./kms-key-policy.json"
}
```

When a CMK is configured (either existing or module-created), the modules automatically add `kms:Decrypt` and `kms:DescribeKey` permissions to the appropriate IAM roles.

### DynamoDB

DynamoDB tables support encryption with an existing customer-managed KMS key via the `audit_table_kms_key_arn` and `selector_table_kms_key_arn` variables. Unlike Secrets Manager, DynamoDB does not offer a module-created key option — provide an existing key ARN or use the default AWS-managed encryption.

If you use the module-created Secrets Manager CMK and want to share it with DynamoDB, you can compose the outputs:

```hcl
module "gate" {
  source = "path/to/deployments/terraform/modules/aws/eks"
  # ...

  github_apps_secret_create_kms_key = true
  audit_table_kms_key_arn           = module.gate.github_apps_secret_kms_key_arn
}
```

Note that sharing a single key across services means you cannot set different key policies or rotation schedules per resource. For production deployments, separate keys per resource type are generally recommended.

## Origin verification

When placing CloudFront in front of a publicly accessible ALB, you should ensure that clients cannot bypass CloudFront and access the ALB directly. One approach is to use a shared secret header: CloudFront sends a secret header with each request, and the application validates it, rejecting requests that arrive without the expected header value.

This approach is appropriate when VPC origins are not available. If you can use VPC origins, prefer that approach as it provides network-level isolation rather than application-level validation.

### Creating the secret

The secret must exist in AWS Secrets Manager before deploying both the CloudFront module (which reads it to configure the header) and the Helm chart (which configures the application to validate it).

**Using Terraform:**

Create this in your infrastructure configuration before the CloudFront module:

```hcl
resource "random_password" "origin_verify" {
  length  = 32
  special = false
}

resource "aws_secretsmanager_secret" "origin_verify" {
  name                    = "${var.service_name}/cloudfront-origin-verify"
  description             = "Shared secret for CloudFront to ALB origin verification"
  recovery_window_in_days = 7

  tags = var.tags
}

resource "aws_secretsmanager_secret_version" "origin_verify" {
  secret_id     = aws_secretsmanager_secret.origin_verify.id
  secret_string = random_password.origin_verify.result
}

output "origin_verify_secret_name" {
  description = "Name of the origin verification secret"
  value       = aws_secretsmanager_secret.origin_verify.name
}
```

**Using AWS CLI:**

```bash
SERVICE_NAME="gate"

# Generate a random secret and store it
aws secretsmanager create-secret \
  --name "${SERVICE_NAME}/cloudfront-origin-verify" \
  --description "Shared secret for CloudFront to ALB origin verification" \
  --secret-string "$(openssl rand -base64 32)"
```

### Configuring the infrastructure modules

All three infrastructure modules (CloudFront, EKS, ECS) accept either `origin_verify_secret_name` or `origin_verify_secret_arn` (mutually exclusive). Using the secret name is recommended as it is human-readable, deterministic, and stable across secret recreation.

```hcl
module "cloudfront" {
  source = "path/to/deployments/terraform/modules/aws/cloudfront"

  providers = {
    aws.us_east_1 = aws.us_east_1
  }

  origin_domain              = module.ecs.alb_dns_name
  origin_verify_secret_name  = "gate/cloudfront-origin-verify"

  # ... other configuration
}

module "eks" {
  source = "path/to/deployments/terraform/modules/aws/eks"

  origin_verify_secret_name = "gate/cloudfront-origin-verify"

  # ... other configuration
}
```

The CloudFront module reads the secret value and configures CloudFront to send it as the `X-Origin-Verify` header with each request to the origin. The EKS and ECS modules grant IAM permissions for the workload to read the secret.

### Configuring the application

For EKS deployments, configure the Helm chart to validate the header. The application reads the expected secret value from a Kubernetes secret, which is synced from Secrets Manager using External Secrets Operator.

In your Helm values:

```yaml
config:
  originVerification:
    enabled: true
    strategy: externalSecret
    externalSecret:
      remoteSecretName: "gate/cloudfront-origin-verify"
```

For ECS deployments, the application reads the secret directly from Secrets Manager at runtime using the IAM task role.

### Deployment order

When using origin verification, follow this deployment order:

1. Create the origin verification secret (Terraform snippet above or AWS CLI)
2. Deploy EKS or ECS infrastructure (Terraform)
3. Deploy application with origin verification enabled (Helm for EKS, or ECS service update)
4. Deploy CloudFront referencing the secret (Terraform)

If you have already deployed the application without origin verification, you can enable it by updating the Helm values and redeploying. Requests through CloudFront will continue to work, while direct ALB requests will be rejected once the application restarts with the new configuration.

## Module documentation

Each module's input variables, output values, and resource dependencies are documented inline in its `variables.tf` and `outputs.tf` files. Refer to these files directly for the most accurate and up-to-date documentation.
