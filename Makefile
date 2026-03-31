# Copyright 2026 Thomson Reuters
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# =============================================================================
# Main variables
# =============================================================================
BINARY_NAME := gate
DOCKER_IMAGE := gate
DOCKER_TAG := latest
GO_VERSION := 1.26.1

# Container tool detection (prefer podman, fallback to docker)
CONTAINER_TOOL := $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null || echo "docker")

# Platform for container builds (override with: make image-build PLATFORM=linux/arm64)
PLATFORM ?= linux/amd64

# Docker registry (override with: make image-push REGISTRY=example-registry.com/example-org)
REGISTRY ?=
IMAGE_NAME := $(if $(REGISTRY),$(REGISTRY)/$(DOCKER_IMAGE),$(DOCKER_IMAGE))

# Directories
CMD_SERVICE_DIR := .
BUILD_DIR := ./bin
COVERAGE_DIR := ./coverage

# Version info
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# FIPS 140-3 compliance (optional)
# Set to "latest" or a specific module version (e.g., "v1.0.0") to enable FIPS 140-3 mode.
# Leave as "off" to build without FIPS support.
# See: https://go.dev/doc/security/fips140
GOFIPS140 ?= off


# Go build flags
LDFLAGS := -s -w \
	-X github.com/thomsonreuters/gate/internal/constants.Version=$(VERSION) \
	-X github.com/thomsonreuters/gate/internal/constants.CommitHash=$(COMMIT) \
	-X github.com/thomsonreuters/gate/internal/constants.BuildTimestamp=$(BUILD_DATE)
GO_BUILD := GOFIPS140=$(GOFIPS140) go build -ldflags="$(LDFLAGS)"

# Development environment
UID := $(shell id -u)
GID := $(shell id -g)
CONFIG_FILE ?= config.yaml
COMPOSE_CMD := HOST_UID=$(UID) HOST_GID=$(GID) CONFIG_FILE=$(CONFIG_FILE) docker compose -f local/compose.dev.yml
VOLUMES_DIR := ./local/.volumes
# =============================================================================

# =============================================================================
# Base image digests
# =============================================================================
# All base images are pinned by SHA256 digest for security.
# Digests are architecture-specific and selected automatically based on PLATFORM.
#
# To update digests:
# podman manifest inspect <image:tag> | jq '.manifests[] | select(.platform.architecture == "amd64" or .platform.architecture == "arm64") | {arch: .platform.architecture, digest}'
#
# Golang Alpine (builder image)
# Image: public.ecr.aws/docker/library/golang:1.26.1-alpine
GOLANG_DIGEST_AMD64 := sha256:d337ecb3075f0ec76d81652b3fa52af47c3eba6c8ba9f93b835752df7ce62946
GOLANG_DIGEST_ARM64 := sha256:c500d8fac0707aa2a887d7e426530cfef09549c9c87ac0c2998543a89ce89d86

# Distroless Static (runtime image)
# Image: gcr.io/distroless/static-debian13:nonroot
DISTROLESS_DIGEST_AMD64 := sha256:88a46f645e304fc0dcfbdacdfa338ce02d9890df5f936872243d553278deae92
DISTROLESS_DIGEST_ARM64 := sha256:d20ee35777d1be105385ebbbf989178e5d3d0d254059e8a09a38ef64aebc741a

# Select digests based on PLATFORM
ifeq ($(PLATFORM),linux/arm64)
    GOLANG_DIGEST := $(GOLANG_DIGEST_ARM64)
    DISTROLESS_DIGEST := $(DISTROLESS_DIGEST_ARM64)
else
    GOLANG_DIGEST := $(GOLANG_DIGEST_AMD64)
    DISTROLESS_DIGEST := $(DISTROLESS_DIGEST_AMD64)
endif
# =============================================================================

# ============================================================================
# Targets
# ============================================================================

