# OpenSpec Legacy Archive

Rota migrated its active spec-driven development workflow from OpenSpec to
Trellis on 2026-07-30.

This directory is retained read-only for historical traceability:

- `specs/` is the capability snapshot used as the source for the initial
  `.trellis/spec/domain/` contracts.
- `changes/archive/` contains the original proposals, designs, task lists,
  delta specs, screenshots, and other evidence explaining how the product
  reached that snapshot.
- `config.yaml` records the former OpenSpec workflow and is no longer an active
  instruction source.

Do not create, apply, verify, or archive new OpenSpec changes. New material work
uses `.trellis/tasks/`; current behavior contracts live in
`.trellis/spec/domain/`; development policy lives in `.trellis/workflow.md`.

When legacy OpenSpec content and active Trellis content differ, Trellis is
authoritative. Preserve this directory so old commits, ADRs, and investigations
can continue to link to the original record.
