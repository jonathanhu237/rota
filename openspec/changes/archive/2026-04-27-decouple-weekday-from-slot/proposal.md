## Why

The template-detail page reads as three nested accordions (template → weekday → time slot → position composition) because the underlying data model bakes `weekday` into the slot's identity. A slot the user *thinks* of as "09:00-10:00, 1 前台负责人 + 2 前台助理, runs Mon-Fri" is stored as **five** `template_slots` rows + **ten** `template_slot_positions` rows — composition repeated five times. Every edit to that composition must fan out to all five rows; the UI has no choice but to surface that fan-out, which is what produced the cluttered "星期一 / 星期二 / 星期三 …" view that prompted this discussion.

This was a misalignment between the original design intent and the schema. The intent: a slot is a `(start_time, end_time, position composition)` unit that has a *set of weekdays it applies to*. Make the schema reflect that, and the UI redundancy collapses on its own.

## What Changes

### Schema

- **BREAKING** — Drop `weekday` from `template_slots`. Slot identity becomes the slot row itself: `(start_time, end_time, position composition)` plus a weekday set.
- **BREAKING** — Introduce `template_slot_weekdays(slot_id BIGINT, weekday INTEGER)` join table with PK `(slot_id, weekday)`, `weekday BETWEEN 1 AND 7` CHECK, FK to `template_slots(id) ON DELETE CASCADE`. A slot's set of applicable weekdays is the set of rows in this table referencing it.
- **BREAKING** — Add `weekday INTEGER` to `availability_submissions` and `assignments`. Both are NOT NULL with `weekday BETWEEN 1 AND 7` CHECK. Natural keys become `availability_submissions UNIQUE(publication_id, user_id, slot_id, weekday)` and `assignments UNIQUE(publication_id, user_id, slot_id, weekday)`. Both new columns reference `template_slot_weekdays(slot_id, weekday)` to guarantee submissions/assignments only exist for `(slot, weekday)` cells the slot actually covers.
- **BREAKING** — Replace the GIST exclusion on `template_slots(template_id, weekday, tsrange)` with a new constraint that holds across the new join: for any given `(template_id, weekday)`, no two slots whose time ranges overlap may both have that weekday in their weekday set. Implemented as a trigger function on `template_slot_weekdays` insert/update (Postgres GIST cannot span tables). The trigger SHALL produce the same SQLSTATE used today so the existing `ErrTemplateSlotOverlap` translation path keeps working. Slots with the same time range and disjoint weekday sets MAY coexist.
- The `assignments_position_belongs_to_slot` trigger SHALL be reused as-is — slot-position membership is unchanged.

### Service & handler layer

- Template service: slot create/update/delete now manages a slot's weekday set via the join table. New endpoint shape on `POST /templates/{id}/slots` and `PATCH /templates/{id}/slots/{slot_id}` accepts a `weekdays: int[]` field; old `weekday: int` field SHALL be removed (BREAKING — no compat shim, the API is unreleased).
- Availability service: submission create/delete operates on `(publication_id, user_id, slot_id, weekday)`. Qualification check unchanged (still keyed on slot composition).
- Auto-assigner: the demand cell is `(slot, weekday, position)` instead of `(slot, position)`; per-weekday overlap groups derive directly from the submission's weekday column instead of the slot's weekday column. The MCMF graph topology is unchanged.
- Assignment board, roster, shift-change, leave: every code path that reads `slot.Weekday` becomes `assignment.Weekday` / `submission.Weekday`. Shift-change `expires_at` derivation uses the assignment's weekday instead of the slot's.
- Roster's `(slot, occurrence_date)` semantics: occurrence date's calendar weekday must match the assignment's weekday column (not the slot's). Validation logic shifts but the user-visible behavior is identical.

### Frontend

- Template detail page is re-grouped around the new unit: a flat list of slots ordered by `(start_time, end_time)`, each slot showing its position composition once and rendering applicable weekdays as 7 chips/checkboxes (Mon-Sun). The "星期一 / 星期二 …" weekday accordion goes away; the "新增时段" duplication goes away with it.
- Availability submission page: still shows a (weekday × time) grid, but each cell maps to a `(slot_id, weekday)` pair rather than a slot id. The user-visible interaction is unchanged.
- Assignment board, roster table, shift-change UI: keep their current presentation but read weekday from the new column.

### Data migration

Pre-production. The migration is destructive: it `TRUNCATE`s `template_slots`, `template_slot_positions`, `availability_submissions`, `assignments`, `shift_change_requests`, and `leaves` (in that dependency order, or as one `TRUNCATE … CASCADE`), then drops the old `weekday` column + GIST exclusion, creates `template_slot_weekdays`, adds `weekday` to `availability_submissions` and `assignments`, and installs the new overlap trigger. There is no backfill. The dev re-runs `make seed SCENARIO=…` after `make migrate-up`.

### Seeding

