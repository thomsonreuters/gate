{{/*
Expand the name of the chart.
*/}}
{{- define "gate.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this.
If release name contains chart name it will be used as a full name.
*/}}
{{- define "gate.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "gate.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "gate.labels" -}}
helm.sh/chart: {{ include "gate.chart" . }}
{{ include "gate.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "gate.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gate.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "gate.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "gate.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Image reference with optional registry.
When image.digest is set it takes precedence over image.tag (joined with "@").
Otherwise image.tag is used (joined with ":"), falling back to Chart.AppVersion.
*/}}
{{- define "gate.image" -}}
{{- $separator := ":" }}
{{- $termination := .Values.image.tag | default .Chart.AppVersion }}
{{- if .Values.image.digest }}
{{- $separator = "@" }}
{{- $termination = .Values.image.digest }}
{{- end }}
{{- if .Values.image.registry }}
{{- printf "%s/%s%s%s" .Values.image.registry .Values.image.repository $separator $termination }}
{{- else }}
{{- printf "%s%s%s" .Values.image.repository $separator $termination }}
{{- end }}
{{- end }}

{{/*
GitHub Apps secret name
Returns the secret name based on the selected strategy.
*/}}
{{- define "gate.githubAppsSecretName" -}}
{{- if eq .Values.config.githubApps.strategy "externalSecret" }}
{{- printf "%s-github-apps" (include "gate.fullname" .) }}
{{- else }}
{{- .Values.config.githubApps.kubernetesSecret.name }}
{{- end }}
{{- end }}

{{/*
Origin verification secret name
Returns the secret name based on the selected strategy.
*/}}
{{- define "gate.originVerifySecretName" -}}
{{- if eq .Values.config.originVerification.strategy "externalSecret" }}
{{- printf "%s-origin-verify" (include "gate.fullname" .) }}
{{- else }}
{{- .Values.config.originVerification.kubernetesSecret.name }}
{{- end }}
{{- end }}

{{/*
Origin verification secret key
Returns the key within the secret containing the verification value.
*/}}
{{- define "gate.originVerifySecretKey" -}}
{{- if eq .Values.config.originVerification.strategy "externalSecret" }}
{{- "value" }}
{{- else }}
{{- .Values.config.originVerification.kubernetesSecret.key }}
{{- end }}
{{- end }}

