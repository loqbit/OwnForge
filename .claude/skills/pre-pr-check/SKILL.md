---
name: pre-pr-check
description: Run the full local pre-PR check suite for OwnForge — gofmt, go vet, golangci-lint, go build, go test across the Go workspace. Use before opening a PR, before pushing, or whenever the user asks to "check", "verify", or "make sure it's green".
---

# pre-pr-check

One-command health check mirroring what CI will run. Runs from repo root.

## Steps (in order, stop on first failure)

1. **Format check** — `gofmt -l .` from repo root. If output is non-empty, run `gofmt -w` on the listed files and report what was reformatted.
2. **Vet** — `go vet ./...` across the workspace. `go.work` means this covers all modules.
3. **Lint** — `golangci-lint run ./...`. If `golangci-lint` is not installed, tell the user how to install (`brew install golangci-lint` or `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`) rather than skipping.
4. **Build** — `go build ./...` to catch compile errors across every module.
5. **Test** — `go test ./...`. If the user has a focused area, allow narrowing: e.g., `go test ./services/notes/...`.
6. **Proto freshness** — if any `.proto` file is newer than its matching `*.pb.go`, warn the user to run the `proto-gen` skill.
7. **Git hygiene** — report:
   - Any staged `.env*` or files matching `*secret*`, `*credential*`, `*.key`, `*.pem` — flag loudly, do not proceed.
   - Uncommitted changes to `go.work`, `go.work.sum`, or any `go.mod` that weren't intentional.

## Reporting

- On full pass: one-line green summary listing which steps ran and time taken.
- On failure: show the failing step's output (trimmed to the relevant part), do not run subsequent steps, suggest the likely fix.

## Scope

- **Go only** for now. When frontend lands under `apps/`, extend this skill with `tsc --noEmit`, `eslint`, and `pnpm test` steps — but only after the frontend actually exists.
- Does **not** run `docker compose` or hit any network/DB. Strictly static + unit tests.
