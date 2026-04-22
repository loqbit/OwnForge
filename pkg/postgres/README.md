# common/postgres — PostgreSQL 连接初始化

带 OTel 追踪和连接池的 `*sql.DB` 初始化。

## 用法

```go
import "github.com/ownforge/ownforge/pkg/postgres"

db, err := postgres.Init(
    postgres.Config{
        Driver: "postgres",
        Source: "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
    },
    postgres.DefaultPoolConfig(), // 生产级默认连接池参数
    log,
)
if err != nil {
    // go-chat: 降级为内存仓储
    // user-platform: 直接 Fatal
}
defer db.Close()
```

## 连接池配置

```go
pool := postgres.PoolConfig{
    MaxOpenConns:    25,             // 最大连接数
    MaxIdleConns:    10,             // 最大空闲连接
    ConnMaxLifetime: 30 * time.Minute,
    ConnMaxIdleTime: 5 * time.Minute,
}
```

## 设计决策

返回 `error` 而非 `log.Fatal`，将「连接失败时是降级还是退出」的决策权交给调用方。
