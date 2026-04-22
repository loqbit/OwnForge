# common/rpc — gRPC 客户端工具

封装常用的 gRPC 客户端初始化，目前提供 ID Generator 客户端。

## ID Generator 客户端

```go
import "github.com/ownforge/ownforge/pkg/rpc"

// 初始化全局客户端（通常在 main 中调用一次）
if err := rpc.InitIDGenClient("id-generator:50052"); err != nil {
    log.Fatal("初始化 ID 生成器失败", zap.Error(err))
}

// 在业务代码中生成分布式唯一 ID
id, err := rpc.GenerateID(ctx)
```

## 注意事项

- `InitIDGenClient` 创建全局单例连接，进程生命周期内只需调用一次
- `GenerateID` 是并发安全的，可在任意 goroutine 中调用
