# API Gateway

`api-gateway` is the unified entrypoint for the microservice system. It handles authentication, rate limiting, observability, and request forwarding, exposing a stable HTTP API externally while connecting internally to services such as `user-platform` and `go-note`.

## Core Features

- JWT authentication and user-context propagation
- Multi-layer rate limiting based on Redis
- Gin + OpenTelemetry + Prometheus metrics collection
- Unified logging and panic recovery middleware
- Request routing to downstream HTTP / gRPC services through configuration

## Directory Layout

```text
api-gateway/
├── cmd/server/                    # service startup entrypoint
├── .env.example                   # environment variable template
├── internal/
│   ├── auth/                      # JWT utilities
│   ├── config/                    # configuration loading
│   ├── grpcclient/                # downstream gRPC clients
│   ├── handler/                   # HTTP handlers, DTOs, and parameter validation
│   ├── middleware/                # logging, auth, and rate-limiting middleware
│   ├── proxy/                     # reverse-proxy wrapper
│   └── restclient/                # downstream REST clients
├── docker-compose.yaml
├── Dockerfile
└── go.mod
```

## Routes

### Public

- `POST /api/v1/users/register`
- `POST /api/v1/users/login`
- `POST /api/v1/users/refresh`

### Authenticated

- `GET /api/v1/users/dashboard`
- `GET /api/v1/users/me/profile`
- `PUT /api/v1/users/me/profile`
- `POST /api/v1/users/logout`
- `GET /api/v1/notes/me/snippets`
- `POST /api/v1/notes/snippets`
- `GET /api/v1/notes/snippets/:id`
- `PUT /api/v1/notes/snippets/:id`

## Quick Start

### Local Run

```bash
go run cmd/server/main.go
```

By default, the service listens on the port defined by environment variables.

### Docker

```bash
docker-compose up -d --build
```

## Configuration

- Runtime configuration is injected uniformly through environment variables
- For local development, copy `.env.example` to `.env`
- Before committing code, make sure sensitive configuration has not been added to the repository

## Git Hygiene

- `.gitignore` already ignores macOS cache files, editor directories, build artifacts, and local environment files
- System files like `internal/.DS_Store` should not be committed; clean them up before committing
- When adding local debug files, first check whether they should be ignored so temporary files do not end up on the main branch
