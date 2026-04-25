# common/rpc — gRPC Client Utilities

Wraps common gRPC client initialization and currently provides an ID Generator client.

## ID Generator Client

```go
import "github.com/loqbit/ownforge/pkg/rpc"

// Initialize the global client, typically once in main
if err := rpc.InitIDGenClient("id-generator:50052"); err != nil {
    log.Fatal("failed to initialize ID generator", zap.Error(err))
}

// Generate distributed unique IDs in business code
id, err := rpc.GenerateID(ctx)
```

## Notes

- `InitIDGenClient` creates a global singleton connection and only needs to be called once per process
- `GenerateID` is concurrency-safe and can be called from any goroutine
