SHELL := /bin/bash

NETWORK_EXTERNAL = go-net

.PHONY: help run run-http run-grpc run-server build build-http build-grpc build-server test lint ent-generate docker-up docker-down docker-build clean

init-networks:
	@docker network inspect $(NETWORK_EXTERNAL) >/dev/null 2>&1 || docker network create $(NETWORK_EXTERNAL)

help:
	@echo "Available targets:"
	@echo "  make run              # 启动 go-note 单主进程服务"
	@echo "  make run-http         # 启动 go-note 旧 HTTP 服务"
	@echo "  make run-grpc         # 启动 go-note 旧 gRPC 服务"
	@echo "  make run-server       # 启动 go-note 单主进程服务"
	@echo "  make build            # 编译单主进程二进制"
	@echo "  make build-http       # 编译旧 HTTP 二进制"
	@echo "  make build-grpc       # 编译 gRPC 二进制"
	@echo "  make build-server     # 编译单主进程二进制"
	@echo "  make test             # 运行所有测试"
	@echo "  make ent-generate     # 重新生成 Ent 代码"
	@echo "  make lint             # Go vet + build 检查"
	@echo "  make docker-up        # Docker Compose 启动"
	@echo "  make docker-down      # Docker Compose 停止"
	@echo "  make docker-build     # Docker Compose 构建并启动"
	@echo "  make clean            # 清理编译产物"

# ==========================================
# 开发
# ==========================================

run:
	@go run ./cmd/server/main.go

run-http:
	@go run ./cmd/http/main.go

run-grpc:
	@go run ./cmd/grpc/main.go

run-server:
	@go run ./cmd/server/main.go

build:
	@go build -o bin/go-note ./cmd/server/main.go
	@echo "Binary built: bin/go-note"

build-http:
	@go build -o bin/go-note-http ./cmd/http/main.go
	@echo "Binary built: bin/go-note-http"

build-grpc:
	@go build -o bin/go-note-grpc ./cmd/grpc/main.go
	@echo "Binary built: bin/go-note-grpc"

build-server:
	@go build -o bin/go-note-server ./cmd/server/main.go
	@echo "Binary built: bin/go-note-server"

test:
	@go test ./internal/...

lint:
	@go vet ./...
	@go build ./...
	@echo "Lint passed"

# ==========================================
# 代码生成
# ==========================================

ent-generate:
	@GOWORK=off go run -mod=mod entgo.io/ent/cmd/ent generate ./internal/ent/schema
	@echo "Ent code generated"

# ==========================================
# Docker
# ==========================================

docker-up: init-networks
	@docker compose up -d
	@echo "Services started"

docker-down:
	@docker compose down
	@echo "Services stopped"

docker-build: init-networks
	@docker compose up -d --build
	@echo "Services built and started"

# ==========================================
# 清理
# ==========================================

clean:
	@rm -rf bin/
	@echo "Cleaned"
