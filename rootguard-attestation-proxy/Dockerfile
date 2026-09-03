# Digest-pinned, not just tag-pinned - see rootguard-core/Dockerfile's
# identical comment for why.
FROM golang:1.26-alpine@sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628 AS builder

WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /rootguard-attestation-proxy .

# scratch, not alpine/distroless: this binary is a pure TCP CONNECT
# tunnel that never terminates TLS itself, so it needs zero CA
# certificates, zero shell, zero OS at all - the smallest possible
# RootGuard image, and the only one that needs neither a `USER`
# workaround comment nor an AVD-DS-0002/0026 .trivyignore.yaml
# suppression (both checks are satisfied for real, not suppressed).
FROM scratch

ARG VERSION=dev
ARG COMMIT=unknown
ARG CREATED=unknown
LABEL org.opencontainers.image.title="RootGuard Attestation Proxy" \
      org.opencontainers.image.description="Minimal CONNECT-only egress proxy so Core/Updater's cosign attestation checks can reach Sigstore/GHCR from an otherwise fully internet-isolated network" \
      org.opencontainers.image.source="https://github.com/foxly-it/rootguard" \
      org.opencontainers.image.licenses="AGPL-3.0-or-later" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${CREATED}"

COPY --from=builder /rootguard-attestation-proxy /rootguard-attestation-proxy

# Fixed non-root numeric UID - scratch has no /etc/passwd, but a numeric
# UID needs no user database entry to be honored. No Docker socket, no
# host mounts, no privilege beyond binding the one port below.
USER 65534:65534

EXPOSE 8888
HEALTHCHECK --interval=15s --timeout=3s --retries=5 \
    CMD ["/rootguard-attestation-proxy", "healthcheck"]
ENTRYPOINT ["/rootguard-attestation-proxy"]
