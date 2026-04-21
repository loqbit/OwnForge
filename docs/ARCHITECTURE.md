# Architecture

OwnForge is a monorepo with a layered "Core + Edge" architecture.

## Layers

### Core (open source, shared across deployments)
Services that run identically on laptop, NAS, VPS, or cloud.
- `user-platform` — SSO and account management
- `gateway` — HTTP/gRPC router with JWT auth
- `notes-api`, `chat-api` — application services
- `ai-worker` — AI abstraction bus (Ollama / OpenAI / Claude / ...)

### Edge (cloud-only, future)
Multi-tenant router, billing, operators. Not part of self-host.

### Storage (driver-based)
- Postgres (cloud RDS / local container / desktop SQLite)
- Object storage (S3 / MinIO / local filesystem)

## Design principles

1. **Tenant-agnostic core**: services read `tenant_id` from context.
2. **One compose, many deployments**: differences live in env flags and compose profiles.
3. **gRPC between services**, REST at the edge.

## Repo layout

- `apps/` — Next.js / Tauri frontends
- `services/` — Go backend services
- `pkg/` — Go shared libraries
- `packages/` — TypeScript shared libraries
- `proto/` — gRPC contracts
- `deploy/compose/` — self-host deployment
