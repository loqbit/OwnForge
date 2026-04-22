# AGENTS.md — OwnForge Project Instructions for AI Agents

This file is the single source of truth for all AI agents working on this repo.
Other tool-specific files (CLAUDE.md, .cursor/rules/, etc.) point here.

## 1. Project Context

OwnForge is a self-hosted, AI-native personal productivity suite. Monorepo.
Read `docs/ARCHITECTURE.md` for the layered architecture (Core + Edge).
Read `docs/ROADMAP.md` for current phase and upcoming milestones.


## 2. Repo Layout

- `apps/` — Next.js / Tauri frontends
- `services/` — Go backend services (gRPC + HTTP)
- `pkg/` — Go shared libraries
- `packages/` — TypeScript shared libraries
- `proto/` — gRPC contracts
- `deploy/compose/` — self-host deployment
- `docs/` — design docs (read before architectural changes)

## 3. Code Style

### Go
- Standard Go layout. Run `gofmt` and `golangci-lint` before committing.
- Errors: return wrapped (`fmt.Errorf("context: %w", err)`), never swallow.
- No panics outside main(). No `init()` with side effects.
- Context-first: every service call takes `ctx context.Context` as first arg.

### TypeScript
- Strict mode on. No `any` without `// TODO: type`.
- Shared code in `packages/*`, not duplicated per app.
- React: function components only, no class components.

### Naming
- Go: `snake_case` files, `CamelCase` exports.
- TS: `kebab-case` files, `camelCase` vars, `PascalCase` components.

## 4. Git & Commits

### Commit format — Conventional Commits (strict)
- `feat(scope): ...` — new feature
- `fix(scope): ...` — bug fix
- `docs(scope): ...` — docs only
- `chore(scope): ...` — tooling / deps
- `refactor(scope): ...` — no behavior change
- `test(scope): ...` — test only

Scope = service or package name, e.g., `feat(notes-api): add tag endpoint`.

### DCO required
All commits must include `Signed-off-by: Name <email>`.
Use `git commit -s` or set `git config --global format.signOff true`.

### Branch naming
- `feat/<short-desc>` for features
- `fix/<short-desc>` for fixes
- `chore/<short-desc>` for tooling

### What NOT to do
- ❌ `git push --force` to main (ever)
- ❌ Commit without DCO sign-off
- ❌ Commit `.env` files or credentials
- ❌ Commit generated code without regenerating proof (for `proto/` output)

## 5. Permissions

Agents may run freely:
- `go test ./...`, `go build ./...`, `go vet ./...`
- `docker compose up/down/ps/logs`
- `git status`, `git diff`, `git log`, `git add`, `git commit`
- Reading any file in the repo

Agents must ASK before:
- `git push` (any branch)
- `docker system prune` or any `docker rm`
- Modifying `LICENSE`, `CLAUDE.md`, `AGENTS.md`, `docs/`
- Installing new dependencies (`go get`, `npm install`)
- Running database migrations
- Any command touching files outside the repo

Agents must NEVER:
- Run `git push --force` without explicit user confirmation per-occurrence
- Commit secrets (`.env`, tokens, keys)
- Delete files in `docs/`

## 6. Testing Expectations

- Every new service endpoint needs at least one test.
- Use real Postgres in integration tests (testcontainers-go), not mocks.
- Run `go test ./...` before declaring a task done.

## 7. Architecture Rules (non-negotiable)

- All backend services read `tenant_id` from `ctx`. Never hardcode tenant.
- Storage access goes through `pkg/storage` interface. No direct S3/FS calls.
- Cross-service calls go through gRPC defined in `proto/`. No HTTP between services.
- Every service must work in `docker compose up` with zero external deps.
- Do not reference or create files matching `OwnForge_*_v*.md`. These are private working docs kept outside the repo.


## 8. Role Hints (optional, for multi-agent workflows)

If you know which agent you are, follow these preferences:

- **Architecture / planning / docs** → You're probably Claude. Read `docs/` first, propose changes in plan form, don't write code unless asked.
- **Autocomplete / small edits** → You're probably Copilot/Cursor Tab. Match surrounding style exactly, no refactoring.
- **Refactoring / renaming / codemods** → Check `pkg/` and `packages/` for shared types, don't duplicate.
- **Test writing** → Use testcontainers, not mocks. Match existing test file layout.

## 9. When in doubt

1. Read `docs/ARCHITECTURE.md` for current phase.
2. Prefer editing existing files over creating new ones.
3. Ask the user rather than guessing.