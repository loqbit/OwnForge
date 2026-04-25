# common/proto — gRPC Proto Definitions

Protocol Buffers definitions and generated Go code shared by all microservices.

## Directory Structure

```
proto/
├── idgen/   # ID Generator Service
│   ├── idgen.proto
│   ├── idgen.pb.go
│   └── idgen_grpc.pb.go
├── auth/    # AuthenticationService
│   ├── auth.proto
│   ├── auth.pb.go
│   └── auth_grpc.pb.go
├── user/    # user service
│   ├── user.proto
│   ├── user.pb.go
│   └── user_grpc.pb.go
└── note/    # note service
    ├── note.proto
    ├── note.pb.go
    └── note_grpc.pb.go
```

## Service Definitions

| Package | Service | Description |
|----|------|------|
| `idgen` | `IDGeneratorService` | Distributed ID generation (Snowflake) |
| `auth` | `AuthService` | Login, token refresh, and logout |
| `user` | `UserService` | Registration and profile management |
| `note` | `NoteService` | Snippet CRUD, groups, and tags |

## Usage

```go
import pb "github.com/loqbit/ownforge/pkg/proto/idgen"

// Client Call
resp, err := client.NextID(ctx, &pb.NextIDRequest{})
fmt.Println(resp.Id) // Snowflake ID
```

## Regenerate

```bash
protoc --go_out=. --go-grpc_out=. proto/idgen/idgen.proto
```
