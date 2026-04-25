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
- **Apply runs on a feature branch named `change/<change-name>`, in a `git worktree`, not on `main`.** As the first step of `/opsx:apply`, Codex SHALL run `git worktree add ../<repo>-<change-name> -b change/<change-name>` from the main worktree's directory, then `cd` into that worktree for the rest of the apply run. The worktree (rather than `git checkout`) is mandatory because parallel Codex runs on different changes need independent working directories — sharing a tree corrupts files. After the worktree exists, copy `.env` over manually because it's gitignored. All apply / verify / fix-up commits land in that worktree on the feature branch; the branch merges back to `main` only after `/opsx:archive` completes — see the next bullet for the merge-back sequence.
- **`/opsx:archive` is followed by a merge-back sequence Claude SHALL perform** when the change is on a `change/*` branch (skip if on `main` directly — legacy serial flow). Right after archive succeeds and the post-archive commit is in place, Claude runs (substituting `<branch>` for the current `change/<change-name>`):

  1. `git push origin <branch>` — pushes the feature branch so CI runs on it.
  2. `main_path="$(git worktree list --porcelain | awk '/^worktree / {p=$2} /^branch refs\/heads\/main$/ {print p; exit}')"` — locate the main worktree.
  3. `git -C "$main_path" merge --no-ff <branch> -m "Merge branch '<branch>' into main"` — integrate, preserving branch history.
  4. `git -C "$main_path" push origin main` — push the merge.
  5. Wait for CI on `main`; report success or surface failures.
  6. `git -C "$main_path" worktree remove <worktree-path>` and `git -C "$main_path" branch -d <branch>` — local cleanup.
  7. `git -C "$main_path" push origin --delete <branch>` — drop the remote branch (best-effort; suppress non-zero exit).

  This sequence is Claude's responsibility because Claude is the only role with the Bash tool wired up for git plumbing. Codex finishes at apply; the user's only inputs in the lifecycle are starting Codex and saying "done".
- **Behavior drift → fix the artifact first.** If review finds the implementation diverges from `design.md` / `tasks.md` / specs in a way that changes user-visible behavior or interfaces, update the artifact first and re-apply. Typos, renames, refactors that preserve behavior, comment tweaks, and logging changes can be patched directly without an artifact update.
- **Small-change fast path.** When a behavior change is contained — one capability, one or two functions touched, no schema migration, no new endpoint, no cross-capability interaction — Claude MAY apply it directly without going through propose / apply / archive. The fast path still requires (a) updating the relevant `openspec/specs/<capability>/spec.md` in the same commit (spec stays source of truth), (b) tests covering the new behavior per the Done definition (one success + one rejection), and (c) a `change/<name>` branch + worktree + the standard merge-back sequence — only the change folder and Codex hand-off are skipped. Examples that fit: invalidating sessions on password reset; adding one rejection branch to an existing handler; tightening a CHECK constraint without a schema change. Examples that do NOT fit: any new table or column, any new endpoint family, any UI feature, any cross-capability refactor.
- **Parallelism is for independent or stacked changes.** Two changes may run in parallel — each on its own Codex + worktree — when (a) their files and schema are disjoint (independent), or (b) they form a stacked pair under the rules below. Never spawn parallel agents to "speed up" the same change folder.
- **Stacked branches for dependent changes.** When change B depends on change A whose code has not yet merged to `main`, B's worktree SHALL be opened with A's branch as its base: `git worktree add ../<repo>-<B-name> -b change/<B-name> change/<A-name>` (substituting actual names). Apply / verify / archive / merge-back proceed as normal but with two extra rules:

  1. When A receives new commits during review, Claude SHALL rebase B onto the latest A: `git -C ../<repo>-<B-name> rebase change/<A-name>`. Spec-delta conflicts (B's MODIFIED requirements layered on A's MODIFIED) are resolved by hand; code conflicts are usually Codex re-applying around the new baseline.
  2. B's `/opsx:archive` + merge-back SHALL happen strictly after A's. The order is: A archive → A merge → B rebase onto main (`git rebase --onto main change/<A-name> change/<B-name>`) → B archive → B merge. Reversing the order makes B's spec sync reference requirements that are not yet in `main`.

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
