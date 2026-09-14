## Development Workflow

This project uses local Markdown for requirements, planning, and decisions.
Trellis is not required; do not initialize it or require its commands or task
activation before implementation.

The active sources of truth are:

- `docs/planning/` for confirmed discussion records and migration plans.
- `.scratch/<feature>/spec.md` and `issues/` for feature specs and implementation
  tickets, following [docs/agents/issue-tracker.md](docs/agents/issue-tracker.md).
- `CONTEXT.md` for domain vocabulary and `docs/adr/` for architectural decisions.
- Root and per-language `AGENTS.md` files for development conventions.

`openspec/` is retained as read-only legacy history. Do not create, apply,
verify, or archive new OpenSpec changes.

### Rules of engagement

- **Conversation/read-only/small change → direct work.** Clearly low-risk edits
  can proceed without a separate task document when they do not materially change product
  behavior, API/admin contracts, data shape, permissions, scheduling
  rules, or operational semantics. Examples include typos, comments,
  formatting, test-only cleanup, scaffold metadata, minor copy, and small
  visual polish that preserves the workflow.
- **Material change → documented scope.** Persist confirmed requirements and
  acceptance criteria before implementation. Complex work also needs a design
  and implementation checklist; these may live alongside the feature spec.
- **Approval before implementation.** Read the relevant specs and conventions,
  and obtain explicit implementation approval. A confirmed discussion record
  may serve as the originating spec; no separate workflow activation is needed.
- **Behavior drift → artifact first.** If implementation would materially
  diverge from confirmed requirements or decisions, update the artifact and
  obtain user confirmation before continuing.
- **One active effort.** Keep one implementation effort active in this checkout
  unless the user explicitly chooses a worktree strategy.

### Loop

```text
clarify and record requirements
  -> user approves implementation
  -> load conventions and plan implementation
  -> implement locally
  -> rsync to Centaurus
  -> project checks on Centaurus
  -> review against the fixed baseline and confirmed spec
  -> fix and re-verify
  -> update documentation and verification record
  -> local commit after successful verification
```

### Git and Centaurus

- Use a dedicated local branch for material tasks. Do not use a `codex/` prefix
  or impose a `planning/` prefix. Record the branch and fixed base commit in
  the feature's planning or spec document.
- The local checkout is the only source of truth. Make edits and perform all Git
  operations locally, then synchronize one-way to Centaurus.
- Run build, lint, type-check, and tests on Centaurus. Use `mise` there for
  additional tools. If a service must be exercised, forward its port locally.
- If Centaurus is unavailable, report it before running resource-intensive
  checks locally.
- Follow [Conventional Commits](https://www.conventionalcommits.org/):

  - `feat(scope):` new feature
  - `fix(scope):` bug fix
  - `chore:` tooling, config, dependencies
  - `docs:` documentation only
  - `test:` tests only

Keep confirmed requirements, decisions, and verification results as durable
Markdown records. Commit only task-related changes; do not commit incomplete
or blocked implementation work.

---

## Commands

**Development commands** (execute on Centaurus under the global workflow):

- `make up` / `make down` — start or stop the API, admin, database, and development mail service
- `make migrate-up` / `make migrate-down-one` / `make migrate-status` — generated `migrate` migrations
- `make api-integration` — run the tagged API suite against a fresh Postgres container

**Tests and checks:**

- API: `cd api && go test ./...`, `go test -p 1 -tags=integration ./...` (needs Postgres running), `go vet ./...`, `go build ./...`
- Admin: `cd admin && pnpm test && pnpm lint && pnpm check && pnpm build`
- `make vuln-check` runs the pinned CI-equivalent `govulncheck ./...` scan from the `api/` module.

**Production stack:**

- `make prod-up` / `make prod-down` / `make prod-logs`

---

## Done definition

A task is done only when all of the following hold:

- Tests are added for the new behavior: a success path and at least one rejection / error path per new service method.
- API: `go build ./...`, `go vet ./...`, `go test ./...` all clean. Changes that touch SQL also run integration tests clean.
- Admin: `pnpm lint`, `pnpm test`, `pnpm build` all clean.
- The confirmed spec's acceptance criteria and implementation checklist are all satisfied.

Missing or insufficient tests are a blocker before committing or archiving, not
a follow-up.

---

## Config conventions

- All configuration is via environment variables.
- New variables must be added to `.env.example`. Non-sensitive entries: include a default. Sensitive entries: leave blank.
- Local development password: `pa55word` uniformly across services.
- Secrets never land in git; verify `.env` is `.gitignore`d before adding any new secret.

---

## Style guide references

When in doubt about a coding style question, consult the public guides pinned in the per-language `AGENTS.md`. Project-level conventions (the rules in those files) take precedence over the public guides where they conflict.

We do not pursue numeric targets for test or comment coverage. Coverage falls out of "every new behavior has a success-path and a rejection-path test" (see Done definition above) plus "comments only where the *why* is non-obvious." Hitting an arbitrary percentage is not a goal.

- API Go: see [api/AGENTS.md](api/AGENTS.md).
- Admin TS / React: see [admin/AGENTS.md](admin/AGENTS.md).

## Agent skills

### Issue tracker

Issues and specs live in local Markdown under `.scratch/<feature>/`.
See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context layout: root `CONTEXT.md` and `docs/adr/`.
See `docs/agents/domain.md`.
