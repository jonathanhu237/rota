# Implementation Plan

## 1. Establish Trellis Foundations

- [x] Create `codex/migrate-openspec-to-trellis` locally.
- [x] Complete and archive the generated `00-bootstrap-guidelines` task without
      an automatic commit.
- [x] Replace backend/frontend/shared placeholder guides with Rota-specific
      conventions and examples.
- [x] Set this task's branch, base branch, and scope metadata.
- [x] Validate `prd.md`, `design.md`, and this plan, then start the task.

## 2. Configure the Active Workflow

- [x] Set `session_auto_commit: false` and `codex.dispatch_mode: inline`.
- [x] Customize `.trellis/workflow.md` without breaking parser-managed
      workflow-state blocks.
- [x] Replace the OpenSpec workflow section in `AGENTS.md` with the concise
      Trellis entry point and preserve project commands/done criteria.
- [x] Verify workflow phase extraction for no-task, planning, and in-progress
      states.

Rollback point: restore these three files before touching legacy entry points.

## 3. Migrate Product Contracts

- [x] Create `.trellis/spec/domain/index.md`.
- [x] Mechanically copy the eight current OpenSpec capability specs into the
      domain layer.
- [x] Replace only the five placeholder Purpose sections with accurate
      summaries.
- [x] Compare Requirement and Scenario counts for every source/destination
      pair.
- [x] Inspect the full diffs and confirm no behavior contract was lost.

Rollback point: remove only `.trellis/spec/domain/`; OpenSpec remains intact.

## 4. Retire OpenSpec as an Execution Workflow

- [x] Add `openspec/README.md` and a legacy header to `openspec/config.yaml`.
- [x] Delete project-local OpenSpec adapters from `.codex/skills/`,
      `.claude/skills/`, and `.claude/commands/opsx/`.
- [x] Update prospective OpenSpec wording in ADR 0002 and ADR 0003.
- [x] Search outside the legacy archive for instructions that still call
      OpenSpec active.

## 5. Local Lightweight Validation

- [x] Run `task.py validate` and all relevant `get_context.py` phase/package
      probes.
- [x] Check Markdown relative links and placeholder markers.
- [x] Run domain parity checks and `git diff --check`.
- [x] Review `git status` for secrets, generated caches, and unrelated files.

Do not run resource-intensive product checks locally under the Centaurus
workflow.

## 6. Synchronize and Validate on Centaurus

- [x] Verify the exact remote mirror path.
- [x] Rsync local to `/home/jonathanhu237/code/rota/` with explicit exclusions
      for secrets, `*.local.*`, caches, dependencies, and build output, with
      deletion scoped to that mirror.
- [x] Use Centaurus `mise` to provide the Go, Node, and pnpm versions required
      by the repository.
- [x] Run `go build ./...`, `go vet ./...`, and `go test ./...` in `backend/`.
- [x] Run a frozen pnpm install, `pnpm lint`, `pnpm test`, and `pnpm build` in
      `frontend/`.
- [x] Re-run Trellis documentation/task validation in the remote mirror.

## 7. Finish

- [x] Apply fixes locally and repeat rsync/remote checks if any validation
      fails.
- [x] Run the final Trellis check and update task acceptance checkboxes.
- [ ] Record the migration in the developer journal.
- [x] Create one local Conventional Commit containing Trellis initialization,
      migrated specs, workflow cutover, and the archived bootstrap task.
- [x] Archive this task without an extra automatic commit and report the local
      branch/commit. Do not push or create a PR.