{{/*
Validate required values
Fails with a helpful error message if required configuration is missing.
*/}}
{{- define "gate.validateValues" -}}
{{/* Validate TLS configuration */}}
{{- if .Values.config.server.tls.enabled }}
  {{- if not .Values.config.server.tls.secretName }}
    {{- fail "config.server.tls.secretName is required when TLS is enabled" }}
  {{- end }}
{{- end }}
{{/* Validate GitHub Apps configuration */}}
{{- if not .Values.config.githubApps.apps }}
  {{- fail "config.githubApps.apps is required - at least one GitHub App must be configured" }}
{{- end }}
{{- if eq .Values.config.githubApps.strategy "kubernetesSecret" }}
  {{- if not .Values.config.githubApps.kubernetesSecret.name }}
    {{- fail "config.githubApps.kubernetesSecret.name is required when strategy is 'kubernetesSecret'" }}
  {{- end }}
{{- else if eq .Values.config.githubApps.strategy "externalSecret" }}
  {{- if not .Values.config.githubApps.externalSecret.enabled }}
    {{- fail "config.githubApps.externalSecret.enabled must be true when strategy is 'externalSecret'" }}
  {{- end }}
  {{- if not .Values.config.githubApps.externalSecret.remoteSecretName }}
    {{- fail "config.githubApps.externalSecret.remoteSecretName is required when strategy is 'externalSecret'" }}
  {{- end }}
  {{/* Validate SecretStore configuration when using externalSecret */}}
  {{- if .Values.externalSecrets.secretStore.create }}
    {{- if not .Values.externalSecrets.secretStore.name }}
      {{- fail "externalSecrets.secretStore.name is required when externalSecrets.secretStore.create is true" }}
    {{- end }}
  {{- end }}
{{- end }}
{{/* Validate origin verification configuration */}}
{{- if .Values.config.originVerification.enabled }}
  {{- if eq .Values.config.originVerification.strategy "kubernetesSecret" }}
    {{- if not .Values.config.originVerification.kubernetesSecret.name }}
      {{- fail "config.originVerification.kubernetesSecret.name is required when strategy is 'kubernetesSecret'" }}
    {{- end }}
    {{- if not .Values.config.originVerification.kubernetesSecret.key }}
      {{- fail "config.originVerification.kubernetesSecret.key is required when strategy is 'kubernetesSecret'" }}
    {{- end }}
  {{- else if eq .Values.config.originVerification.strategy "externalSecret" }}
    {{- if not .Values.config.originVerification.externalSecret.remoteSecretName }}
      {{- fail "config.originVerification.externalSecret.remoteSecretName is required when strategy is 'externalSecret'" }}
    {{- end }}
  {{- end }}
{{- end }}
{{/* Validate audit backend configuration */}}
{{- if eq .Values.config.audit.backend "dynamodb" }}
  {{- if not .Values.config.audit.dynamodb.tableName }}
    {{- fail "config.audit.dynamodb.tableName is required when audit backend is 'dynamodb'" }}
  {{- end }}
{{- else if eq .Values.config.audit.backend "sql" }}
  {{- if and (not .Values.config.audit.sql.dsn) (not .Values.config.audit.sql.existingSecret.name) }}
    {{- fail "Either config.audit.sql.dsn or config.audit.sql.existingSecret.name is required when audit backend is 'sql'" }}
  {{- end }}
  {{- if and .Values.config.audit.sql.dsn .Values.config.audit.sql.existingSecret.name }}
    {{- fail "config.audit.sql: Cannot set both 'dsn' and 'existingSecret.name' - choose one" }}
  {{- end }}
  {{- if and .Values.config.audit.sql.existingSecret.name (not .Values.config.audit.sql.existingSecret.key) }}
    {{- fail "config.audit.sql.existingSecret.key is required when existingSecret.name is set" }}
  {{- end }}
{{- end }}
{{/* Validate selector store configuration */}}
{{- if eq .Values.config.selector.type "redis" }}
  {{- if not .Values.config.selector.redis.address }}
    {{- fail "config.selector.redis.address is required when selector store is 'redis'" }}
  {{- end }}
  {{- if and .Values.config.selector.redis.password .Values.config.selector.redis.existingSecret.name }}
    {{- fail "config.selector.redis: Cannot set both 'password' and 'existingSecret.name' - choose one" }}
  {{- end }}
  {{- if and .Values.config.selector.redis.existingSecret.name (not .Values.config.selector.redis.existingSecret.key) }}
    {{- fail "config.selector.redis.existingSecret.key is required when existingSecret.name is set" }}
  {{- end }}
{{- else if eq .Values.config.selector.type "dynamodb" }}
  {{- if not .Values.config.selector.dynamodb.tableName }}
    {{- fail "config.selector.dynamodb.tableName is required when selector type is 'dynamodb'" }}
  {{- end }}
{{- end }}
{{/* Validate AWS region when using DynamoDB backends */}}
{{- if or (eq .Values.config.audit.backend "dynamodb") (eq .Values.config.selector.type "dynamodb") }}
  {{- if not .Values.config.awsRegion }}
    {{- fail "config.awsRegion is required when using DynamoDB backends (audit or selector)" }}
  {{- end }}
{{- end }}
{{/* Validate OIDC audience */}}
{{- if not .Values.config.oidc.audience }}
  {{- fail "config.oidc.audience is required" }}
{{- end }}
{{/* Validate policy configuration */}}
{{- if not .Values.config.policy.trustPolicyPath }}
  {{- fail "config.policy.trustPolicyPath is required" }}
{{- end }}
{{- if not .Values.config.policy.providers }}
  {{- fail "config.policy.providers is required (at least one OIDC provider)" }}
{{- else if eq (len .Values.config.policy.providers) 0 }}
  {{- fail "config.policy.providers must contain at least one OIDC provider" }}
{{- end }}
{{/* Validate TTL configuration */}}
{{- if gt .Values.config.policy.defaultTokenTTL .Values.config.policy.maxTokenTTL }}
  {{- fail "config.policy.defaultTokenTTL cannot exceed config.policy.maxTokenTTL" }}
{{- end }}
{{/* Production security warnings (non-fatal) */}}
{{- if and (eq .Values.config.selector.type "redis") (not .Values.config.selector.redis.tls) }}
  {{- printf "\nWARNING: config.selector.redis.tls is disabled. This is insecure for production deployments with managed Redis services.\n" | print }}
{{- end }}
{{- if and (eq .Values.config.audit.backend "sql") .Values.config.audit.sql.dsn }}
  {{- printf "\nWARNING: config.audit.sql.dsn is set directly in values. For production, use existingSecret instead to avoid exposing credentials.\n" | print }}
{{- end }}
{{- if and (eq .Values.config.selector.type "redis") .Values.config.selector.redis.password }}
  {{- printf "\nWARNING: config.selector.redis.password is set directly in values. For production, use existingSecret instead to avoid exposing credentials.\n" | print }}
{{- end }}
{{- end }}
