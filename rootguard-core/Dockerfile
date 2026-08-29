# Digest-pinned, not just tag-pinned - found in review: two builds of the
# same commit could otherwise pull different content if the tag moved
# between them, and a compromised/re-pushed upstream tag would be pulled
# without any change to this file. Same reasoning already applied to
# cosign below and to rootguard-unbound/rootguard-blockpage's base images.
FROM golang:1.26-alpine@sha256:28d89ee9cc0ff9fec75c82ca201e6bf7fdf9a679d4b7b24dfa04f2bb766bb468 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /rootguard-core ./cmd/rootguard

FROM ghcr.io/sigstore/cosign/cosign:v3.0.6@sha256:de9c65609e6bde17e6b48de485ee788407c9502fa08b8f4459f595b21f56cd00 AS cosign

FROM docker:29-cli@sha256:000bb62ff495f986c9f5578eb67cc2cb98b91138eda81d7762d5371eb8a497fe

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

# Found in review: docker-cli-compose used to be apk-installed here
# unpinned (a real reproducible-builds gap - the exact package version
# fetched could drift between two builds of the same commit). Verified
# against the upstream docker-library/docker source, correlated to this
# exact pinned digest's own build date (2026-08-10, via `docker buildx
# imagetools inspect`) rather than that source's current HEAD - a
# follow-up review correctly caught that the two can disagree - that
# docker:29-cli already bundles docker compose (v5.4.0 as of this
# digest) as a CLI plugin at
# /usr/local/libexec/docker/cli-plugins/docker-compose - one of Docker
# CLI's standard plugin search paths, and exactly where `docker compose
# ...` (the subcommand form this codebase actually invokes, see
# internal/installer/manager.go's m.run calls) looks - so the apk
# install was purely redundant, not filling a real gap. Removing it
# entirely closes the reproducibility gap outright instead of merely
# pinning a package version that was never needed.
RUN mkdir -p /var/lib/rootguard/unbound

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
