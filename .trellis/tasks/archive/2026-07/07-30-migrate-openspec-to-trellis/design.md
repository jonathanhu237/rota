# Design: OpenSpec to Trellis Migration

## Architecture and Ownership

After cutover, project knowledge has four distinct owners:

| Concern | Active owner |
|---|---|
| Development phases and task policy | `.trellis/workflow.md` |
| Language/layer conventions | `.trellis/spec/backend/`, `frontend/`, `guides/` |
| Product behavior contracts | `.trellis/spec/domain/` |
| Change intent and execution record | `.trellis/tasks/` and its archive |

`AGENTS.md` is the concise entry point and points to those owners. It retains
commands, done criteria, configuration rules, and per-language references, but
does not duplicate the full Trellis workflow.

## Domain Contract Migration

Create `.trellis/spec/domain/index.md` and one Markdown file for each current
OpenSpec capability:

```text
attendance.md
audit.md
auth.md
branding.md
dev-tooling.md
frontend-shell.md
outbox.md
scheduling.md
```

The body of each file starts from the corresponding
`openspec/specs/<capability>/spec.md`. The Requirement/Scenario structure is
already executable and remains valid in Trellis, so it is preserved rather
than rewritten into a new schema.

Five source specs contain an archived placeholder Purpose. Their Trellis copies
receive accurate purpose summaries based on the existing requirement headings:
attendance, branding, dev-tooling, frontend-shell, and outbox. No requirement
or scenario semantics change.

Parity verification compares requirement/scenario heading counts and checks the
text from `## Requirements` onward after normalizing terminal blank lines. The
expected content differences are the five purpose summaries plus the domain
index; all requirement/scenario bodies otherwise remain text-identical.

## Legacy Boundary

Keep `openspec/specs/`, `openspec/changes/archive/`, and
`openspec/config.yaml` in place. Add a read-only notice in
`openspec/README.md` and at the top of the config. This preserves blame, links,
screenshots, designs, and the original 31-change narrative.

Remove the tracked project-local OpenSpec execution adapters for both supported
hosts: five `.codex/skills/openspec-*` skills, five
`.claude/skills/openspec-*` skills, and five `.claude/commands/opsx/*`
commands. They are operational workflow adapters, not historical records, and
leaving them installed would expose two competing execution systems.

Update only prospective wording in ADR 0002 and ADR 0003. Their decisions and
historical context remain unchanged.

## Workflow Customization

Retain Trellis's generated parser-sensitive workflow-state blocks, but update
the surrounding and breadcrumb policy consistently:

- no task is required for conversation, investigation, or a clearly small
  low-risk direct patch;
- material changes require user consent to create a task;
- complex tasks require PRD, design, and implementation plan;
- implementation starts only after `task.py start`;
- only one active task is allowed per checkout;
- material tasks record a dedicated branch and base branch;
- inline Codex loads `trellis-before-dev`, edits, then runs `trellis-check`;
- behavior drift is fixed in the task/domain artifact before code;
- validation runs on Centaurus after one-way synchronization;
- commits are local Conventional Commits; no automatic task/journal commits.

Set `session_auto_commit: false` to prevent Trellis helpers from staging or
committing mixed user changes. Set `codex.dispatch_mode: inline` so the main
session owns implementation and check context.

## Centaurus Data Flow

```text
local working tree (authoritative)
        |
        | rsync --archive --delete, explicit excludes
        v
/home/jonathanhu237/code/rota (disposable validation mirror)
        |
        +--> mise-managed Go toolchain --> build/vet/test
        |
        +--> mise-managed Node/pnpm --> frozen install/lint/test/build
```

The rsync target is an explicit project directory created for this migration.
The command excludes `.git/`, `.env`, `*.local.*`, certificates/keys,
dependency and build output, Python caches, Trellis runtime session files, and
local Trellis update/cache state such as `.template-hashes.json`. `--delete`
applies only inside that verified remote mirror so stale validation files cannot
mask a local deletion.

Centaurus never writes back to the local checkout and performs no Git
operations. Additional tools are installed or invoked through its existing
`/home/jonathanhu237/.local/bin/mise`.

## Compatibility and Rollback

This migration changes documentation and agent workflow only. Runtime and data
compatibility are unchanged.

Rollback is Git-based:

1. restore the pre-migration `AGENTS.md`, OpenSpec skills, and ADR wording;
2. stop treating `.trellis/` as authoritative;
3. retain the untouched OpenSpec specs/archive;
4. resynchronize the reverted local tree to the same disposable Centaurus
   mirror.

Because OpenSpec history is not deleted or transformed, rollback does not
require reconstructing any product contract.
