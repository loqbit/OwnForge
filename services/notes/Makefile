SHELL := /bin/bash

NETWORK_EXTERNAL = go-net

.PHONY: help run build test lint ent-generate docker-up docker-down docker-build clean

init-networks:
	@docker network inspect $(NETWORK_EXTERNAL) >/dev/null 2>&1 || docker network create $(NETWORK_EXTERNAL)

help:
	@echo "Available targets:"
	@echo "  make run              # start the go-note service (HTTP + gRPC)"
	@echo "  make run-ai-worker    # start the AI worker"
	@echo "  make build            # build the go-note binary"
	@echo "  make build-ai-worker  # build the AI worker binary"
	@echo "  make test             # run all tests"
	@echo "  make ent-generate     # regenerate Ent code"
	@echo "  make lint             # run go vet and build checks"
	@echo "  make docker-up        # start Docker Compose"
	@echo "  make docker-down      # stop Docker Compose"
	@echo "  make docker-build     # build and start Docker Compose"
	@echo "  make clean            # clean build artifacts"

# ==========================================
# Development
# ==========================================

run:
	@go run ./cmd/server/main.go

run-ai-worker:
	@go run ./cmd/ai-worker/main.go

build:
	@go build -o bin/go-note ./cmd/server/main.go
	@echo "Binary built: bin/go-note"

build-ai-worker:
	@go build -o bin/ai-worker ./cmd/ai-worker/main.go
	@echo "Binary built: bin/ai-worker"

test:
	@go test ./internal/...

lint:
	@go vet ./...
	@go build ./...
	@echo "Lint passed"

# ==========================================
# Code generation
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
# Cleanup
# ==========================================

clean:
	@rm -rf bin/
	@echo "Cleaned"
