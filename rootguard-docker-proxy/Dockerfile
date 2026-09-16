# Digest-pinned, not just tag-pinned - see rootguard-core/Dockerfile's
# identical comment for why.
FROM golang:1.26-alpine@sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628 AS builder

WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /rootguard-docker-proxy .

# scratch, not alpine/distroless: this binary only ever dials a Unix
# socket (the real Docker socket) and never resolves a hostname or
# terminates TLS itself, so it needs zero CA certificates, zero shell,
# zero OS at all - same reasoning as rootguard-attestation-proxy's own
# Dockerfile.
FROM scratch

ARG VERSION=dev
ARG COMMIT=unknown
ARG CREATED=unknown
LABEL org.opencontainers.image.title="RootGuard Docker Proxy" \
      org.opencontainers.image.description="Purpose-built, filtering Docker Engine API proxy so Core/Updater no longer need direct, unrestricted access to the Docker socket" \
      org.opencontainers.image.source="https://github.com/foxly-it/rootguard" \
      org.opencontainers.image.licenses="AGPL-3.0-or-later" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${CREATED}"

COPY --from=builder /rootguard-docker-proxy /rootguard-docker-proxy

# Deliberately no USER instruction: this is the one container that holds
# the real Docker socket, and socket access requires either running as
# root or matching the host's docker-group GID, which differs per
# installation and can't be hardcoded here without breaking on hosts
# where it doesn't match - the exact same constraint documented in
# rootguard-core/Dockerfile and rootguard-updater/Dockerfile, which is
# why both of them ran as root too. The actual improvement this component
# delivers isn't "root disappears" - the socket fundamentally still needs
# a root-capable process somewhere - it's that root-capable code shrinks
# from two large, frequently-changing, feature-rich services (Core's
# whole WebGUI-driven API, Updater's update/rollback logic) down to this
# one small, single-purpose, exhaustively unit-tested request filter.
# Suppressed for this file specifically in .trivyignore.yaml (AVD-DS-0002),
# not inline, for the same reason as Core/Updater's own suppressions.
EXPOSE 2375
HEALTHCHECK --interval=15s --timeout=3s --retries=5 \
    CMD ["/rootguard-docker-proxy", "healthcheck"]
ENTRYPOINT ["/rootguard-docker-proxy"]
