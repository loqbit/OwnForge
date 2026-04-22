# ID Generator Service

Distributed unique ID generation service. It is based on the Twitter Snowflake algorithm and exposes high-performance, globally unique ID generation over gRPC.

## Tech Stack

- **Go 1.25+** / **Viper** for configuration management / **Zap** for structured logging
- **gRPC** + **Protobuf** for high-performance communication
- **Snowflake** algorithm (millisecond timestamp + node ID + sequence number)
- **Docker** for containerized deployment

## Directory Structure

```text
├── cmd/idgen/main.go                  # main entrypoint: config loading, gRPC registration, and graceful shutdown
├── internal/
│   ├── idgen/snowflake.go             # core Snowflake implementation
│   └── platform/config/config.go      # Viper config structs plus godotenv loading
├── .env.example                       # environment variable template
├── .env                               # environment variables (do not commit; see `.env.example`)
└── docker-compose.yaml                # container orchestration
```

## Quick Start

### 1. Configure Environment Variables

```bash
cp .env.example .env
```

### 2. Run Locally

```bash
go mod tidy
go run cmd/idgen/main.go
```

The service listens on `:50059` by default.

### 3. Docker Deployment

```bash
docker-compose up -d --build

# View logs
docker logs -f id-generator
```

## Configuration

### .env

| Variable | Description |
|------|------|
| `APP_ENV` | runtime environment, affects log coloring |
| `SERVER_PORT` | gRPC listen port |
| `SERVER_MODE` | runtime mode |
| `SNOWFLAKE_NODE_ID` | Snowflake node ID; each instance must use a unique value in clustered deployments |

## gRPC Interface

```protobuf
service IDGenerator {
  rpc NextID(NextIDRequest) returns (NextIDResponse);
}
```

Call `NextID` to get a globally unique `int64` ID.
