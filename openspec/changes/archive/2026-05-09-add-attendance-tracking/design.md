## Context

Rota already models a publication as a weekly baseline roster plus occurrence-level overrides from shift changes and leave coverage. Attendance must therefore operate on concrete occurrences, not on template slots alone or baseline assignments alone. The shift leader is not currently represented as a first-class concept; the closest durable concept is the template slot's position composition.

The user-facing workflow is intentionally operational: the responsible person opens an attendance page during their current shift, records arrivals, and can add overtime entries for that same shift within the publication's configured overtime entry window. Administrators need a separate management surface for corrections and missed leader actions.

## Goals / Non-Goals

**Goals:**

- Represent exactly one attendance-responsible position entry per template slot.
- Record immutable leader arrival entries for concrete roster occurrence users.
- Derive pending, present, late, and absent status from roster, shift time, and arrival rows.
- Record overtime independently from roster changes as user + decimal hours + required note.
- Let administrators correct arrivals, manage overtime records, and configure publication overtime entry windows.
- Preserve existing publication, assignment, occurrence override, leave, and audit invariants.

**Non-Goals:**

- Attendance export, payroll, wage calculations, and grace-period calculations.
- Employee self check-in, geofence/device flows, QR codes, or push-style reminders.
- Backup leaders, multiple leader positions, or leader handoff workflows.
- Automatic excused-state derivation from leave records.
- New external runtime dependencies.

## Decisions

### D1. Responsible position marker

Add `template_slot_positions.attendance_responsible BOOLEAN NOT NULL DEFAULT FALSE`. Exactly one position entry per slot is the attendance-responsible entry, and that entry must have `required_headcount = 1`.

The backend enforces the invariant on slot-position create/update/delete flows, template detail responses expose the marker, and the frontend template slot-position editor lets administrators choose one responsible position per slot. Publication attendance entry is unavailable for slots that violate the invariant, which also protects existing publications whose templates predate the marker.

Alternative rejected: add a separate `slot_leaders` table keyed by slot. It duplicates the slot-position composition and would need separate synchronization when positions are deleted or slot composition changes.

### D2. Occurrence-based attendance reads

Attendance views compute current actual roster users using the same model as roster reads: `assignments` plus `assignment_overrides` for `(assignment_id, occurrence_date)`. Arrival writes re-check that the target user is still the actual roster user for that assignment occurrence. If a leader loaded stale data and the occurrence was reassigned, the write is rejected and the client refreshes.

Alternative rejected: store a denormalized roster snapshot for every publication week. Existing specs intentionally avoid materialized weekly roster rows; attendance can reuse the existing occurrence computation without changing that invariant.

### D3. Arrival records are write-once for leaders

`attendance_records` stores the current effective arrival value for a roster occurrence user. A leader `record arrival` call inserts one row and fails if a row already exists. Leaders cannot update or clear arrival rows. Administrators can upsert or clear rows through admin endpoints, with audit events recording old and new values.

Derived status is:

```
pending: no arrived_at and now < scheduled_end
absent:  no arrived_at and now >= scheduled_end
present: arrived_at <= scheduled_start
late:    arrived_at > scheduled_start
```

There is no stored status column. The service derives status in read models so late/absent behavior follows the current clock without background jobs.

Alternative rejected: store explicit status chosen by leaders. That creates a second source of truth and allows subjective late/absent decisions even though the user wants the system to decide from arrival time.

### D4. Overtime is independent from roster

`attendance_overtime_records` stores overtime as `user_id`, `hours`, required `note`, and the concrete shift context. It does not mutate `assignments` or `assignment_overrides`; emergency cover and post-shift extra work are both overtime facts rather than formal roster changes.

Leaders can create overtime records for any active user while they are the responsible person for the shift and the shift is still inside its overtime entry window. Overtime save is final for leaders. Administrators can create, update, or delete overtime records at any time.

Alternative rejected: model emergency cover through `assignment_overrides`. That would rewrite the actual roster, disturb absence derivation for the originally scheduled user, and blur a late operational overtime fact with an approved schedule transfer.

