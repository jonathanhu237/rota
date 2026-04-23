## Why

Real-world schedules (e.g., our current production rota, `使用时间 2026-03-02 ~ 2026-05-03`) are structured as a **list of time slots per weekday**, where each slot carries a **team composition** of multiple positions with their own required headcounts. A single weekday can have 4–5 disjoint time slots; different slots can have completely different position compositions (daytime slot = `{前台负责人, 校园卡, 网络岗}`, evening slot = `{外勤负责人, 外勤岗}`).

Our current schema flattens this two-level structure into a single table — `template_shifts (weekday, start_time, end_time, position_id, required_headcount)` — where the "time slot" is an implicit grouping by `(weekday, start_time, end_time)` rather than a first-class entity. This leaks through in three ways that we have either already paid for or are about to:

1. **A whole class of integrity bugs is structurally unpreventable.** Admins can create assignments that double-book the same user across overlapping slots; admins can define overlapping shifts for the same user; a user can hold two positions in the same time slot with no constraint stopping it. These cannot be fixed by a service-layer check alone without losing expressiveness — the model itself allows them.
2. **Admin ergonomics are poor.** A modest schedule produces ~74 individual `template_shift` rows with no "slot" to operate on. Bulk operations (copy a weekday, duplicate a slot's composition, delete an entire slot) have no natural API shape.
3. **Future extensions (recurrence beyond weekly, slot-level constraints, slot-level audit) have no structural foothold** — they all degenerate into "scan a bunch of rows and hope the implicit grouping holds."

We have **no external users and no production data**, so the cost of a clean migration is at its historical minimum. Deferring the refactor makes every subsequent feature more expensive.

## What Changes

- **BREAKING** — Replace `template_shifts` with a two-table structure:
  - `template_slots` — the time container: `(id, template_id, weekday, start_time, end_time)`.
  - `template_slot_positions` — the per-slot position requirement: `(id, slot_id, position_id, required_headcount)`.
- **BREAKING** — Change the `assignments` shape: `template_shift_id` is replaced by `(slot_id, position_id)`; the natural key becomes `UNIQUE(publication_id, user_id, slot_id)` so a user can hold at most one position per slot.
- Add a database-level exclusion constraint forbidding time-overlap between slots of the same `(template_id, weekday)`.
- Add a service-layer time-overlap check to `CreateAssignment`: reject if the new assignment would leave the target user with any two assignments whose slots overlap on the same weekday. Admin path was previously unchecked (known bug); shift-change apply path was already correct and keeps its existing check.
- Rewrite the `scheduling` spec sections whose wording is shaped around the old `template_shift` concept. In the same pass, resolve the existing spec-level drift where the archived `Assignment window` requirement still claims `ASSIGNING`-only even though `admin-shift-adjustments` widened the contract to `{ASSIGNING, PUBLISHED, ACTIVE}`.
- Refresh the `audit` spec's metadata rules so audit events for assignment mutations reference `slot_id + position_id` rather than `template_shift_id`.
- Rework the assignment-board / roster / shift-change handler responses to group by `slot` and list its `positions[]` — frontend types and UI components update accordingly.
- Big-bang migration: one goose migration moves the schema, a single PR cuts over the backend + frontend + specs in one step.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `scheduling`: replaces `template_shift` with the `slot + slot_position` pair throughout (data model, CRUD endpoints, assignment shape, auto-assign MCMF, assignment-board response, roster response, shift-change request references); adds slot-level DB constraints (non-overlapping slots per weekday, one-position-per-user-per-slot); adds admin-path time-overlap check; rewrites the `Assignment window` requirement to match the already-widened `{ASSIGNING, PUBLISHED, ACTIVE}` mutable state window. Assignment-related audit event metadata emits `slot_id + position_id` instead of `template_shift_id`; this is a scheduling-side change because the audit capability's requirements are about the audit log's structure and rules, not about the specific metadata keys scheduling chooses to emit.

## Non-goals

Out of scope for this change — each is a deliberate "no":

- **Recurrence beyond weekly.** Slots remain `weekday`-keyed; bi-weekly, monthly, or date-specific slots are not introduced here.
- **Cross-publication slot sharing or slot-template library.** Each template owns its own slots; slots are not deduplicated across templates.
- **Slot-level "fully staffed" hard constraint.** Understaffing remains allowed (as today for `template_shift` headcount) — we do not start blocking `PUBLISHED` transitions on partial slot coverage.
- **Restructuring the publication state machine.** No new states, no new transitions; only the wording that mentions `template_shift` changes.
- **Adding per-assignment time overrides** (e.g., "Alice covers 09:00–10:00 only of this slot"). An assignment still means "this user holds this position in this entire slot."
- **Phased / dual-writing migration.** We have no production data to preserve; big-bang is cheaper and cleaner, at the cost of a single large PR.
- **Frontend admin bulk-operations UI.** This change keeps the existing one-row-at-a-time admin UX; it only makes a future bulk UI structurally possible.
- **Per-weekday or per-slot audit bundling.** Every per-row audit event stays per-row; no new aggregate event shapes.

## Impact

- **Database**: one goose migration. Drops `template_shifts`; creates `template_slots` and `template_slot_positions`; adds a PostgreSQL `EXCLUDE USING gist` constraint on `template_slots` for `(template_id, weekday, tsrange(start_time, end_time))` to forbid within-weekday overlap; rewrites `assignments` to carry `(slot_id, position_id)` with `UNIQUE(publication_id, user_id, slot_id)`; adjusts FK cascades.
- **Backend**:
  - `backend/internal/model/`: new types for `TemplateSlot` and `TemplateSlotPosition`; `Assignment` and `AssignmentCandidate` shapes change.
  - `backend/internal/repository/`: every query touching `template_shifts` or `assignments.template_shift_id` rewrites — `assignment.go`, `template.go`, `publication.go`, `shift_change.go`.
  - `backend/internal/service/`: `publication_pr4.go` (state guard already widened; now gains the overlap check and new response shapes), `shift_change.go` (request rows now reference slot/position), auto-assign MCMF (graph construction retargets to slot-position pairs).
  - `backend/internal/handler/`: response JSON for assignment-board, roster, shift-change detail; error taxonomy gains `ASSIGNMENT_TIME_CONFLICT`.
  - `backend/internal/audit/`: new metadata keys on assignment.create / assignment.delete; old `template_shift_id` key removed.
- **Frontend**:
  - `frontend/src/lib/types.ts`: `AssignmentBoardShift` → `AssignmentBoardSlot` with `positions: AssignmentBoardPosition[]`.
  - `frontend/src/components/assignments/*`: board UI groups by slot, renders each slot's position list and its candidates / assignments.
  - `frontend/src/components/requests/*`: swap/give display references slot + position.
  - `frontend/src/lib/api-error.ts`: add `ASSIGNMENT_TIME_CONFLICT`.
  - `frontend/src/i18n/locales/{en,zh}.json`: copy that mentioned "shift" in the template/slot sense is retargeted to "slot"; new error copy added.
- **Specs**: `openspec/specs/scheduling/spec.md` — ~15 requirements rewrite or modify; `openspec/specs/audit/spec.md` — metadata-rules requirement modifies. Delta spec captures both via `## MODIFIED Requirements` / `## REMOVED Requirements` / `## ADDED Requirements` as appropriate.
- **No data migration required**: no production data to preserve. Local dev DBs reset.
- **No new dependencies**: Postgres `btree_gist` extension is already available in stock Postgres; GIST exclusion constraint uses it. No new Go modules, no new npm packages.
