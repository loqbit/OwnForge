# syntax=docker/dockerfile:1
# Build context is the repo root (see docker-compose.yml).
FROM docker.m.daocloud.io/library/golang:1.25-alpine AS builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,https://goproxy.io,direct

COPY pkg/ ./pkg/
COPY services/user-platform/go.mod services/user-platform/go.sum ./services/user-platform/

WORKDIR /app/services/user-platform
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY services/user-platform/ ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o /out/user_platform ./cmd/server

FROM docker.m.daocloud.io/library/alpine:3.21

RUN apk --no-cache add ca-certificates tzdata wget \
    && addgroup -S appgroup && adduser -S appuser -G appgroup

ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /out/user_platform .

USER appuser

EXPOSE 8081 9091
CMD ["./user_platform"]
