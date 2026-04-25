# syntax=docker/dockerfile:1
# Build context is the repo root (see docker-compose.yml).
FROM golang:1.25-alpine AS builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,https://goproxy.io,direct

COPY pkg/ ./pkg/
COPY services/gateway/go.mod services/gateway/go.sum ./services/gateway/

WORKDIR /app/services/gateway
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY services/gateway/ ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o /out/api_gateway ./cmd/server

FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata \
    && addgroup -S appgroup && adduser -S appuser -G appgroup

ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /out/api_gateway .

USER appuser

EXPOSE 8000
CMD ["./api_gateway"]
