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

## Frontend rules (React + TS + shadcn + Tailwind + TanStack)

Repo owner does not review frontend code in detail. Generated frontend MUST self-enforce the following. Violations are bugs, not style issues.

**Design system**: see [docs/design.md](docs/design.md) — visual/interaction constitution. Follow strictly.

**Vue → React porting**: see [docs/vue-to-react-porting-guide.md](docs/vue-to-react-porting-guide.md) when migrating old Vue code.

**Stack (do not deviate without explicit ask)**:
- React 18+ with TypeScript, Vite
- Routing: TanStack Router
- Server state: TanStack Query
- Local state: `useState` / Zustand (only when truly shared)
- Styling: Tailwind + shadcn/ui components (copy-in, owned in repo)
- Forms: react-hook-form + zod
- Icons: lucide-react

**Hard rules (anti-pattern detection)**:
1. **Do not use `useEffect` to sync one state to another.** If A derives from B, compute during render or use `useMemo`. `useEffect` is for external-system sync only (subscriptions, timers, DOM imperative APIs).
2. **Do not fetch in `useEffect`.** All API calls go through TanStack Query (`useQuery` / `useMutation`). No exceptions.
3. **List `key` must be a stable ID, never array index.** If no ID exists, add one upstream.
4. **Do not sprinkle `useCallback` / `useMemo` by default.** Only add when profiling shows a measurable re-render issue. Unnecessary memoization is worse than none.
5. **Do not pass props through more than 2 levels.** If a value crosses 3+ components, use Context or Zustand.
6. **`async` is not allowed as a `useEffect` callback directly.** Define async inside, call it: `useEffect(() => { const run = async () => {...}; run(); }, [])`.
7. **Never call hooks conditionally, in loops, or after early returns.**
8. **Controlled inputs**: inputs backed by state must have `value` + `onChange`, not `defaultValue`. Exception: react-hook-form which manages this.
9. **Tailwind classes**: no arbitrary `style={{...}}` unless truly dynamic (e.g. computed color from data). Use Tailwind classes.
10. **No custom CSS files** in app code. Extend `tailwind.config.ts` if tokens missing.

**Positive preferences**:
- Data flow: server state via TanStack Query, invalidate on mutation. Do NOT cache server state in local `useState`.
- Suspense + Error Boundary at route level for loading/error states, not `isLoading &&` ladders everywhere.
- shadcn/ui components: install via `npx shadcn add <name>`. If a needed component does not exist in shadcn, prefer Radix primitive + Tailwind over building from scratch.
- Forms: always react-hook-form + zodResolver. No manual `useState` for form fields.
- File structure: colocate component + its styles + its tests. Route files in `src/routes/`, shared in `src/components/`, shadcn in `src/components/ui/`.

**When in doubt**:
- If a pattern feels like it needs more than one `useEffect`, stop and reconsider — the need for sync is usually a sign of derived state that should be computed, not stored.
- If a component grows past ~150 lines, split it.
- If you reach for Redux / MobX / Recoil — don't. Zustand or Context is enough for this project's scope.