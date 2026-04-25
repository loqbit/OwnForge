---
name: proto-gen
description: Regenerate Go gRPC code from `.proto` files under `pkg/proto/`. Use after editing any `.proto` file, or when the user asks to regenerate proto / pb.go / grpc stubs.
---

# proto-gen

OwnForge uses raw `protoc` (not `buf`). Proto files and generated Go code live together under `pkg/proto/<package>/`.

## Layout

```
pkg/proto/
├── auth/       auth_api.proto       → auth_api.pb.go, auth_api_grpc.pb.go
├── idgen/      idgen.proto          → idgen.pb.go, idgen_grpc.pb.go
├── note/       note_api.proto       → note_api.pb.go, note_api_grpc.pb.go
└── user/       user_api.proto       → user_api.pb.go, user_api_grpc.pb.go
```

## Procedure

1. Identify which proto package changed (from git status or user instruction).
2. From `pkg/proto/`, run:
   ```
   protoc --go_out=. --go-grpc_out=. <package>/<file>.proto
   ```
   Example: `protoc --go_out=. --go-grpc_out=. note/note_api.proto`
3. To regenerate **all** protos:
   ```
   cd pkg/proto && protoc --go_out=. --go-grpc_out=. */*.proto
   ```
4. After generation:
   - Run `gofmt -w pkg/proto/<package>/`.
   - Run `go build ./...` from repo root to confirm nothing downstream broke.
5. Generated files (`*.pb.go`, `*_grpc.pb.go`) **must be committed** alongside the `.proto` change. Scope the commit as `feat(proto)` or `fix(proto)`.

## Pre-flight checks

- `protoc --version` should be ≥ v7.x, with `protoc-gen-go` ≥ v1.36 and `protoc-gen-go-grpc` ≥ v1.6 on `PATH`. If missing, tell the user to install via `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest` and `google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest`.
- Never edit `*.pb.go` or `*_grpc.pb.go` by hand — they carry `// Code generated ... DO NOT EDIT.` headers.

## Common mistakes

- Running `protoc` from repo root instead of `pkg/proto/` → import paths break.
- Forgetting to commit regenerated files → downstream builds fail in CI.
- Hand-editing generated files instead of fixing the `.proto`.
