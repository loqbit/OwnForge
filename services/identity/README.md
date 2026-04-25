# User Platform

A Go microservice platform that provides unified account registration, multi-application authentication, and session management, while exposing both HTTP and gRPC protocols.

## Core Features

- **Account system**: phone-required registration, password hashing, and unique-username constraints
- **Dual login modes**: keep username/email + password login while adding phone verification-code login
- **Multi-application authentication**: log in with `app_code`, issue Access Token + Refresh Token, and automatically create the authorization relation on first login
- **Session management**: device-level session tracking, logout, and token rotation
- **Event-driven architecture**: Transactional Outbox ensures reliable delivery of registration events to Kafka
- **Data synchronization**: Debezium CDC syncs PostgreSQL change data downstream with minimal intrusion into business logic
- **Infrastructure**: use the `common` component library for unified Redis / PostgreSQL pool management and OTel tracing
- **Dual protocol**: HTTP (Gin) + gRPC (grpc-go) share the same service layer

## Tech Stack

| Layer | Technology |
|---|------|
| Web framework | Gin (HTTP) / grpc-go (gRPC) |
| ORM | Ent |
| Database | PostgreSQL |
| Cache | Redis |
| Message queue | Kafka (segmentio/kafka-go) |
| Configuration | Viper + godotenv |
| Logging | Zap (structured + colored) |
| ID generation | Remote Snowflake (gRPC) |
| Containerization | Docker + Docker Compose |
| Observability | Prometheus + Grafana + Loki |
| Real-time sync | Debezium (CDC) + PostgreSQL logical replication |

## Project Structure

```text
├── cmd/
│   ├── http/main.go              # HTTP entrypoint
│   └── grpc/main.go              # gRPC entrypoint
├── internal/
│   ├── service/                  # business logic (registration, login, authentication)
│   ├── repository/               # data access (User, Session, EventOutbox)
│   ├── transport/
│   │   ├── http/                 # Gin routes + handlers
│   │   └── grpc/                 # gRPC server implementation
│   ├── ent/                      # Ent generated code + schema
│   └── platform/
│       ├── config/               # Viper config loading
│       ├── database/             # PostgreSQL initialization
│       └── cache/                # Redis initialization
├── .env.example                  # environment variable template
├── .env                          # sensitive credentials (do not commit; see `.env.example`)
└── docker-compose-service.yaml   # service orchestration
```

## Quick Start

### 1. Configure Environment Variables

```bash
cp .env.example .env
# Edit `.env` and fill in the database connection, Redis password, and JWT secret
```

### 2. Start Infrastructure

```bash
make local-infra-up   # start PostgreSQL + Redis + Kafka
```

### 3. Run Locally

```bash
make local-run-http   # HTTP service :8081
make local-run-grpc   # gRPC service :9091
```

### 4. One-Command Docker Deployment

```bash
make docker-up        # build and start all containers
make docker-logs      # view logs
make docker-down      # stop and clean up
```

## Configuration

### .env

| Variable | Description |
|------|------|
| `APP_ENV` | runtime environment, affects log coloring |
| `SERVER_PORT` | HTTP listen port |
| `GRPC_SERVER_PORT` | gRPC listen port |
| `DATABASE_SOURCE` | PostgreSQL connection string |
| `REDIS_PASSWORD` | Redis password |
| `JWT_SECRET` | JWT signing secret |
| `KAFKA_BROKERS` | Kafka address |
| `ID_GENERATOR_ADDR` | ID generator gRPC address |

## HTTP API

Base URL: `http://localhost:8081/api/v1`

```bash
# Register
curl -X POST localhost:8081/api/v1/users/register \
  -H 'Content-Type: application/json' \
  -d '{"phone":"13800138000","email":"alice@example.com","username":"alice123","password":"Password123"}'

# Login
curl -X POST localhost:8081/api/v1/users/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice123","password":"Password123","app_code":"go-note","device_id":"macbook-user-1"}'

# Send a phone verification code (in development, the code is returned in `debug_code` for easier curl testing)
curl -X POST localhost:8081/api/v1/users/phone/code \
  -H 'Content-Type: application/json' \
  -d '{"phone":"13800138000","scene":"login"}'

# Unified phone verification-code login/registration entrypoint
curl -X POST localhost:8081/api/v1/users/phone/entry \
  -H 'Content-Type: application/json' \
  -d '{"phone":"13800138000","verification_code":"123456","app_code":"go-note","device_id":"macbook-user-1"}'

# Refresh Token
curl -X POST localhost:8081/api/v1/users/refresh \
  -H 'Content-Type: application/json' \
  -d '{"token":"<refresh_token>"}'

# Logout
curl -X POST localhost:8081/api/v1/users/logout \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <access_token>' \
  -d '{"device_id":"macbook-user-1"}'
```

## gRPC API

Address: `localhost:9091`

```bash
# Register
grpcurl -plaintext -d '{"phone":"13800138000","email":"alice@example.com","username":"alice123","password":"Password123"}' \
  localhost:9091 user.UserService/Register

# Login
grpcurl -plaintext -d '{"username":"alice123","password":"Password123","app_code":"go-note","device_id":"macbook-user-1"}' \
  localhost:9091 user.AuthService/Login

# Send phone verification code
grpcurl -plaintext -d '{"phone":"13800138000","scene":"login"}' \
  localhost:9091 user.AuthService/SendPhoneCode

# Phone verification-code login/registration
grpcurl -plaintext -d '{"phone":"13800138000","verification_code":"123456","app_code":"go-note","device_id":"macbook-user-1"}' \
  localhost:9091 user.AuthService/PhoneAuthEntry
```

## Architecture Highlights

### Transactional Outbox Pattern

During registration, the `users` table and the `event_outboxes` table are written in the same database transaction. Debezium CDC listens to the Outbox table and forwards events to Kafka, preserving data consistency with minimal business-layer intrusion.

### Kafka Topic Naming Convention

Use lowercase names with dot separators consistently, such as `user.registered` and `user.deleted`. Do not mix in styles like `UserRegistered` or `user_registered`, which can cause Kafka to auto-create duplicate topics with overlapping meaning.

### Event Contract Governance

Shared event contracts are centralized in `common/mq`. Event types and topics use lowercase names with dot separators so Outbox, Debezium, and downstream consumers keep consistent semantics. Shared event payloads should explicitly carry a `version` field to support smooth future evolution.

### Three-Stage Bootstrap Entrypoint

`main.go` is organized as `initInfra` -> `buildRouter` -> `runServer`, keeping infrastructure setup, dependency injection, and service startup clearly separated.

## Common Make Commands

| Command | Description |
|------|------|
| `make local-infra-up` | start local infrastructure |
| `make local-run-http` | run HTTP locally |
| `make local-run-grpc` | run gRPC locally |
| `make docker-up` | build and start all containers |
| `make docker-down` | stop and clean up containers |
| `make docker-logs` | view service logs |
| `make proto-gen` | regenerate Protobuf |
| `make health` | health check |

## License

For learning and internal development only.
