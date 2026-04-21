# go-note

`go-note` 是一个提供笔记管理功能的微服务。它作为核心业务服务之一，同时提供 gRPC 与 HTTP 两种入口，但业务契约以 proto 为单一真相源：标准业务接口通过 gRPC 实现，再由 `grpc-gateway` 暴露 HTTP；少量 HTTP-only 场景（如 upload / public share / group tree）保留手写路由。

## 架构

```text
浏览器 → api-gateway → grpc-gateway → go-note gRPC
                        └──────────────→ go-note HTTP-only 路由
内部服务 → go-note gRPC Server → 获取 / 操作笔记数据
```

- **协议**：一个主进程同时监听 HTTP 与 gRPC；标准业务 HTTP 通过 `grpc-gateway` 自动翻译
- **认证**：网关向 go-note 透传 `X-User-Id` / gRPC metadata，go-note 不重复做 JWT 验签
- **其它依赖**：通过 gRPC 调用 id-generator 服务进行 ID 生成
- **ORM**：ent（与 user-platform 一致）
- **配置**：Viper + godotenv（env-first）
- **共享模块**：`github.com/luckysxx/common`（logger、errs、postgres、redis 连接池、otel 等）

## 技术栈

- Go、Gin、ent、PostgreSQL、Redis
- gRPC、grpc-gateway、Gin
- Viper（配置管理）
- `common` 基础设施支持（含 Redis 与 PostgreSQL 统一连接池及监控、OpenTelemetry链路追踪）

## 目录结构

```text
go-note/
├── cmd/
│   ├── server/main.go            # 推荐主入口：同时启动 HTTP + gRPC
│   ├── http/main.go              # 过渡期旧 HTTP 入口
│   └── grpc/main.go              # 过渡期旧 gRPC 入口
├── .env.example                  # 环境变量模板
├── internal/
│   ├── ent/schema/*              # Ent Schema
│   ├── platform/{config,database,idgen,storage}
│   ├── repository/*              # 仓储层
│   ├── service/*                 # 业务服务层
│   └── transport/
│       ├── grpc/                 # gRPC Server 与 interceptor
│       └── http/
│           ├── server/router     # 仅 HTTP-only 路由
│           ├── server/handler    # upload / public share / group tree
│           └── codec/*           # response / errs / validator 等
├── docker-compose.yaml
├── Dockerfile.server
└── Makefile
```

## 快速开始

### 前置条件

- Go 1.25+
- PostgreSQL、Redis（可通过根目录 docker-compose 启动基础设施）
- id-generator gRPC 服务运行在 `localhost:50059`

### 配置

```bash
cp .env.example .env
# 编辑 .env，设置数据库、Redis 和下游服务地址
```

### 运行

```bash
# 启动单主进程（推荐）
make run

# 如需兼容旧入口
make run-http
make run-grpc

# 编译后运行
make build
make build-http
make build-grpc
./bin/go-note
./bin/go-note-http
./bin/go-note-grpc
```

### 常用命令

```bash
make help           # 查看全部命令
make run            # 启动单主进程服务
make run-http       # 启动旧 HTTP 服务
make run-grpc       # 启动旧 gRPC 服务
make build          # 编译单主进程二进制
make build-http     # 编译旧 HTTP 二进制
make build-grpc     # 编译 gRPC 二进制
make test           # 运行测试
make ent-generate   # 重新生成 Ent 代码
make lint           # 代码检查
```

## API 端点

通过 `api-gateway` 访问时，标准业务接口会统一走 `/api/v1/notes/*`。典型入口包括：

| 方法 | 路径                 | 说明                 |
|------|----------------------|----------------------|
| GET  | `/healthz`             | 存活探针             |
| GET  | `/readyz`              | 就绪探针             |
| POST | `/api/v1/notes/snippets`     | 创建笔记       |
| GET  | `/api/v1/notes/snippets/:id` | 获取笔记       |
| PUT  | `/api/v1/notes/snippets/:id` | 更新笔记       |
| GET  | `/api/v1/notes/me/snippets`  | 获取我的笔记列表 |
| POST | `/api/v1/notes/uploads`      | multipart 上传 |
| GET  | `/api/v1/notes/public/shares/:token` | 公开分享 |

## 服务端口

| 服务               | 端口  |
|--------------------|-------|
| go-note HTTP       | 8080  |
| go-note gRPC       | 9093  |
| probe/metrics      | HTTP 同进程注册 |
| id-generator gRPC  | 50059 |
