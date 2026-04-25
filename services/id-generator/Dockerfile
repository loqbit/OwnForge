# syntax=docker/dockerfile:1
# Build context is the repo root (see docker-compose.yml).
FROM golang:1.25-alpine AS builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,https://goproxy.io,direct

COPY pkg/ ./pkg/
COPY services/id-generator/go.mod services/id-generator/go.sum ./services/id-generator/

WORKDIR /app/services/id-generator
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY services/id-generator/ ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o /out/id-generator ./cmd/idgen

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/id-generator /id-generator

USER 65534

EXPOSE 50059
ENTRYPOINT ["/id-generator"]
