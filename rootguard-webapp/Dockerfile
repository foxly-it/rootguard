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
FROM golang:1.26-bookworm@sha256:9fdc884aacc3bec89b20ffc69f4bb369c78210e3e4f600387b5128b12c199f81 AS backend-builder

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

FROM gcr.io/distroless/base-debian12@sha256:62730825d3cf03571e0a1b8f014748de94d0404500f063593b614c23da38841d

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
