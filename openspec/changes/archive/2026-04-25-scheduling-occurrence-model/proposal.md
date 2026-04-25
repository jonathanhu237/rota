## Why

The shift-change spec currently treats an `assignment` row as "Alice fills slot Mon 09-12 Position A throughout this publication". Approving a `give_pool` rewrites `assignments.user_id` to Bob, which means Bob now permanently holds *every* Mon 09-12 occurrence for the rest of the publication, not just the one week Alice asked help for. This conflicts with the everyday meaning of "swap a class" or "cover next Monday only", and it directly blocks the planned `add-leave` feature — we cannot model "Alice is out 5/11 only" without a per-occurrence concept. The discovery surfaced during the leave design exploration; once seen, it is also a latent correctness issue for the existing shift-change flow.

A second, related problem is that `publications` only has `planned_active_from` — no planned end date. The duration of a publication is implicit in admin-issued `POST /end` calls. This makes "how many weeks long is this publication?" and "what concrete dates does each slot occur on?" ambiguous to the UI and to any feature that needs to enumerate occurrences.

Both issues stem from the same root: the scheduling model has no first-class concept of a *concrete week occurrence* of a slot. This change introduces it. There is no production data yet, so the migration is straightforward.

## What Changes

- **BREAKING (DB schema):** `publications` adds `planned_active_until TIMESTAMPTZ NOT NULL`. The window CHECK becomes `submission_start_at < submission_end_at <= planned_active_from < planned_active_until`. All seed/test fixtures need updating.
- Effective-state resolution gains an "ended by clock" rule: a stored `ACTIVE` publication whose `NOW() >= planned_active_until` resolves to effective `ENDED`.
- `PATCH /publications/{id}` is introduced; admin can extend or shorten a publication by changing `planned_active_until`. Reopening an "ended-by-clock" publication is just pushing `until` further into the future.
- `POST /publications/{id}/end` is reframed as syntactic sugar for `PATCH ... { planned_active_until: NOW() }`. Existing callers continue to work; semantics are unchanged.
- New `assignment_overrides` table: `(assignment_id, occurrence_date, user_id)` records "this specific occurrence belongs to a different user than the baseline assignment row says". Roster reads layer overrides on top of assignments.
- `shift_change_requests` adds `occurrence_date DATE NOT NULL`. The natural key for a request becomes `(requester_assignment_id, occurrence_date)`. Approving a swap or give writes one or two override rows; it no longer mutates the `assignments` table.
- `shift_change_requests.expires_at` is derived per request from the occurrence's actual start time (`planned_active_from + (weekday-1) days + slot.start_time`), not from the publication's `planned_active_from`.
- D2 invariant (single non-ENDED publication) is preserved via a "sweep before create": when admin creates a new publication, the service first transitions any `state=ACTIVE` publication whose `NOW >= planned_active_until` to `state=ENDED` in a write-through, then enforces the partial unique index.
- Frontend shift-change UI makes occurrence explicit by adding week navigation to the roster and passing the selected concrete week into give/swap requests.
- Roster read (`GET /publications/{id}/roster`) accepts a `?week=YYYY-MM-DD` parameter and returns the concrete week with overrides applied.
- No data migration: confirmed during exploration that no production publications exist yet.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `scheduling`: introduces the occurrence concept and the supporting `assignment_overrides` table, redefines `shift_change_requests` to be occurrence-level, adds `planned_active_until` to publications with a time-driven ENDED transition and a `PATCH` endpoint, and reroutes the existing `POST /end` and roster reads through the new model.

## Non-goals

- **Leave functionality.** The `add-leave` change is the next step and depends on this one. This change deliberately ships the architecture without any user-visible new feature.
- **Materializing per-occurrence rows in `assignments`.** Assignments stay one-row-per-baseline; overrides are sparse exceptions. The model treats the baseline as authoritative and overrides as deltas.
- **Changing auto-assign behavior.** Auto-assign continues to operate at the `assignments` level (it produces the weekly baseline). Occurrence-level adjustments are exclusively manual or shift-change-driven.
- **Cross-publication shift changes.** A request still references a single publication. The occurrence_date must fall inside that publication's `[planned_active_from, planned_active_until)` window.
- **Background jobs to advance ENDED state.** Lazy resolution on read and the on-create sweep are the only mechanisms — no cron, no poller.
- **Retroactive override of past occurrences.** Requests can only target an occurrence whose actual start time is `> NOW()` at creation.
- **Per-occurrence audit replay.** Audit records the override write; we do not synthesize per-occurrence audit events for the unchanged baseline.

## Impact

- **Backend code**:
  - New migration: adds `publications.planned_active_until`, creates `assignment_overrides`, alters `shift_change_requests` with `occurrence_date` and the new natural-key uniqueness, removes the `expires_at = planned_active_from` default (now caller-provided per request).
  - Service layer: rewrites `ApplySwap` / `ApplyGive` to write overrides instead of mutating `assignments.user_id`; rewrites the cascade-invalidate logic to span occurrences; reroutes `POST /end` through the new PATCH path; adds the on-create D2 sweep.
  - Repository layer: new `AssignmentOverride` repo with CRUD plus the "for-roster" join query; updated `ShiftChange` repo for the new natural key and `occurrence_date`; updated `Roster` queries to layer overrides.
  - Handler layer: new `PATCH /publications/{id}`; updated `POST /publications/{id}/shift-changes` request body (carries `occurrence_date`); updated `GET /publications/{id}/roster` with the `week` query param.
- **Frontend code**:
  - Shift-change creation flow: occurrence selected from the roster week being viewed.
  - Shift-change list / detail: surface the concrete `occurrence_date` next to the slot.
  - Publication admin view: show and edit `planned_active_until`.
  - Roster view: week-by-week navigation within a publication.
- **Tests**:
  - Heavy: every publication-creating test fixture needs `planned_active_until`. Service-test and integration-test helpers need updating.
  - New: `assignment_overrides` repository integration tests; service tests for the new apply paths; effective-state resolution tests for the time-driven ENDED rule.
- **No new third-party dependencies.**
- **No cross-capability impact.** Auth/audit/dev-tooling unaffected.

## Risks / safeguards

- **Risk:** the D2 sweep introduces a write inside what callers expect to be a precondition check. **Mitigation:** the sweep runs only inside `POST /publications` (already a write path); the sweep itself is a single conditional `UPDATE ... WHERE state='ACTIVE' AND planned_active_until <= NOW()`.
- **Risk:** existing tests assume `expires_at = planned_active_from`. **Mitigation:** the new derivation is straightforward per-request; updates are mechanical.
- **Risk:** a pending request whose `occurrence_date` is in the past at activation (e.g., admin delays activation) is meaningless. **Mitigation:** existing "lazy expiry on read" already handles it under the new `expires_at` derivation.
- **Risk:** scope creep into add-leave territory. **Mitigation:** explicit non-goal and tasks.md will not include any leave artifacts.
