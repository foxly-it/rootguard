# Digest-pinned, not just tag-pinned - see rootguard-core/Dockerfile's
# identical comment for why.
# Compiles natively on the build host, cross-compiled for the target
# platform, instead of under QEMU for arm64 (#736).
FROM --platform=$BUILDPLATFORM golang:1.26-alpine@sha256:9ce231b28998b32e116927c61aa5c406183061d4f3f2d61b292c88e7f6bdf7ca AS builder
ARG TARGETOS TARGETARCH

WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /rootguard-updater .

# Bumped from v3.0.6 - see rootguard-core/Dockerfile's identical comment
# for the full rationale (39 HIGH findings in the old pin's own binary,
# plus a real signature-verification-bypass fix in v3.1.3; v3.1.4 for
# CVE-2026-63209 in klauspost/compress, #766).
FROM ghcr.io/sigstore/cosign/cosign:v3.1.4@sha256:0f06154836d2de34f887a5c273506d52ca43edaa1d9e53e6510fa78a48f69c2d AS cosign

FROM docker:29-cli@sha256:6fae2214c3121a28d5aa639d09032f61310b7c9c75ffbad1025f261a1b621800

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
# contexts. The #703 digest bump left only the OpenSSL pin; libexpat is
# pinned again for CVE-2026-77214 (#745), brotli-libs for CVE-2026-111497
# (#778).
RUN apk del openssh-client \
    && apk add --no-cache --upgrade \
        libssl3=3.5.9-r0 libcrypto3=3.5.9-r0 libexpat=2.9.0-r0 brotli-libs=1.2.1-r0 \
    && rm -f /usr/local/libexec/docker/cli-plugins/docker-buildx

COPY --from=builder /rootguard-updater /usr/local/bin/rootguard-updater
COPY --from=cosign /ko-app/cosign /usr/local/bin/cosign

# Deliberately no USER instruction: like Core, the Updater no longer holds
# the real Docker socket at all (rootguard-docker-proxy does - see the
# matching note in rootguard-core/Dockerfile for the full rationale,
# including why dropping root is a separate, not-yet-done follow-up).
# Suppressed for this file specifically in .trivyignore.yaml
# (AVD-DS-0002), not inline - trivy's inline-ignore comments only attach
# to a specific line, and this check has none.
EXPOSE 8082
ENTRYPOINT ["rootguard-updater"]
