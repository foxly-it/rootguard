FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /rootguard-core ./cmd/rootguard

FROM ghcr.io/sigstore/cosign/cosign:v3.0.6@sha256:de9c65609e6bde17e6b48de485ee788407c9502fa08b8f4459f595b21f56cd00 AS cosign

FROM docker:29-cli

ARG VERSION=dev
ARG COMMIT=unknown
ARG CREATED=unknown
LABEL org.opencontainers.image.title="RootGuard Core" \
      org.opencontainers.image.description="Privileged orchestration and configuration for RootGuard" \
      org.opencontainers.image.source="https://github.com/foxly-it/rootguard" \
      org.opencontainers.image.licenses="AGPL-3.0-or-later" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${CREATED}"

RUN apk add --no-cache docker-cli-compose \
    && mkdir -p /var/lib/rootguard/unbound

COPY --from=builder /rootguard-core /usr/local/bin/rootguard-core
COPY --from=cosign /ko-app/cosign /usr/local/bin/cosign

# Deliberately no USER instruction: Core holds the Docker socket to
# orchestrate the other managed services, and socket access requires either
# running as root or matching the host's docker group GID, which differs
# per installation and can't be hardcoded here without breaking on hosts
# where it doesn't match. Tracked as a real gap, not an oversight - the
# concrete follow-up is a dedicated docker-socket-proxy sidecar that lets
# Core itself run unprivileged while a narrow, allowlisted proxy holds the
# actual socket (see ROADMAP.md 0.5's threat-model item). Suppressed for
# this file specifically in .trivyignore.yaml (AVD-DS-0002), not inline -
# trivy's inline-ignore comments only attach to a specific line, and this
# check (like AVD-DS-0026 below) has none: it's a whole-file absence check,
# so an inline comment anywhere in the file can never match it.
EXPOSE 8081
ENTRYPOINT ["rootguard-core"]
