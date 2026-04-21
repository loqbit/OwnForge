# 运行方式说明

## 方式 1：本地开发

### 前置条件

- PostgreSQL 和 Redis 已启动（通过 `docker-compose-infra.yaml` 启动公共基础设施）
- id-generator 服务可访问

### 运行步骤

1. **配置环境变量**

```bash
cp .env.example .env
# 编辑 .env，设置 DATABASE_SOURCE 和 REDIS_PASSWORD
```

2. **启动服务**

```bash
make run
```

`make run` 现在默认启动 go-note 单主进程入口，同时监听：

- HTTP: `:8080`
- gRPC: `:9093`

3. **访问服务**

- go-note API: `http://localhost:8080`
- 健康检查: `http://localhost:8080/healthz`
- 就绪检查: `http://localhost:8080/readyz`

---

## 方式 2：Docker Compose

### 前置条件

- 公共基础设施已通过 `docker-compose-infra.yaml` 启动（PostgreSQL、Redis 等）
- `go-net` 网络已创建

### 运行步骤

1. **确保基础设施网络存在**

```bash
docker network inspect go-net >/dev/null 2>&1 || docker network create go-net
```

2. **在 global-postgres 中创建数据库**

```bash
docker exec global-postgres psql -U luckys -d postgres -c "CREATE DATABASE go_note;"
```

3. **启动服务**

```bash
make docker-up
```

4. **访问服务**

- go-note API: `http://localhost:8080`
- go-note gRPC: `localhost:9093`（容器网络内）

### 配置说明

Docker Compose 通过 `.env` + `environment` 注入配置，关键变量包括：

- `DATABASE_SOURCE`
- `REDIS_ADDR`
- `REDIS_PASSWORD`
- `GRPC_SERVER_PORT`

---

## 配置项

| 配置项                   | 说明               | 默认值        |
| ------------------------ | ------------------ | ------------- |
| `APP_ENV`                | 运行环境           | `development` |
| `SERVER_PORT`            | HTTP 端口          | `8080`        |
| `GRPC_SERVER_PORT`       | gRPC 端口          | `9093`        |
| `DATABASE_DRIVER`        | 数据库驱动         | `postgres`    |
| `DATABASE_SOURCE`        | 数据库连接串       | `.env` 中设置 |
| `DATABASE_AUTO_MIGRATE`  | 是否自动建表       | `.env` 中设置 |
| `REDIS_ADDR`             | Redis 地址         | `localhost:6379` |
| `REDIS_PASSWORD`         | Redis 密码         | `.env` 中设置 |
| `METRICS_PORT`           | 兼容旧 gRPC 旁路探针端口配置 | `9094` |

---

## 常见问题

### Q: 数据库表不存在

A: 检查 `.env` 中 `DATABASE_AUTO_MIGRATE` 是否开启，以及 `DATABASE_SOURCE` 是否正确。Ent 会在启动时自动创建表。

### Q: 为什么既有 8080 又有 9093

A: `8080` 是 go-note 的 HTTP 入口，承接 grpc-gateway 和少量 HTTP-only 路由；`9093` 是内部 gRPC 入口，供 api-gateway 或其他服务调用。
