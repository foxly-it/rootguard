# Digest-pinned, not just tag-pinned - see the backend-builder stage
# below for why this applies to a builder stage too, even though it
# never ships in the final image.
FROM node:22-bookworm@sha256:8a34c4ab3ea2c5cd194f07e317b2a8f09461d3c8b05c4e34c8ccd56d56024c4d AS frontend-builder

WORKDIR /frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Digest-pinned, not just tag-pinned - found in review: two builds of the
# same commit could otherwise compile against different toolchain
# content if the tag moved between them (a supply-chain risk even for a
# stage that never ships - it's what compiled the final binary), and a
# compromised/re-pushed upstream tag would be pulled with no change to
# this file. Same reasoning already applied to the runtime stage's own
# gcr.io/distroless base a few lines down.
FROM golang:1.26-bookworm@sha256:d9c68c2c51161e12fd77e4c6320687c9cd86e1af1e3ad6e6cd63ff970641453c AS backend-builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG TARGETARCH

WORKDIR /app
COPY backend/go.mod backend/go.sum* ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend-builder /frontend/dist ./web
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build \
    -ldflags="-s -w \
      -X main.version=${VERSION} \
      -X main.commit=${COMMIT}" \
    -o rootguard-webapp \
    ./cmd/rootguard-webapp

# static, not base (#695): the binary is CGO_ENABLED=0 and statically
# linked, so base-debian12's glibc/libssl3/OpenSSL were never loaded - only
# scanner surface (CVE-2026-84782 needed a suppression for exactly that).
# static still ships CA certificates, tzdata and the nonroot user.
FROM gcr.io/distroless/static-debian12@sha256:d75cdd72874d4790092fcb1b058493ecf6bb5bf2b2b897045b00ff01d91843f2

# ARGs don't cross FROM boundaries - re-declared here (already used above
# for stage 2's -X main.version/-X main.commit ldflags) so the same build
# invocation's values also end up as this stage's OCI labels.
ARG VERSION=dev
ARG COMMIT=unknown
ARG CREATED=unknown
LABEL org.opencontainers.image.title="RootGuard WebApp" \
      org.opencontainers.image.description="Authenticated backend proxy and React UI for RootGuard" \
      org.opencontainers.image.source="https://github.com/foxly-it/rootguard" \
      org.opencontainers.image.licenses="AGPL-3.0-or-later" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${CREATED}"

WORKDIR /app
COPY --from=backend-builder /app/rootguard-webapp .
COPY --from=backend-builder /app/web ./web

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/rootguard-webapp"]
