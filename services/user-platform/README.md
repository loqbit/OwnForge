# User Platform

Go 微服务中台，提供统一账号注册、多应用登录鉴权、Session 管理，同时暴露 HTTP 和 gRPC 两种协议。

## 核心特性

- **账号体系**：手机号必填注册、密码加密、用户名唯一约束
- **双登录模式**：保留用户名/邮箱 + 密码登录，同时新增手机号验证码登录
- **多应用鉴权**：携带 `app_code` 登录，签发 Access Token + Refresh Token，首次登录自动建立授权关系
- **Session 管理**：设备级 Session 追踪、登出、Token 轮换刷新
- **事件驱动**：Transactional Outbox 模式保证注册事件可靠投递至 Kafka
- **数据同步**：结合 Debezium CDC 实现 PostgreSQL 变更数据非侵入式同步至下游
- **基础设施**：基于 `common` 组件库实现 Redis / PostgreSQL 统一连接池管理与 OTel 链路追踪
- **双协议**：HTTP (Gin) + gRPC (grpc-go) 共享同一套 Service 层

## 技术栈

| 层 | 技术 |
|---|------|
| Web 框架 | Gin (HTTP) / grpc-go (gRPC) |
| ORM | Ent |
| 数据库 | PostgreSQL |
| 缓存 | Redis |
| 消息队列 | Kafka (segmentio/kafka-go) |
| 配置 | Viper + godotenv |
| 日志 | Zap (结构化 + 彩色) |
| ID 生成 | 远程 Snowflake (gRPC) |
| 容器化 | Docker + Docker Compose |
| 可观测 | Prometheus + Grafana + Loki |
| 实时同步 | Debezium (CDC) + PostgreSQL 逻辑复制 |

## 项目结构

```text
├── cmd/
│   ├── http/main.go              # HTTP 入口
│   └── grpc/main.go              # gRPC 入口
├── internal/
│   ├── service/                   # 业务逻辑（注册、登录、鉴权）
│   ├── repository/                # 数据访问（User、Session、EventOutbox）
│   ├── transport/
│   │   ├── http/                  # Gin 路由 + Handler
│   │   └── grpc/                  # gRPC Server 实现
│   ├── ent/                       # Ent 生成代码 + Schema
│   └── platform/
│       ├── config/                # Viper 配置加载
│       ├── database/              # PostgreSQL 初始化
│       └── cache/                 # Redis 初始化
├── .env.example                   # 环境变量模板
├── .env                           # 敏感凭证（不提交，见 .env.example）
└── docker-compose-service.yaml    # 服务编排
```

## 快速开始

### 1. 配置环境变量
```bash
cp .env.example .env
# 编辑 .env，填入数据库连接、Redis 密码、JWT 密钥
```

### 2. 启动基础设施
```bash
make local-infra-up   # 启动 PostgreSQL + Redis + Kafka
```

### 3. 本地运行
```bash
make local-run-http   # HTTP 服务 :8081
make local-run-grpc   # gRPC 服务 :9091
```

### 4. Docker 一键部署
```bash
make docker-up        # 构建并启动全部容器
make docker-logs      # 查看日志
make docker-down      # 停止并清理
```

## 配置说明

### .env（运行配置与敏感信息）
| 变量 | 说明 |
|------|------|
| `APP_ENV` | 运行环境，影响日志颜色 |
| `SERVER_PORT` | HTTP 监听端口 |
| `GRPC_SERVER_PORT` | gRPC 监听端口 |
| `DATABASE_SOURCE` | PostgreSQL 连接字符串 |
| `REDIS_PASSWORD` | Redis 密码 |
| `JWT_SECRET` | JWT 签名密钥 |
| `KAFKA_BROKERS` | Kafka 地址 |
| `ID_GENERATOR_ADDR` | 发号器 gRPC 地址 |

## HTTP API

Base URL: `http://localhost:8081/api/v1`

```bash
# 注册
curl -X POST localhost:8081/api/v1/users/register \
  -H 'Content-Type: application/json' \
  -d '{"phone":"13800138000","email":"alice@example.com","username":"alice123","password":"Password123"}'

# 登录
curl -X POST localhost:8081/api/v1/users/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice123","password":"Password123","app_code":"go-note","device_id":"macbook-user-1"}'

# 发送手机号验证码（开发环境会在 debug_code 返回验证码，便于 curl 联调）
curl -X POST localhost:8081/api/v1/users/phone/code \
  -H 'Content-Type: application/json' \
  -d '{"phone":"13800138000","scene":"login"}'

# 手机号验证码登录/注册一体化入口
curl -X POST localhost:8081/api/v1/users/phone/entry \
  -H 'Content-Type: application/json' \
  -d '{"phone":"13800138000","verification_code":"123456","app_code":"go-note","device_id":"macbook-user-1"}'

# 刷新 Token
curl -X POST localhost:8081/api/v1/users/refresh \
  -H 'Content-Type: application/json' \
  -d '{"token":"<refresh_token>"}'

# 登出
curl -X POST localhost:8081/api/v1/users/logout \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <access_token>' \
  -d '{"device_id":"macbook-user-1"}'
```

## gRPC API

地址：`localhost:9091`

```bash
# 注册
grpcurl -plaintext -d '{"phone":"13800138000","email":"alice@example.com","username":"alice123","password":"Password123"}' \
  localhost:9091 user.UserService/Register

# 登录
grpcurl -plaintext -d '{"username":"alice123","password":"Password123","app_code":"go-note","device_id":"macbook-user-1"}' \
  localhost:9091 user.AuthService/Login

# 发送手机号验证码
grpcurl -plaintext -d '{"phone":"13800138000","scene":"login"}' \
  localhost:9091 user.AuthService/SendPhoneCode

# 手机号验证码登录/注册
grpcurl -plaintext -d '{"phone":"13800138000","verification_code":"123456","app_code":"go-note","device_id":"macbook-user-1"}' \
  localhost:9091 user.AuthService/PhoneAuthEntry
```

## 架构亮点

### Transactional Outbox 模式
注册时在同一个数据库事务中写入 `users` 表和 `event_outboxes` 表，由 Debezium CDC 监听 Outbox 表并转发 Kafka，保证数据一致性与低业务侵入。

### Kafka Topic 命名约定
统一使用全小写 + 点分隔风格，例如 `user.registered`、`user.deleted`。不要混用 `UserRegistered`、`user_registered` 这类 PascalCase 或下划线风格，避免 Kafka 因自动建 Topic 生成多份语义重复的主题。

### 事件契约治理
共享事件契约统一收敛在 `common/mq` 中。事件类型与 Topic 统一使用全小写 + 点分隔风格，便于 Outbox、Debezium 和下游消费者保持一致语义；共享事件消息体建议显式携带 `version` 字段，便于后续平滑演进。

### Bootstrap 三段式入口
`main.go` 采用 `initInfra` → `buildRouter` → `runServer` 三段式组织，基础设施初始化、依赖注入、服务启动职责清晰分离。

## 常用 Make 命令

| 命令 | 说明 |
|------|------|
| `make local-infra-up` | 启动本地基础设施 |
| `make local-run-http` | 本地启动 HTTP |
| `make local-run-grpc` | 本地启动 gRPC |
| `make docker-up` | Docker 一键部署 |
| `make docker-down` | 停止并清理容器 |
| `make docker-logs` | 查看服务日志 |
| `make proto-gen` | 重新生成 Protobuf |
| `make health` | 健康检查 |

## License

仅用于学习与内部开发。
