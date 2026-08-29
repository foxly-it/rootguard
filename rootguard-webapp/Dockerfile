# =====================================================
# RootGuard WebApp - Multi-Stage Production Dockerfile
# Location: rootguard-webapp/Dockerfile
# =====================================================
#
# Features:
# - Multi-arch compatible (amd64 / arm64)
# - Injects build metadata (version + commit)
# - Builds frontend (Vite)
# - Builds backend (Go)
# - Uses distroless runtime
# - Non-root execution
#
# =====================================================


# =====================================================
# Stage 1 – Frontend Builder
# =====================================================
# Digest-pinned, not just tag-pinned - see the backend-builder stage
# below for why this applies to a builder stage too, even though it
# never ships in the final image.
FROM node:22-bookworm@sha256:8a34c4ab3ea2c5cd194f07e317b2a8f09461d3c8b05c4e34c8ccd56d56024c4d AS frontend-builder

# Arbeitsverzeichnis
WORKDIR /frontend

# Nur Dependency Files kopieren (Layer Caching!)
COPY frontend/package.json frontend/package-lock.json ./

# Install production deps
RUN npm ci

# Restliches Frontend kopieren
COPY frontend/ ./

# Production Build (Output: /frontend/dist)
RUN npm run build



# =====================================================
# Stage 2 – Backend Builder
# =====================================================
# Digest-pinned, not just tag-pinned - found in review: two builds of the
# same commit could otherwise compile against different toolchain
# content if the tag moved between them (a supply-chain risk even for a
# stage that never ships - it's what compiled the final binary), and a
# compromised/re-pushed upstream tag would be pulled with no change to
# this file. Same reasoning already applied to the runtime stage's own
# gcr.io/distroless base a few lines down.
FROM golang:1.26-bookworm@sha256:e8c859f5632dcfde7b32d2012b4351728f6437930887c2f6a91ea242459e5514 AS backend-builder

# -----------------------------------------------------
# Build Arguments (werden von docker build übergeben)
# -----------------------------------------------------
ARG VERSION=dev
ARG COMMIT=unknown

# Multi-Arch Support (wird automatisch von buildx gesetzt)
ARG TARGETARCH

WORKDIR /app

# Go Module Files zuerst kopieren (Cache!)
COPY backend/go.mod backend/go.sum* ./

# Module Download
RUN go mod download

# Backend Source kopieren
COPY backend/ ./

# Frontend Build Artefakte übernehmen
COPY --from=frontend-builder /frontend/dist ./web

# -----------------------------------------------------
# Go Binary bauen mit Build Metadata
# -----------------------------------------------------
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build \
    -ldflags="-s -w \
      -X main.version=${VERSION} \
      -X main.commit=${COMMIT}" \
    -o rootguard-webapp \
    ./cmd/rootguard-webapp



# =====================================================
# Stage 3 – Minimal Runtime (Distroless)
# =====================================================
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

# Binary kopieren
COPY --from=backend-builder /app/rootguard-webapp .

# Statische Web Assets kopieren
COPY --from=backend-builder /app/web ./web

# Container Port
EXPOSE 8080

# Sicherheit: kein root
USER nonroot:nonroot

# Start Command
ENTRYPOINT ["/app/rootguard-webapp"]