.PHONY: help
help:
	@echo "AVAILABLE TARGETS"
	@echo ""
	@echo "Build:"
	@echo "  make build                - Build GATE binary"
	@echo ""
	@echo "Image:"
	@echo "  make image-build          - Build container image"
	@echo "  make image-push           - Push image to registry"
	@echo "  make image-scan           - Scan image for vulnerabilities"
	@echo ""
	@echo "Testing:"
	@echo "  make test                 - Run all tests (unit + integration)"
	@echo "  make test-unit            - Run unit tests only"
	@echo "  make test-integration     - Run integration tests only"
	@echo ""
	@echo "Development:"
	@echo "  make run                  - Run GATE locally"
	@echo "  make lint                 - Run linters"
	@echo "  make fmt                  - Format code"
	@echo "  make vet                  - Run go vet"
	@echo "  make tidy                 - Tidy Go modules"
	@echo "  make deps                 - Install dependencies"
	@echo "  make clean                - Remove build artifacts"
	@echo "  make setup                - Setup development environment"
	@echo ""
	@echo "Local development (Docker Compose + Air hot-reload):"
	@echo "  make dev-init             - Initialize dev environment (build images)"
	@echo "  make dev-deps             - Start only dependencies (postgres, redis)"
	@echo "  make dev                  - Start full dev environment with hot-reload"
	@echo "  make dev-down             - Stop dev environment"
	@echo "  make dev-clean            - Stop and remove dev volumes/images"
	@echo ""
	@echo "  CONFIG_FILE=config.postgres.yaml make dev (use specific config)"
	@echo ""
	@echo "CI/CD:"
	@echo "  make security             - Run security scan"
	@echo "  make check                - Run all quality checks (fmt, vet, lint, test)"
	@echo "  make ci                   - Full CI pipeline"
	@echo ""
	@echo "Information:"
	@echo "  make version              - Show version info"
	@echo "  make digests              - Show current base image digests"
	@echo ""
	@echo ""
	@echo "Container tool:   $(CONTAINER_TOOL)"
	@echo "Current platform: $(PLATFORM)"

.PHONY: test
test: test-unit test-integration
	@echo "All tests completed."

.PHONY: test-unit
test-unit:
	@echo "Running unit tests..."
	@mkdir -p $(COVERAGE_DIR)
	@go test -v -race -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	@go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@go tool cover -func=$(COVERAGE_DIR)/coverage.out | grep total | awk '{print "Unit test coverage: " $$3}'

.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	@go test -v -race -tags=integration ./test/integration/...

.PHONY: build
build:
	@echo "Building GATE..."
	@echo "  Version:  $(VERSION)"
	@echo "  Commit:   $(COMMIT)"
	@echo "  FIPS 140: $(GOFIPS140)"
	@mkdir -p $(BUILD_DIR)
	$(GO_BUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_SERVICE_DIR)
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)"

.PHONY: image-build
image-build:
	@echo "Building container image..."
	@echo "  Container tool: $(CONTAINER_TOOL)"
	@echo "  Platform:       $(PLATFORM)"
	@echo "  Golang digest:  $(GOLANG_DIGEST)"
	@echo "  Runtime digest: $(DISTROLESS_DIGEST)"
	@echo "  FIPS 140-3:     $(GOFIPS140)"
	@echo "  Version:        $(VERSION)"
	@echo "  Commit:         $(COMMIT)"
	@echo "  Build date:     $(BUILD_DATE)"
	$(CONTAINER_TOOL) build \
		--platform $(PLATFORM) \
		--build-arg GOLANG_DIGEST=$(GOLANG_DIGEST) \
		--build-arg DISTROLESS_DIGEST=$(DISTROLESS_DIGEST) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--build-arg GOFIPS140=$(GOFIPS140) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):$(DOCKER_TAG) \
		-f Dockerfile \
		.
	@echo "✓ Image built: $(IMAGE_NAME):$(VERSION)"
	@echo "✓ Image tagged: $(IMAGE_NAME):$(DOCKER_TAG)"

.PHONY: image-push
image-push:
	@if [ -z "$(REGISTRY)" ]; then \
		echo "ERROR: REGISTRY not set. Usage: make image-push REGISTRY=example-registry.com/example-org"; \
		exit 1; \
	fi
	@echo "Pushing container image to registry..."
	$(CONTAINER_TOOL) push $(IMAGE_NAME):$(VERSION)
	$(CONTAINER_TOOL) push $(IMAGE_NAME):$(DOCKER_TAG)
	@echo "✓ Pushed: $(IMAGE_NAME):$(VERSION)"
	@echo "✓ Pushed: $(IMAGE_NAME):$(DOCKER_TAG)"

