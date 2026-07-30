## Development Workflow

This project uses **Trellis** for spec-driven development.

The active sources of truth are:

- [`.trellis/workflow.md`](.trellis/workflow.md) for task phases, risk
  classification, validation, and finish behavior.
- [`.trellis/spec/`](.trellis/spec/) for backend/frontend conventions, shared
  guides, and product behavior contracts.
- [`.trellis/tasks/`](.trellis/tasks/) for the active task's requirements,
  design, implementation plan, and archived execution record.

`openspec/` is retained as read-only legacy history. Do not create, apply,
verify, or archive new OpenSpec changes.

### Rules of engagement

- **Conversation/read-only/small change → direct work.** Clearly low-risk edits
  can proceed without a Trellis task when they do not materially change product
  behavior, backend/frontend contracts, data shape, permissions, scheduling
  rules, or operational semantics. Examples include typos, comments,
  formatting, test-only cleanup, scaffold metadata, minor copy, and small
  visual polish that preserves the workflow.
- **Material change → Trellis task.** Obtain task-creation consent, create the
  task, and persist requirements before implementation. Lightweight tasks may
  be PRD-only; complex tasks require `prd.md`, `design.md`, and `implement.md`.
- **Activate before editing.** Material implementation begins only after
  `task.py start` changes the task to `in_progress` and relevant specs have been
  loaded through `trellis-before-dev`.
- **Behavior drift → artifact first.** If implementation would materially
  diverge from the task or `.trellis/spec/domain/`, update and review the
  artifact before continuing.
- **One active task.** Keep one Trellis task active in this checkout unless the
  user explicitly chooses a worktree strategy.

### Loop

```text
trellis-start
  -> task.py create (material work only)
  -> prd.md [+ design.md + implement.md for complex work]
  -> task.py start
  -> trellis-before-dev
  -> implement locally
  -> rsync to Centaurus
  -> trellis-check + project checks on Centaurus
  -> trellis-update-spec
  -> local commit
  -> trellis-finish-work
```

### Git and Centaurus

- Use a dedicated local branch for material tasks; Codex-created branches use
  the `codex/` prefix by default. Record branch and base branch in `task.json`.
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

Trellis task archives and developer journals are durable records and should be
committed after the work commit according to `.trellis/workflow.md`.

---

## Commands

**Development commands** (execute on Centaurus under the global workflow):

- `make run-backend` — start the Go server
- `make run-frontend` — start the Vite dev server
- `make migrate-up` / `make migrate-down` / `make migrate-status` — goose migrations

**Tests and checks:**

- Backend: `cd backend && go test ./...` (unit), `go test -tags=integration ./...` (needs Postgres running), `go vet ./...`, `go build ./...`, `govulncheck ./...`
- Frontend: `cd frontend && pnpm test && pnpm lint && pnpm build`

**Production stack:**

- `make prod-up` / `make prod-down` / `make prod-logs`

---

## Done definition

A task is done only when all of the following hold:

- Tests are added for the new behavior: a success path and at least one rejection / error path per new service method.
- Backend: `go build ./...`, `go vet ./...`, `go test ./...` all clean. Changes that touch SQL also run integration tests clean.
- Frontend: `pnpm lint`, `pnpm test`, `pnpm build` all clean.
- The active task's acceptance criteria and `implement.md` boxes are all ticked.

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

- Backend Go: see [backend/AGENTS.md](backend/AGENTS.md).
- Frontend TS / React: see [frontend/AGENTS.md](frontend/AGENTS.md).
<!-- TRELLIS:START -->
# Trellis Instructions

These instructions are for AI assistants working in this project.

This project is managed by Trellis. The working knowledge you need lives under `.trellis/`:

- `.trellis/workflow.md` — development phases, when to create tasks, skill routing
- `.trellis/spec/` — package- and layer-scoped coding guidelines (read before writing code in a given layer)
- `.trellis/workspace/` — per-developer journals and session traces
- `.trellis/tasks/` — active and archived tasks (PRDs, research, jsonl context)

If a Trellis command is available on your platform (e.g. `/trellis:finish-work`, `/trellis:continue`), prefer it over manual steps. Not every platform exposes every command.

If you're using Codex or another agent-capable tool, additional project-scoped helpers may live in:
- `.agents/skills/` — reusable Trellis skills
- `.codex/agents/` — optional custom subagents

Managed by Trellis. Edits outside this block are preserved; edits inside may be overwritten by a future `trellis update`.

<!-- TRELLIS:END -->
