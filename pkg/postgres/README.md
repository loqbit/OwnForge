# common/postgres — PostgreSQL connection initialization

Initialize `*sql.DB` with OTel tracing and connection pooling.

## Usage

```go
import "github.com/loqbit/ownforge/pkg/postgres"

db, err := postgres.Init(
    postgres.Config{
        Driver: "postgres",
        Source: "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
    },
    postgres.DefaultPoolConfig(), // production-grade default pool parameters
    log,
)
if err != nil {
    // go-chat: degrade to in-memory storage
    // user-platform: exit with log.Fatal
}
defer db.Close()
```

## Connection Pool Configuration

```go
pool := postgres.PoolConfig{
    MaxOpenConns:    25,             // maximum open connections
    MaxIdleConns:    10,             // maximum idle connections
    ConnMaxLifetime: 30 * time.Minute,
    ConnMaxIdleTime: 5 * time.Minute,
}
```

## Design Decision

Return `error` rather than `log.Fatal`, so the caller decides whether to degrade or exit when connection setup fails.
