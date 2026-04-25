---
name: commit-dco
description: Create a git commit that follows OwnForge's conventions — Conventional Commits format with scope = service or package name, plus mandatory DCO sign-off via `git commit -s`. Use whenever the user asks to commit, stage a commit, or create a commit message in this repo.
---

# commit-dco

Create commits that conform to OwnForge's conventions. Non-negotiable:

1. **Conventional Commits**: `<type>(<scope>): <subject>`
   - Types: `feat`, `fix`, `docs`, `chore`, `refactor`, `test`, `perf`, `build`, `ci`
   - Scope is **required** and must match a real directory name:
     - A service: `gateway`, `notes`, `user-platform`, `id-generator`
     - A package: `pkg/mq`, `pkg/logger`, `pkg/otel`, etc. (use the leaf, e.g. `mq`, `logger`)
     - `proto` for changes under `pkg/proto/`
     - `deploy`, `docs`, `ci`, `deps` for cross-cutting changes
   - Subject: imperative mood, no trailing period, ≤ 72 chars.

2. **DCO sign-off**: always pass `-s` to `git commit`. This appends `Signed-off-by: <name> <email>` using the user's git config. Never bypass with `--no-verify` or skip `-s`.

3. **Never** commit `.env*`, credentials, or generated binaries. Check `git diff --cached` before committing.

## Procedure

1. Run `git status` and `git diff --cached` (or `git diff` if nothing staged) to understand what's changing.
2. If no files are staged, ask the user which files to include — do not run `git add -A`.
3. Infer type + scope from the changed paths. If the change spans multiple scopes, prefer the most specific shared scope (e.g., two service handlers calling a new `pkg/mq` helper → scope is `mq` if the diff centers there, otherwise split into multiple commits).
4. Draft the message. Body (optional) explains **why**, not what. No trailing summaries, no AI attribution lines.
5. Run `git commit -s -m "<message>"`. If the pre-commit hook fails, **fix the underlying issue and create a new commit** — do not `--amend`.
6. Run `git status` to confirm.

## Examples

- `feat(notes): add tag filter to list endpoint`
- `fix(gateway): propagate tenant_id through bearer middleware`
- `refactor(mq): extract envelope codec into its own file`
- `chore(deps): bump google.golang.org/grpc to v1.66`
- `docs(proto): document idgen service RPC semantics`

## Anti-patterns (refuse these)

- `git commit` without `-s`
- Missing scope: `feat: add thing` ❌ → `feat(notes): add thing` ✅
- `git add -A` followed by commit without reviewing diff
- `--amend` after a failed hook
- Co-authored-by or "Generated with Claude" trailers unless the user explicitly asks
