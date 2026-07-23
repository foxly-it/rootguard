FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /rootguard-updater .

FROM docker:29-cli

RUN apk add --no-cache docker-cli-compose ca-certificates
COPY --from=builder /rootguard-updater /usr/local/bin/rootguard-updater

EXPOSE 8082
ENTRYPOINT ["rootguard-updater"]
