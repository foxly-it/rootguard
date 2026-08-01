FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /rootguard-core ./cmd/rootguard

FROM ghcr.io/sigstore/cosign/cosign:v3.0.6@sha256:de9c65609e6bde17e6b48de485ee788407c9502fa08b8f4459f595b21f56cd00 AS cosign

FROM docker:29-cli

RUN apk add --no-cache docker-cli-compose \
    && mkdir -p /var/lib/rootguard/unbound

COPY --from=builder /rootguard-core /usr/local/bin/rootguard-core
COPY --from=cosign /ko-app/cosign /usr/local/bin/cosign

EXPOSE 8081
ENTRYPOINT ["rootguard-core"]
