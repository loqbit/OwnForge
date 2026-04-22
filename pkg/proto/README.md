# common/proto — gRPC Proto 定义

所有微服务共享的 Protocol Buffers 定义和生成的 Go 代码。

## 目录结构

```
proto/
├── idgen/   # ID Generator 服务
│   ├── idgen.proto
│   ├── idgen.pb.go
│   └── idgen_grpc.pb.go
├── auth/    # 认证服务
│   ├── auth.proto
│   ├── auth.pb.go
│   └── auth_grpc.pb.go
├── user/    # 用户服务
│   ├── user.proto
│   ├── user.pb.go
│   └── user_grpc.pb.go
└── note/    # 笔记服务
    ├── note.proto
    ├── note.pb.go
    └── note_grpc.pb.go
```

## 服务定义

| 包 | 服务 | 说明 |
|----|------|------|
| `idgen` | `IDGeneratorService` | 分布式 ID 生成（雪花算法） |
| `auth` | `AuthService` | 登录、Token 刷新、登出 |
| `user` | `UserService` | 注册、个人资料管理 |
| `note` | `NoteService` | 代码片段 CRUD、分组、标签 |

## 用法

```go
import pb "github.com/ownforge/ownforge/pkg/proto/idgen"

// 客户端调用
resp, err := client.NextID(ctx, &pb.NextIDRequest{})
fmt.Println(resp.Id) // 雪花算法 ID
```

## 重新生成

```bash
protoc --go_out=. --go-grpc_out=. proto/idgen/idgen.proto
```