### D5. Publication-scoped overtime window

Add `publications.overtime_entry_window_hours NUMERIC(5,2) NOT NULL DEFAULT 24.00`. The window is configured per publication because the user asked for the setting on the "班表" rather than globally. Leader overtime permission is valid when:

`scheduled_start <= now <= scheduled_end + overtime_entry_window_hours`

This setting does not extend leader arrival permission. Leader arrival recording is valid only while:

`scheduled_start <= now < scheduled_end`

Alternative rejected: environment variable or global settings table. This is operational policy tied to a concrete publication and should not retroactively change historical publications.

### D6. Admin management is the correction surface

Admins get `/publications/:publicationId/attendance`. The page lists occurrence shifts by date, opens a shift detail, displays current roster attendance statuses, exposes orphan records that no longer match current roster after schedule changes, and manages overtime records. Admin writes are separate endpoints and audit actions so leader writes remain clearly distinguishable.

Alternative rejected: rely on direct database edits for admin correction. Attendance data is operationally sensitive enough to need validation, UI feedback, and audit events.

### D7. No new dependencies

The implementation uses existing Go, PostgreSQL, TanStack Query, router, shadcn/ui, and current test tooling.

Rejected dependency: adding a scheduler/cron package or `pg_cron` for absence materialization. Absence is derived on read, so no background job is needed.

## Schema

Goose Up SQL:

```sql
ALTER TABLE template_slot_positions
    ADD COLUMN attendance_responsible BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX template_slot_positions_one_attendance_responsible_idx
    ON template_slot_positions (slot_id)
    WHERE attendance_responsible;

ALTER TABLE template_slot_positions
    ADD CONSTRAINT template_slot_positions_responsible_headcount_chk
        CHECK (attendance_responsible = FALSE OR required_headcount = 1);

ALTER TABLE publications
    ADD COLUMN overtime_entry_window_hours NUMERIC(5,2) NOT NULL DEFAULT 24.00,
    ADD CONSTRAINT publications_overtime_entry_window_hours_chk
        CHECK (overtime_entry_window_hours >= 0 AND overtime_entry_window_hours <= 168);

CREATE TABLE attendance_records (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES publications(id) ON DELETE CASCADE,
    assignment_id BIGINT NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
    occurrence_date DATE NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    arrived_at TIMESTAMPTZ NOT NULL,
    recorded_by_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (assignment_id, occurrence_date, user_id)
);

CREATE INDEX attendance_records_publication_occurrence_idx
    ON attendance_records (publication_id, occurrence_date);
CREATE INDEX attendance_records_user_idx
    ON attendance_records (user_id, occurrence_date);

CREATE TABLE attendance_overtime_records (
    id BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES publications(id) ON DELETE CASCADE,
    slot_id BIGINT NOT NULL REFERENCES template_slots(id) ON DELETE CASCADE,
    weekday INTEGER NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    occurrence_date DATE NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    hours NUMERIC(5,2) NOT NULL CHECK (hours > 0 AND hours <= 24),
    note TEXT NOT NULL CHECK (btrim(note) = note AND char_length(note) BETWEEN 1 AND 500),
    recorded_by_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX attendance_overtime_publication_occurrence_idx
    ON attendance_overtime_records (publication_id, occurrence_date);
CREATE INDEX attendance_overtime_user_idx
    ON attendance_overtime_records (user_id, occurrence_date);
```

Goose Down SQL:

```sql
DROP INDEX IF EXISTS attendance_overtime_user_idx;
DROP INDEX IF EXISTS attendance_overtime_publication_occurrence_idx;
DROP TABLE IF EXISTS attendance_overtime_records;

DROP INDEX IF EXISTS attendance_records_user_idx;
DROP INDEX IF EXISTS attendance_records_publication_occurrence_idx;
DROP TABLE IF EXISTS attendance_records;

ALTER TABLE publications
    DROP CONSTRAINT IF EXISTS publications_overtime_entry_window_hours_chk,
    DROP COLUMN IF EXISTS overtime_entry_window_hours;

DROP INDEX IF EXISTS template_slot_positions_one_attendance_responsible_idx;
ALTER TABLE template_slot_positions
    DROP CONSTRAINT IF EXISTS template_slot_positions_responsible_headcount_chk,
    DROP COLUMN IF EXISTS attendance_responsible;
```

