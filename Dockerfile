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

ARG GOLANG_DIGEST
ARG DISTROLESS_DIGEST

# -----------------------------------------------------------------------------
# Stage 1: Builder
# -----------------------------------------------------------------------------
FROM public.ecr.aws/docker/library/golang@${GOLANG_DIGEST} AS builder


RUN apk add --no-cache \
    git=2.52.0-r0 \
    ca-certificates=20251003-r0 \
    file=5.46-r2

WORKDIR /build

# Optional: Custom CA certificates for corporate proxies (e.g., Zscaler, mitmproxy)
# Place .crt files in certs/ directory – they will be automatically added to system trust store
# Supports certificate chains (multiple certs in one file)
COPY certs/ /usr/local/share/ca-certificates/
RUN update-ca-certificates

COPY go.mod go.sum ./
RUN go mod download && \
    go mod verify

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# FIPS 140-3 compliance (optional)
# Set to "latest" or a specific module version (e.g., "v1.0.0") to build with FIPS support.
# Default "off" builds without FIPS. See: https://go.dev/doc/security/fips140
ARG GOFIPS140=off

RUN CGO_ENABLED=0 \
    GOFIPS140=${GOFIPS140} \
    go build \
    -ldflags="-s -w \
      -X github.com/thomsonreuters/gate/internal/constants.Version=${VERSION} \
      -X github.com/thomsonreuters/gate/internal/constants.CommitHash=${COMMIT} \
      -X github.com/thomsonreuters/gate/internal/constants.BuildTimestamp=${BUILD_DATE}" \
    -trimpath \
    -o /build/gate \
    .

RUN file /build/gate | grep -q 'statically linked' || \
    (echo "ERROR: Binary is not statically linked" && \
     file /build/gate && \
     exit 1)

# -----------------------------------------------------------------------------
# Stage 2: Runtime (distroless)
# -----------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian13@${DISTROLESS_DIGEST}

ARG VERSION=dev
LABEL org.opencontainers.image.title="GATE: GitHub Authenticated Token Exchange" \
      org.opencontainers.image.description="Secure token service for exchanging OIDC tokens for GitHub App installation tokens" \
      org.opencontainers.image.vendor="Thomson Reuters" \
      org.opencontainers.image.source="https://github.com/thomsonreuters/gate" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${VERSION}" \
      security.scan.enabled="true" \
      security.scan.minimum-severity="HIGH"

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder --chown=65532:65532 /build/gate /usr/local/bin/gate

EXPOSE 8080

USER 65532:65532

ENTRYPOINT ["/usr/local/bin/gate"]

CMD ["server"]
