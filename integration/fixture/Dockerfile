FROM golang:1.26-alpine AS builder

ARG VERSION=dev
WORKDIR /src
COPY main.go .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /fixture main.go

FROM alpine:3.23
ARG FAIL_HEALTH=false
ENV FAIL_HEALTH=${FAIL_HEALTH}
COPY --from=builder /fixture /usr/local/bin/fixture
USER 65532:65532
ENTRYPOINT ["fixture"]
