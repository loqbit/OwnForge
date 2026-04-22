# common/redis — Redis 客户端初始化

统一的 Redis 客户端创建，内置连接池和日志。

## 用法

```go
import commonRedis "github.com/ownforge/ownforge/pkg/redis"

redisClient := commonRedis.Init(commonRedis.Config{
    Addr:     "localhost:6379",
    Password: "123456",
    DB:       0,
}, log)
defer redisClient.Close()

// 返回标准 *redis.Client，直接使用 go-redis API
redisClient.Set(ctx, "key", "value", time.Minute)
val, err := redisClient.Get(ctx, "key").Result()
```

## Config 字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `Addr` | string | Redis 地址，如 `"localhost:6379"` |
| `Password` | string | 密码 |
| `DB` | int | 数据库编号 |
