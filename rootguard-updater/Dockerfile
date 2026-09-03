# Digest-pinned, not just tag-pinned - see rootguard-core/Dockerfile's
# identical comment for why.
FROM golang:1.26-alpine@sha256:28d89ee9cc0ff9fec75c82ca201e6bf7fdf9a679d4b7b24dfa04f2bb766bb468 AS builder

WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /rootguard-updater .

# Bumped from v3.0.6 - see rootguard-core/Dockerfile's identical comment
# for the full rationale (39 HIGH findings in the old pin's own binary,
# plus a real signature-verification-bypass fix in v3.1.3).
FROM ghcr.io/sigstore/cosign/cosign:v3.1.3@sha256:9e5c2f2edc34351160407ca3416c61855bdf9403c3c5936e0f0be7fc261611b8 AS cosign

FROM docker:29-cli@sha256:000bb62ff495f986c9f5578eb67cc2cb98b91138eda81d7762d5371eb8a497fe

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

# Found in review: both packages used to be apk-installed here unpinned
# (a real reproducible-builds gap) but are already present in this exact
# pinned docker:29-cli digest - ca-certificates from its own base
# Dockerfile's `apk add ca-certificates ...`, docker-cli-compose (the
# CLI-plugin form, matching how this codebase actually invokes it -
# `docker compose ...`, see manager.go's m.run calls) bundled as
# /usr/local/libexec/docker/cli-plugins/docker-compose. Removing the
# install entirely closes the reproducibility gap outright rather than
# merely pinning packages that were never needed - see
# rootguard-core/Dockerfile's identical finding/fix for the full
# verification.
#
# Found in review: see rootguard-core/Dockerfile's identical comment for
# the full CVE-2026-14456 and unused-buildx-plugin rationale - this exact
# pinned digest is the same one Core uses, so both stopgaps apply here
# too. OpenSSH is removed because RootGuard never uses ssh:// Docker
# contexts; libexpat retains its CVE-2026-66046/CVE-2026-76641 stopgap.
RUN apk del openssh-client \
    && apk add --no-cache --upgrade \
        libssl3=3.5.8-r0 libcrypto3=3.5.8-r0 libexpat=2.8.4-r0 \
    && rm -f /usr/local/libexec/docker/cli-plugins/docker-buildx

COPY --from=builder /rootguard-updater /usr/local/bin/rootguard-updater
COPY --from=cosign /ko-app/cosign /usr/local/bin/cosign

# Deliberately no USER instruction: like Core, the Updater holds the Docker
# socket to drive paired update/rollback across the managed services, and
# socket access requires either running as root or matching the host's
# docker group GID, which differs per installation (see the matching note
# in rootguard-core/Dockerfile). Suppressed for this file specifically in
# .trivyignore.yaml (AVD-DS-0002), not inline - trivy's inline-ignore
# comments only attach to a specific line, and this check has none.
EXPOSE 8082
ENTRYPOINT ["rootguard-updater"]
