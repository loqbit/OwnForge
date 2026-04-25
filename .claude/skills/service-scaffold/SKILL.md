---
name: service-scaffold
description: Scaffold a new Go backend service under `services/<name>/` following OwnForge's monorepo conventions — cmd/server entrypoint, internal/ layout, Dockerfile, Makefile, and go.mod wired into go.work. Use when the user asks to add / create / scaffold a new service.
---

# service-scaffold

Create a new service that matches the shape of existing ones (`gateway`, `notes`, `user-platform`, `id-generator`). Before scaffolding, **read one existing service first** (prefer `notes` — it's the most complete reference) and mirror its structure.

## Intake

Confirm with the user before generating files:

1. **Service name** — kebab-case, e.g., `billing`, `search-indexer`. This becomes the directory `services/<name>/`, the module path, and the Conventional Commits scope.
2. **Protocols** — HTTP only, gRPC only, or both? Most OwnForge services do both via gRPC-gateway or parallel servers.
3. **Proto package** — does it need a new `pkg/proto/<name>/` package? If yes, run the `proto-gen` skill after creating the `.proto` file.
4. **Dependencies** — Postgres? Redis? NATS/Kafka? OpenTelemetry? (All available under `pkg/`.)

## Target layout

```
services/<name>/
├── cmd/server/main.go          # entrypoint; wire config, logger, otel, servers
├── internal/
│   ├── config/config.go        # viper-based, mirror services/notes/internal/config
│   ├── handler/                # gRPC handlers (and HTTP if applicable)
│   ├── service/                # business logic, tenant-aware
│   └── repo/                   # data access (Postgres/Redis)
├── Dockerfile                  # multi-stage, matches services/notes/Dockerfile
├── Makefile                    # targets: run, build, test, lint, docker-up/down
├── go.mod                      # module name: github.com/luckysxx/ownforge/services/<name>
├── .env.example                # document env vars; never commit .env itself
└── README.md                   # one-paragraph purpose + how to run
```

## Procedure

1. Read `services/notes/` top-to-bottom. Copy structure, not content.
2. Generate each file. Key rules:
   - Every service reads `tenant_id` from `ctx` — never hardcode. Import `pkg/tenant` if it exists.
   - Every service call takes `ctx context.Context` as first arg.
   - Use `pkg/logger`, `pkg/otel`, `pkg/probe`, `pkg/metrics`, `pkg/health` — do not re-implement.
   - No `init()` with side effects. No panics outside `main()`.
3. After files are written:
   - Add the new module to `go.work`: `go work use ./services/<name>`.
   - Run `go mod tidy` inside the new service.
   - Run `go build ./...` from repo root to confirm wiring.
4. If `deploy/compose/` has a docker-compose file, add a service entry (ask user before modifying shared infra).
5. Commit in two parts: (a) `feat(<name>): scaffold service skeleton`, (b) the proto changes separately if any, scoped `feat(proto)`.

## Anti-patterns

- Copying a service and leaving old scope names/imports in place.
- Adding new shared helpers to the new service instead of `pkg/`.
- Skipping `go work use` — the new module won't build against local `pkg/*`.
- Creating a Dockerfile that doesn't match the repo's multi-stage pattern (breaks `deploy/`).
