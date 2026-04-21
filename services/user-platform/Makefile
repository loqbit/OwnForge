.PHONY: init-networks local-run local-run-http local-run-grpc local-test proto-gen docker-up docker-down docker-logs ps health fe-install fe-dev fe-build fe-lint fe-type-check fe-preview

NETWORK_EXTERNAL = go-net
COMPOSE_FILES = -f docker-compose.yaml
FRONTEND_DIR = view

init-networks:
	@docker network inspect $(NETWORK_EXTERNAL) >/dev/null 2>&1 || docker network create $(NETWORK_EXTERNAL)

local-run:
	go run ./cmd/http
local-run-http:
	go run ./cmd/http

local-run-grpc:
	go run ./cmd/grpc

local-test:
	go test ./...

proto-gen:
	protoc --proto_path=. --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/user/user_api.proto proto/auth/auth_api.proto

docker-up: init-networks
	docker compose $(COMPOSE_FILES) up -d --build

docker-down:
	docker compose $(COMPOSE_FILES) down

docker-logs:
	docker compose $(COMPOSE_FILES) logs -f user-http user-grpc

ps:
	docker compose $(COMPOSE_FILES) ps

health:
	docker compose $(COMPOSE_FILES) ps --format "table {{.Name}}\t{{.State}}\t{{.Health}}"
	docker compose $(COMPOSE_FILES) logs --tail=40 user-http user-grpc

fe-install:
	pnpm --dir $(FRONTEND_DIR) install

fe-dev:
	pnpm --dir $(FRONTEND_DIR) dev

fe-build:
	pnpm --dir $(FRONTEND_DIR) build

fe-lint:
	pnpm --dir $(FRONTEND_DIR) lint

fe-type-check:
	pnpm --dir $(FRONTEND_DIR) type-check

fe-preview:
	pnpm --dir $(FRONTEND_DIR) preview
