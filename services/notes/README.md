# go-note

`go-note` is a microservice for note management. As one of the core business services, it provides both gRPC and HTTP entrypoints, while keeping proto as the single source of truth for business contracts: standard business APIs are implemented over gRPC and exposed over HTTP through `grpc-gateway`, while a few HTTP-only cases such as upload, public share, and group tree remain handwritten routes.

## Architecture

```text
Browser -> api-gateway -> grpc-gateway -> go-note gRPC
                        └──────────────-> go-note HTTP-only routes
Internal services -> go-note gRPC Server -> query / modify note data
```

- **Protocol**: A single process listens on both HTTP and gRPC; standard business HTTP is translated automatically through `grpc-gateway`
- **Authentication**: The gateway forwards `X-User-Id` / gRPC metadata to go-note, and go-note does not validate JWT again
- **Other dependencies**: ID generation is handled by calling the `id-generator` service over gRPC
- **ORM**: Ent, consistent with `user-platform`
- **Configuration**: Viper + godotenv, with an environment-first approach
- **Shared modules**: `github.com/luckysxx/common` (logger, errs, postgres, Redis pools, OTel, and related infrastructure)

## Tech Stack

- Go, Gin, Ent, PostgreSQL, Redis
- gRPC, grpc-gateway
- Viper for configuration management
- Shared infrastructure support from `common`, including Redis and PostgreSQL pools, monitoring, and OpenTelemetry tracing

## Directory Structure

```text
go-note/
├── cmd/
│   ├── server/main.go            # recommended main entrypoint: start both HTTP and gRPC
│   ├── http/main.go              # legacy HTTP entrypoint kept during the transition
│   └── grpc/main.go              # legacy gRPC entrypoint kept during the transition
├── .env.example                  # environment variable template
├── internal/
│   ├── ent/schema/*              # Ent schema
│   ├── platform/{config,database,idgen,storage}
│   ├── repository/*              # repository layer
│   ├── service/*                 # service layer
│   └── transport/
│       ├── grpc/                 # gRPC server and interceptors
│       └── http/
│           ├── server/router     # HTTP-only routes
│           ├── server/handler    # upload / public share / group tree
│           └── codec/*           # response / errs / validator and related codecs
├── docker-compose.yaml
├── Dockerfile.server
└── Makefile
```

## Quick Start

### Prerequisites

- Go 1.25+
- PostgreSQL and Redis (infrastructure can be started through the root `docker-compose`)
- The `id-generator` gRPC service running on `localhost:50059`

### Configuration

```bash
cp .env.example .env
# Edit `.env` and set the database, Redis, and downstream service addresses
```

### Run

```bash
# Start the single-process entrypoint (recommended)
make run

# For compatibility with legacy entrypoints
make run-http
make run-grpc

# Build and run
make build
make build-http
make build-grpc
./bin/go-note
./bin/go-note-http
./bin/go-note-grpc
```

### Common Commands

```bash
make help           # show all commands
make run            # start the single-process service
make run-http       # start the legacy HTTP service
make run-grpc       # start the legacy gRPC service
make build          # build the single-process binary
make build-http     # build the legacy HTTP binary
make build-grpc     # build the gRPC binary
make test           # run tests
make ent-generate   # regenerate Ent code
make lint           # run code checks
```

## API Endpoints

When accessed through `api-gateway`, standard business APIs are routed uniformly under `/api/v1/notes/*`. Typical entrypoints include:

| Method | Path | Description |
|------|------|------|
| GET  | `/healthz` | liveness probe |
| GET  | `/readyz` | readiness probe |
| POST | `/api/v1/notes/snippets` | create note |
| GET  | `/api/v1/notes/snippets/:id` | get note |
| PUT  | `/api/v1/notes/snippets/:id` | update note |
| GET  | `/api/v1/notes/me/snippets` | get my notes list |
| POST | `/api/v1/notes/uploads` | multipart upload |
| GET  | `/api/v1/notes/public/shares/:token` | public share |

## Service Ports

| Service | Port |
|------|------|
| go-note HTTP | 8080 |
| go-note gRPC | 9093 |
| probe/metrics | registered in the same HTTP process |
| id-generator gRPC | 50059 |
