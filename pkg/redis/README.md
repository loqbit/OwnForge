# common/redis — Redis Client Initialization

Unified Redis client creation with a built-in connection pool and logging.

## Usage

```go
import commonRedis "github.com/loqbit/ownforge/pkg/redis"

redisClient := commonRedis.Init(commonRedis.Config{
    Addr:     "localhost:6379",
    Password: "123456",
    DB:       0,
}, log)
defer redisClient.Close()

// Returns a standard `*redis.Client` and uses the go-redis API directly
redisClient.Set(ctx, "key", "value", time.Minute)
val, err := redisClient.Get(ctx, "key").Result()
```

## Config Field

| Field | Type | Description |
|------|------|------|
| `Addr` | string | Redis Address, for example `"localhost:6379"` |
| `Password` | string | Password |
| `DB` | int | Database number |
