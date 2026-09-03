# Pinned by digest, not just the tags - see dnssec-test-zone/setup.sh's
# identical comment on why. Found in review, round 12.
FROM golang:1.26-alpine@sha256:28d89ee9cc0ff9fec75c82ca201e6bf7fdf9a679d4b7b24dfa04f2bb766bb468 AS builder

ARG VERSION=dev
WORKDIR /src
COPY main.go .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /fixture main.go

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40
ARG FAIL_HEALTH=false
ENV FAIL_HEALTH=${FAIL_HEALTH}
COPY --from=builder /fixture /usr/local/bin/fixture
USER 65532:65532
ENTRYPOINT ["fixture"]
