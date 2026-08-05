FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /rootguard-updater .

FROM docker:29-cli

ARG VERSION=dev
ARG COMMIT=unknown
LABEL org.opencontainers.image.title="RootGuard Updater" \
      org.opencontainers.image.description="Isolated control-plane update and rollback helper for RootGuard" \
      org.opencontainers.image.source="https://github.com/foxly-it/rootguard-updater" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.licenses="AGPL-3.0-or-later"

RUN apk add --no-cache docker-cli-compose ca-certificates
COPY --from=builder /rootguard-updater /usr/local/bin/rootguard-updater

EXPOSE 8082
ENTRYPOINT ["rootguard-updater"]
