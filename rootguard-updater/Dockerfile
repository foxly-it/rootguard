FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /rootguard-updater .

FROM docker:29-cli

ARG VERSION=dev
ARG COMMIT=unknown
ARG CREATED=unknown
LABEL org.opencontainers.image.title="RootGuard Updater" \
      org.opencontainers.image.description="Isolated control-plane update and rollback helper for RootGuard" \
      org.opencontainers.image.source="https://github.com/foxly-it/rootguard" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${CREATED}" \
      org.opencontainers.image.licenses="AGPL-3.0-or-later"

RUN apk add --no-cache docker-cli-compose ca-certificates
COPY --from=builder /rootguard-updater /usr/local/bin/rootguard-updater

# Deliberately no USER instruction: like Core, the Updater holds the Docker
# socket to drive paired update/rollback across the managed services, and
# socket access requires either running as root or matching the host's
# docker group GID, which differs per installation (see the matching note
# in rootguard-core/Dockerfile). Suppressed for this file specifically in
# .trivyignore.yaml (AVD-DS-0002), not inline - trivy's inline-ignore
# comments only attach to a specific line, and this check has none.
EXPOSE 8082
ENTRYPOINT ["rootguard-updater"]
