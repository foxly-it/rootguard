# Digest-pinned, not just tag-pinned - found in review: two builds of the
# same commit could otherwise pull different content if the tag moved
# between them, and a compromised/re-pushed upstream tag would be pulled
# without any change to this file. Same reasoning already applied to
# cosign below and to rootguard-unbound/rootguard-blockpage's base images.
FROM golang:1.26-alpine@sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /rootguard-core ./cmd/rootguard

# Bumped from v3.0.6 - found live via the new trivy-image-scan.sh step
# (round 13): that pin's own bundled binary carried 39 HIGH findings of
# its own (mostly the same Go-stdlib CVEs docker:29-cli's own CLI binary
# has, since it embedded the same stale toolchain, plus a handful of
# sigstore/fulcio-specific ones), none of which the review's own scan had
# surfaced. v3.1.3 also fixes GHSA-fx35-mq7g-6g98, a real signature-
# verification bypass via an unexpected public key in a legacy bundle -
# directly relevant here, since this exact binary is what
# stack.RequireAttestation (rootguard-core/internal/stack) calls at
# deploy time to verify a published image's attestation before trusting
# it. release-alpha.yml's own cosign-installer pin must move with this
# one - see its identical comment.
FROM ghcr.io/sigstore/cosign/cosign:v3.1.3@sha256:9e5c2f2edc34351160407ca3416c61855bdf9403c3c5936e0f0be7fc261611b8 AS cosign

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
#
# Found in review: this exact pinned digest also needs the package fixes
# and runtime reductions below. Replace the version pins with a plain
# digest bump once a docker:29-cli build contains those fixes upstream;
# the unused-tool removals remain intentional across future digest bumps.
#   - libssl3/libcrypto3 3.5.7-r0 here has CVE-2026-14456, fixed in
#     3.5.8-r0 - already published on Alpine 3.24's own `main` repo
#     (confirmed live via pkgs.alpinelinux.org), just not yet what this
#     digest's own image layer has baked in.
#   - the bundled buildx CLI plugin
#     (/usr/local/libexec/docker/cli-plugins/docker-buildx, confirmed via
#     the upstream docker-library/docker Dockerfile this digest is built
#     from) is dead weight, not a stopgap: nothing in this codebase's own
#     `docker ...` invocations (m.run calls, see above) or in any of the
#     managed containers ever calls `docker buildx` - only this repo's own
#     CI (release-alpha.yml) does, on the runner's Docker CLI, entirely
#     separate from this image. Removing it is a real image-content
#     reduction (fewer packages a scanner can find something in), not
#     something a later digest bump would ever need to add back.
#   - docker:29-cli installs OpenSSH only for optional ssh:// Docker
#     contexts. RootGuard deliberately uses the mounted local Docker socket
#     and never creates or selects a remote context, so remove OpenSSH
#     instead of carrying and continuously patching an unused SSH client.
#   - Found live, round 15: libexpat 2.8.2-r0 here (a different package
#     from Blockpage's own nginx-base libexpat pin - this is
#     docker:29-cli's own bundled copy) has CVE-2026-66046/CVE-2026-76641
#     (both "Expat through 2.8.3" DoS), fixed in 2.8.4-r0 - already
#     published on Alpine 3.24's own `main` repo (confirmed live). Same
#     stopgap pattern as the rest of this block.
RUN apk del openssh-client \
    && apk add --no-cache --upgrade \
        libssl3=3.5.8-r0 libcrypto3=3.5.8-r0 libexpat=2.8.4-r0 \
    && rm -f /usr/local/libexec/docker/cli-plugins/docker-buildx \
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
