# Running Modes

## Mode 1: Local Development

### Prerequisites

- PostgreSQL and Redis are already running through `docker-compose-infra.yaml`
- The `id-generator` service is reachable

### Run Steps

1. **Configure Environment Variables**

```bash
cp .env.example .env
# Edit `.env` and set `DATABASE_SOURCE` and `REDIS_PASSWORD`
```

2. **Start the Service**

```bash
make run
```

`make run` now starts the single-process go-note entrypoint by default and listens on:

- HTTP: `:8080`
- gRPC: `:9093`

3. **Access the Service**

- go-note API: `http://localhost:8080`
- health check: `http://localhost:8080/healthz`
- readiness check: `http://localhost:8080/readyz`

---

## Mode 2: Docker Compose

### Prerequisites

- Shared infrastructure has already been started through `docker-compose-infra.yaml` (PostgreSQL, Redis, and related services)
- The `go-net` network has already been created

### Run Steps

1. **Ensure the Infrastructure Network Exists**

```bash
docker network inspect go-net >/dev/null 2>&1 || docker network create go-net
```

2. **Create the Database in `global-postgres`**

```bash
docker exec global-postgres psql -U luckys -d postgres -c "CREATE DATABASE go_note;"
```

3. **Start the Service**

```bash
make docker-up
```

4. **Access the Service**

- go-note API: `http://localhost:8080`
- go-note gRPC: `localhost:9093` (inside the container network)

### Configuration

Docker Compose injects configuration through `.env` plus `environment`. Key variables include:

- `DATABASE_SOURCE`
- `REDIS_ADDR`
- `REDIS_PASSWORD`
- `GRPC_SERVER_PORT`

---

## Configuration Items

| Configuration Item | Description | Default Value |
|------|------|------|
| `APP_ENV` | runtime environment | `development` |
| `SERVER_PORT` | HTTP port | `8080` |
| `GRPC_SERVER_PORT` | gRPC port | `9093` |
| `DATABASE_DRIVER` | database driver | `postgres` |
| `DATABASE_SOURCE` | database connection string | set in `.env` |
| `DATABASE_AUTO_MIGRATE` | whether to auto-create tables | set in `.env` |
| `REDIS_ADDR` | Redis address | `localhost:6379` |
| `REDIS_PASSWORD` | Redis password | set in `.env` |
| `METRICS_PORT` | legacy-compatible gRPC sidecar probe port | `9094` |

---

## FAQ

### Q: Database tables do not exist

A: Check whether `DATABASE_AUTO_MIGRATE` is enabled in `.env` and whether `DATABASE_SOURCE` is correct. Ent creates tables automatically on startup.

### Q: Why are there both 8080 and 9093?

A: `8080` is the go-note HTTP entrypoint that handles grpc-gateway and a small number of HTTP-only routes. `9093` is the internal gRPC entrypoint used by `api-gateway` and other services.
