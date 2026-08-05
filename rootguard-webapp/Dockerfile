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
FROM node:22-bookworm AS frontend-builder

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
FROM golang:1.26-bookworm AS backend-builder

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
FROM gcr.io/distroless/base-debian12:latest

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