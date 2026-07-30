# Migrate OpenSpec to Trellis

## Goal

Make Trellis the only active spec-driven development workflow for Rota without
changing product behavior or losing the rationale and contracts accumulated
under OpenSpec.

## Background

- The repository currently has eight OpenSpec capability specifications:
  attendance, audit, auth, branding, dev-tooling, frontend-shell, outbox, and
  scheduling.
- `openspec/changes/archive/` contains the historical proposal/design/task
  record and must remain available for investigation.
- Trellis 0.6.7 has been initialized in this checkout, including Codex hooks,
  task scripts, and project-scoped skills.
- The global Centaurus workflow makes the local checkout the only source of
  truth. Code and Git operations happen locally; one-way rsync sends the result
  to Centaurus for build and test.
- The user approved proceeding with the migration and specifically requested
  that Centaurus perform verification.

## Requirements

1. `.trellis/spec/` SHALL become the active source of coding conventions and
   product behavior contracts.
2. The current OpenSpec capability requirements and scenarios SHALL be carried
   into `.trellis/spec/domain/` without intentional behavior changes.
3. Project-specific backend, frontend, and shared guides SHALL describe actual
   Rota patterns and SHALL contain no bootstrap placeholders or unrelated
   Trellis CLI examples.
4. `AGENTS.md`, `.trellis/workflow.md`, and `.trellis/config.yaml` SHALL route
   future development through Trellis.
5. The workflow SHALL preserve the existing risk threshold:
   - read-only work and clearly small, low-risk edits may proceed without a
     task;
   - changes to product workflows, APIs, persistence, auth/permissions,
     scheduling invariants, or operational semantics require a Trellis task;
   - complex tasks require `prd.md`, `design.md`, and `implement.md` before
     implementation.
6. Codex SHALL use Trellis inline mode, and Trellis scripts SHALL not
   auto-commit task archives or journal changes.
7. The OpenSpec specs, configuration, and archived changes SHALL remain in the
   repository as read-only legacy material with an explicit migration notice.
   Historical OpenSpec archives SHALL NOT be fabricated into Trellis tasks.
8. Project-local OpenSpec execution skills SHALL be retired so future agents do
   not start a second active workflow accidentally.
9. Historical ADR references that direct future work to OpenSpec SHALL instead
   direct future work to a focused Trellis task.
10. All local edits and Git operations SHALL occur in this checkout. The exact
    local source tree SHALL be synchronized one-way to
    `/home/jonathanhu237/code/rota` on Centaurus for validation.
11. No backend, frontend, database, or runtime behavior SHALL change as part of
    this migration.

## Acceptance Criteria

- [x] Backend, frontend, and shared Trellis guides contain real Rota paths,
      patterns, examples, and pre-development checklists with no placeholders.
- [x] `.trellis/spec/domain/index.md` indexes exactly the eight migrated
      capabilities and declares their authority and provenance.
- [x] Each migrated domain file retains the OpenSpec requirement and scenario
      set; any content change is limited to migration metadata or replacing a
      placeholder purpose with an accurate summary.
- [x] Root `AGENTS.md` names Trellis as the active workflow and no longer tells
      agents to create/apply/archive OpenSpec changes.
- [x] `.trellis/workflow.md` encodes the Rota task threshold, artifact gates,
      single-active-task rule, branch metadata, quality gate, Centaurus
      validation, and Conventional Commits.
- [x] `.trellis/config.yaml` sets `session_auto_commit: false` and
      `codex.dispatch_mode: inline`.
- [x] `openspec/README.md` and the legacy config header make the frozen status
      explicit while all tracked OpenSpec history remains present.
- [x] No project-local `.codex/skills/openspec-*`,
      `.claude/skills/openspec-*`, or `.claude/commands/opsx/` execution entry
      point remains.
- [x] The two ADRs that mention future OpenSpec changes now reference Trellis
      tasks.
- [x] Trellis task/context validation, Markdown/link checks, `git diff --check`,
      and migration parity checks pass.
- [x] After one-way rsync, Centaurus passes backend build/vet/unit tests and
      frontend locked install/lint/tests/build.
- [x] No secrets, local `.env`, Git database, dependency cache, or certificate
      material is copied to Centaurus.

## Out of Scope

- Rewriting or reorganizing product behavior.
- Converting the historical OpenSpec archive into synthetic Trellis tasks.
- Deleting OpenSpec history.
- Changing production deployment state, running migrations, or starting
  long-lived services.
- Pushing a branch or opening a pull request.
