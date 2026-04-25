# syntax=docker/dockerfile:1
# Build context is the repo root (see docker-compose.yml).
FROM golang:1.25-alpine AS builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,https://goproxy.io,direct

COPY pkg/ ./pkg/
COPY services/notes/go.mod services/notes/go.sum ./services/notes/

WORKDIR /app/services/notes
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY services/notes/ ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o /out/go_note_server ./cmd/server

FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata \
    && addgroup -S appgroup && adduser -S appuser -G appgroup

ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /out/go_note_server .

USER appuser

EXPOSE 8080 9093
CMD ["./go_note_server"]