- `make seed SCENARIO=realistic` produces 5 `template_slots` (one per time block) + 35 `template_slot_weekdays` rows (5 × 7) + 10 `template_slot_positions` rows (4 daytime × 2 + 1 evening × 2). The previous shape was 35 + 70 + 0 — about a 5× row reduction with no behavior change. The `availability_submissions` count is unchanged (still ~hundreds of `(slot, weekday)` cells).
- `make seed SCENARIO=full` produces 8 logical slots (was 10 per-weekday rows that already represented the same logical surface in `fullSlotDefinitions()`); composition-equivalent slots are coalesced.
- `make seed SCENARIO=stress` similarly coalesces.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `scheduling`: the *Template and shift data model* requirement (and most of its scenarios), the *Template CRUD and shift CRUD* request/response shapes, the *Availability submissions* table shape, the *Assignments table shape*, the *Auto-assigner graph construction* requirement, the *Shift change request data model* (`expires_at` derivation), and the *Occurrence model* requirement (`(slot, occurrence_date)` becomes `(slot, weekday, occurrence_date)` in validation).
- `dev-tooling`: the *Local-development data seeding command* requirement updates row counts in the `realistic` scenario block (5 / 35 / 10 instead of 35 / 70) and the `full` scenario block (slot count down from ~10 to whatever the coalesced count is — design.md will pin the number after the coalesce pass).

## Impact

- **Backend code**: large.
  - `migrations/00016_decouple_weekday_from_slot.sql` (new).
  - `backend/internal/repository/template_repo*.go`, `availability_*.go`, `assignment_*.go`, `shift_change_*.go`, `leave_*.go`, `publication_*.go` (queries change shape).
  - `backend/internal/service/autoassign.go`, `availability.go`, `assignment.go`, `shift_change.go`, `leave.go`, `publication_pr*.go`, `template.go` (every site reading `slot.Weekday`).
  - `backend/internal/handler/template.go`, `availability.go`, `assignment.go`, `shift_change.go`, `roster.go` (request/response shapes).
  - `backend/internal/model/template.go`, `availability_submission.go`, `assignment.go`, `publication.go` (struct fields).
  - `backend/cmd/seed/scenarios/{full,stress,realistic}.go` (slot/weekday/composition reshaped; realistic.go regenerated by `generate_realistic_seed.py`).
  - Integration tests across all of the above.
- **Frontend code**: significant.
  - `frontend/src/routes/templates.$id.tsx` (or equivalent) — full page rewrite.
  - `frontend/src/routes/availability/*` — payload shape per cell.
  - `frontend/src/routes/assignments/*`, `shift-changes/*`, `roster/*` — read weekday from the new field.
  - Zod schemas under `frontend/src/api/*` and TanStack Query hooks under `frontend/src/queries/*`.
- **Specs**: `scheduling/spec.md` heavily rewritten (mostly textual — the requirements are mostly the same, the field they reference moves); `dev-tooling/spec.md` row counts updated.
- **No new third-party dependencies. No infra changes.**
- **No backwards-compatibility shims.** The API is local-only and the schema is local-only; the migration is destructive on dev databases.

## Non-goals

- **Per-weekday composition variation.** A single slot has exactly one position composition that applies to every weekday in its set. If a real cohort needs "Mon morning needs 2 assistants but Sat morning needs 3," the model SHALL be: two separate slots with the same time range and disjoint weekday sets. The redesigned overlap trigger permits this (the time ranges only collide on weekdays where both slots claim the day, which by construction can't happen).
- **Per-occurrence-date assignment rows.** The recurrence-pattern model stays (one `assignments` row per `(publication, user, slot, weekday)` covers every weekly occurrence in the publication's active window). Per-occurrence overrides are a separate, much larger conversation.
- **Preserving existing dev data through the migration.** No production yet; the migration is destructive. Re-seeding is the recovery path.
- **UI for editing weekday-set bulk operations** beyond the seven Mon-Sun chips per slot. No "copy from another slot," no "apply to all slots in template" — out of scope.
- **Changing availability submission semantics.** Submissions stay per `(slot, weekday)`; the qualification check stays per slot composition.
- **Changing the auto-assigner's algorithm or coverage objective.** Demand cell shape changes, MCMF graph construction stays.
- **Backwards compatibility / dual-write phases.** The change is one shipping unit; no feature flag, no compat layer, no soft cutover.

## Risks / safeguards

- **Risk: trigger-based overlap exclusion is slower than GIST.** Templates rarely change; the trigger fires on `template_slot_weekdays` insert and does an indexed lookup. Acceptable for a 7-row-per-slot edit pattern.
- **Risk: dev forgets to re-seed after the migration and hits empty tables.** Mitigation: the migration's `goose Up` SQL prints (via `RAISE NOTICE`) a "run `make seed` to repopulate" hint, and the apply task runs `make seed SCENARIO=basic` immediately after `make migrate-up` as part of the smoke check.
- **Risk: the auto-assigner test suite (which is large) goes red because every fixture changes shape.** Mitigation: the integration test fixture builders move to the new shape in the same change; tests assert the same outcomes.
- **Risk: the spec rewrite is mechanical (rename `slot.weekday` → `submission.weekday` / `assignment.weekday`) but extensive.** Mitigation: design.md catalogs every scenario that needs a textual update so review can be a diff-only pass.
