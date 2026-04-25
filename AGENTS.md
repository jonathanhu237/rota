## Development Workflow

This project uses **OpenSpec** for spec-driven development. Every behavior change goes through a change folder under `openspec/changes/` before any code is touched. Specs in `openspec/specs/` are the source of truth.

### Roles

- **Claude**: explores, writes spec artifacts (`proposal.md`, `design.md`, `tasks.md`, delta specs), reviews implementation against spec, commits.
- **Codex**: applies tasks (implements code, runs tests, reports outcomes).

### Loop

```
/opsx:explore (optional)
    → /opsx:propose <change-name>
    → /opsx:apply
    → /opsx:verify (recommended for multi-session changes)
    → review
    → /opsx:archive
    → git commit
```

1. **Explore (optional):** `/opsx:explore` — read the codebase, compare options, sketch designs. No code written.
2. **Propose:** `/opsx:propose <change-name>` — scaffold `openspec/changes/<name>/` with `proposal.md`, `design.md`, `tasks.md`, and any delta under `specs/`.
3. **Apply:** `/opsx:apply` — work through `tasks.md`, ticking each `- [ ]` to `- [x]`. Codex drives.
4. **Verify (optional):** `/opsx:verify` — completeness / correctness / coherence pass. Useful when the change spans multiple sessions or multiple agents; redundant if a human review immediately follows `/opsx:apply`.
5. **Review:** Claude verifies implementation against the change artifacts. Behavior drift is fixed in artifacts first, then re-applied — do not patch code to match a stale design.
6. **Archive:** `/opsx:archive` — move the change to `openspec/changes/archive/YYYY-MM-DD-<name>/` and merge delta specs into `openspec/specs/`. Commit the result.

Skill details live under `.claude/skills/openspec-*/SKILL.md` (mirrored to `.codex/skills/`).

### Rules of engagement

- **One live writer per change.** Do not have Claude and Codex editing the same files concurrently. Handoff happens through the OpenSpec artifacts, not through the working tree.
- **Apply runs on a feature branch, not on `main`.** Before `/opsx:apply`, check out a branch named `change/<change-name>` from `main`. Codex's apply, Claude's review, the post-archive commit, and any post-smoke fix-up commits all happen on that branch. The branch merges back to `main` (`git merge --no-ff`) only after `/opsx:archive` succeeds and the post-archive commit is in place. This lets `main` stay green while a long-running apply is in progress, and lets Claude propose / explore / verify other changes in parallel without working-tree contention.
- **Behavior drift → fix the artifact first.** If review finds the implementation diverges from `design.md` / `tasks.md` / specs in a way that changes user-visible behavior or interfaces, update the artifact first and re-apply. Typos, renames, refactors that preserve behavior, comment tweaks, and logging changes can be patched directly without an artifact update.
- **Parallelism is for independent work.** Spawn parallel agents only when tasks genuinely don't share files or state; never to "speed up" the same change folder.

### Commit convention

Follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat(scope):` new feature
- `fix(scope):` bug fix
- `chore:` tooling, config, dependencies
- `docs:` documentation only
- `test:` tests only

The archived change directory under `openspec/changes/archive/` is part of the commit — that is the durable record of what was built and why.

---

## Commands

**Run locally:**

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
- `tasks.md` boxes are all ticked.

Missing or insufficient tests are a blocker before archiving, not a follow-up.

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