## API Shape

- `GET /attendance/current` returns leader-visible shifts for the caller: currently running arrival windows and overtime windows still open.
- `POST /attendance/arrivals` records one arrival for a leader's current shift.
- `POST /attendance/overtime` records one locked overtime entry for a leader's eligible shift.
- `GET /publications/{id}/attendance?date=YYYY-MM-DD` lists admin shift summaries for a date.
- `GET /publications/{id}/attendance/shifts/{slot_id}/{occurrence_date}` returns one admin shift detail.
- `PUT /publications/{id}/attendance/arrivals` admin upserts an arrival.
- `DELETE /publications/{id}/attendance/arrivals/{record_id}` admin clears an arrival.
- `POST /publications/{id}/attendance/overtime` admin creates overtime.
- `PATCH /publications/{id}/attendance/overtime/{record_id}` admin updates overtime.
- `DELETE /publications/{id}/attendance/overtime/{record_id}` admin deletes overtime.
- `PATCH /publications/{id}/attendance/settings` updates `overtime_entry_window_hours`.

## Error Codes

- `INVALID_REQUEST` (400): malformed payload, bad date, invalid hours, blank note, invalid overtime window.
- `INVALID_OCCURRENCE_DATE` (400): requested occurrence does not belong to the publication slot/date window.
- `PUBLICATION_NOT_FOUND` (404): publication missing.
- `TEMPLATE_SLOT_NOT_FOUND` (404): slot missing for publication template.
- `USER_NOT_FOUND` (404): target user missing, disabled where active is required, or hidden from the operation.
- `ATTENDANCE_RECORD_NOT_FOUND` (404): admin references a missing attendance or overtime record.
- `ATTENDANCE_NOT_LEADER` (403): caller is not the responsible user for that concrete shift occurrence.
- `ATTENDANCE_WINDOW_CLOSED` (409): leader arrival or overtime write is outside the permitted window.
- `ATTENDANCE_ALREADY_RECORDED` (409): leader attempts to record an arrival for an already recorded occurrence user.
- `ATTENDANCE_ROSTER_STALE` (409): write target no longer matches the current actual roster for the occurrence.
- `ATTENDANCE_RESPONSIBLE_REQUIRED` (409): template/publication operation would leave a slot without exactly one responsible position.
- `INTERNAL_ERROR` (500): unexpected failures.

## Risks / Trade-offs

- Existing templates have no responsible marker → Attendance entry stays unavailable until admins configure templates; seed data can mark `负责人` positions for local demos.
- Orphan arrival records can appear after admin schedule changes → Admin UI surfaces them separately and exports are out of scope, so no automatic migration is needed.
- Status is clock-derived, so tests need controlled clocks → Use existing service clock interface and deterministic handler tests.
- Leader overtime can reference any active user → This matches the user's emergency-cover requirement; audit and mandatory notes provide accountability.
- Unique responsible marker index enforces at most one leader, not at least one → Service/template validation enforces exactly one where attendance-enabled behavior matters.

## Migration Plan

1. Add migration `00021_add_attendance_tracking.sql` with the Up/Down SQL above.
2. Update backend models, repository queries, service validation, audit constants, and handlers.
3. Update frontend types, query helpers, routes, navigation, and UI.
4. Update seed scenarios so demo templates mark one responsible position per slot when position names contain `负责人`.
5. Run backend and frontend verification commands from `tasks.md`.

Rollback uses the Down SQL. It drops attendance/overtime data and removes the two new scheduling columns; this is acceptable for local rollback and should be treated as destructive in production.

## Open Questions

- None for first-version scope. Attendance export and grace handling are intentionally deferred.