.PHONY: image-scan
image-scan:
	@echo "Scanning container image for vulnerabilities..."
	@if command -v trivy >/dev/null 2>&1; then \
		echo "Using trivy..."; \
		trivy image --severity HIGH,CRITICAL $(IMAGE_NAME):$(DOCKER_TAG); \
	elif docker scout version >/dev/null 2>&1; then \
		echo "Using docker scout..."; \
		docker scout cves $(IMAGE_NAME):$(DOCKER_TAG); \
	else \
		echo "ERROR: No security scanner found. Install trivy or docker scout."; \
		exit 1; \
	fi

.PHONY: run
run: build
	@echo "Starting service locally..."
	$(BUILD_DIR)/$(BINARY_NAME)

.PHONY: lint
lint:
	@echo "Running linters..."
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not installed. Installing..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	}
	golangci-lint run ./...

.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

.PHONY: vet
vet:
	@echo "Running go vet..."
	go vet ./...

.PHONY: tidy
tidy:
	@echo "Tidying Go modules..."
	go mod tidy

.PHONY: deps
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod verify

.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -rf $(COVERAGE_DIR)
	go clean -cache -testcache
	@echo "Clean complete"

.PHONY: security
security:
	@echo "Running security scan..."
	@command -v gosec >/dev/null 2>&1 || { \
		echo "gosec not installed. Installing..."; \
		go install github.com/securego/gosec/v2/cmd/gosec@latest; \
	}
	gosec ./...

.PHONY: check
check: fmt vet lint test
	@echo "All checks passed!"

.PHONY: ci
ci: deps check security
	@echo "CI pipeline complete!"

.PHONY: setup
setup:
	@echo "Setting up local development environment..."
	@command -v golangci-lint >/dev/null 2>&1 || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@command -v gosec >/dev/null 2>&1 || go install github.com/securego/gosec/v2/cmd/gosec@latest
	go mod download
	@echo "Setup complete!"

.PHONY: dev-init
dev-init:
	@echo "Initializing dev environment..."
	@mkdir -p $(VOLUMES_DIR)
	$(COMPOSE_CMD) build

.PHONY: dev-deps
dev-deps:
	@echo "Starting service dependencies..."
	$(COMPOSE_CMD) up -d postgres redis redis-insight localstack adminer
	@echo "Dependencies started (postgres:5432, redis:6379, redis-insight:5540, localstack/dynamodb:4566, adminer:8081)"

.PHONY: dev
dev: dev-deps
	@echo "Starting dev environment with hot-reload..."
	$(COMPOSE_CMD) down server
	$(COMPOSE_CMD) up server -d
	$(COMPOSE_CMD) logs -f server

.PHONY: dev-down
dev-down:
	@echo "Stopping dev environment..."
	$(COMPOSE_CMD) down

.PHONY: dev-clean
dev-clean:
	@echo "Cleaning dev environment..."
	$(COMPOSE_CMD) down -v --rmi local
	rm -rf $(VOLUMES_DIR)

.PHONY: version
version:
	@echo "Version:    $(VERSION)"
	@echo "Commit:     $(COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

.PHONY: digests
digests:
	@echo "Base Image Digests for $(PLATFORM)"
	@echo "========================================"
	@echo ""
	@echo "Golang Alpine (builder):"
	@echo "  Image: public.ecr.aws/docker/library/golang:$(GO_VERSION)-alpine"
	@echo "  AMD64: $(GOLANG_DIGEST_AMD64)"
	@echo "  ARM64: $(GOLANG_DIGEST_ARM64)"
	@echo "  Selected: $(GOLANG_DIGEST)"
	@echo ""
	@echo "Distroless Static (runtime):"
	@echo "  Image: gcr.io/distroless/static-debian13:nonroot"
	@echo "  AMD64: $(DISTROLESS_DIGEST_AMD64)"
	@echo "  ARM64: $(DISTROLESS_DIGEST_ARM64)"
	@echo "  Selected: $(DISTROLESS_DIGEST)"
	@echo ""
	@echo "To update digests, run:"
	@echo "  podman manifest inspect <image:tag> | jq '.manifests[] | select(.platform.architecture == \"amd64\" or .platform.architecture == \"arm64\")'"
# =============================================================================
