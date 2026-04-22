# API Gateway

`api-gateway` 是整个微服务系统的统一入口，负责鉴权、限流、观测与请求转发，对外暴露稳定的 HTTP API，对内对接 `user-platform` 和 `go-note` 等服务。

## Core Features

- JWT 鉴权与用户上下文透传
- 基于 Redis 的多层限流
- Gin + OpenTelemetry + Prometheus 指标采集
- 统一日志与崩溃恢复中间件
- 通过配置将请求路由到下游 HTTP / gRPC 服务

## Directory Layout

```text
api-gateway/
├── cmd/server/                    # 服务启动入口
├── .env.example                   # 环境变量模板
├── internal/
│   ├── auth/                      # JWT 工具
│   ├── config/                    # 配置加载
│   ├── grpcclient/                # 下游 gRPC 客户端
│   ├── handler/                   # HTTP Handler、DTO、参数校验
│   ├── middleware/                # 日志、鉴权、限流中间件
│   ├── proxy/                     # 反向代理封装
│   └── restclient/                # 下游 REST 客户端
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

默认监听环境变量中的服务端口。

### Docker

```bash
docker-compose up -d --build
```

## Configuration

- 运行配置统一通过环境变量注入
- 本地开发请从 `.env.example` 复制出 `.env`
- 提交代码前请确认敏感配置未写入仓库

## Git Hygiene

- `.gitignore` 已忽略 macOS 缓存、编辑器目录、构建产物和本地环境文件。
- 像 `internal/.DS_Store` 这类系统文件不应提交，清理后再做 commit。
- 新增本地调试文件时，优先确认是否属于应忽略范围，避免把临时文件带进主分支。
