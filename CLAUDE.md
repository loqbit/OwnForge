See [AGENTS.md](./AGENTS.md) for full project conventions.

## Critical rules (do not violate)

- **DCO required**: all commits must be signed off. Use `git commit -s`.
- **Tenant-agnostic core**: backend services read `tenant_id` from `ctx`. Never hardcode.
- **Conventional Commits**: `feat(scope): ...`, `fix(scope): ...`, etc.
- **Never `git push --force` to main**.
- **Never commit `.env` files or secrets**.
- **Prefer editing existing files over creating new ones**.

## Before large changes

1. Read `docs/ROADMAP.md` for current phase.
2. Propose the plan before implementing.