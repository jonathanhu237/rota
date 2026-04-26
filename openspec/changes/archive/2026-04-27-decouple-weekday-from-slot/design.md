## Context

Today's `template_slots` keys on `(template_id, weekday, start_time, end_time)`. A logical "slot" the operator thinks about — `09:00-10:00, {1×前台负责人, 2×前台助理}, Mon-Fri` — fans out to 5 `template_slots` rows + 10 `template_slot_positions` rows. The composition is duplicated on every weekday, downstream tables (`availability_submissions`, `assignments`) inherit the per-weekday shape via their `slot_id` FK, and the template-detail UI is forced to render a weekday-keyed accordion that surfaces the duplication. The user's original design intent — a slot is `(time, composition)` plus a *set of weekdays it applies to* — never made it into the schema.

This change re-shapes the model around the user's intent: weekday becomes a join-table membership of slots, downstream tables grow an explicit `weekday` column. The cosmetics of the template-detail UI fall out as a free consequence.

## Goals / Non-Goals

**Goals:**

- Slot identity is the slot row itself: `(start_time, end_time, position composition)` plus its weekday set; same-time slots may coexist when their weekday sets are disjoint.
- A slot's applicable weekdays are a set, exposed over the API as `weekdays: int[]`.
- `availability_submissions` and `assignments` carry an explicit `weekday` column; their natural keys widen to include it.
- The auto-assigner's candidate-cell unit becomes `(slot, weekday, position)` and per-weekday overlap groups derive from the submission's weekday rather than the slot's.
- The template-detail page renders one row per slot with a 7-cell weekday chip strip — no more weekday-keyed accordion.
- The migration is destructive on dev databases. The smoke flow is `make migrate-up && make seed SCENARIO=…`.

**Non-Goals:**

- Per-weekday composition variation. One slot has exactly one composition that applies to every weekday in its set. If a real cohort needs Mon ≠ Sat composition, model it as two separate slots with disjoint weekday sets.
- Per-occurrence-date assignment rows. The recurrence-pattern model stays — one `assignments` row per `(publication, user, slot, weekday)` covers every weekly occurrence.
- Backwards-compatibility for the old per-weekday slot shape. No feature flag, no compat shim.
- Preserving existing dev data through the migration. The migration TRUNCATEs and the dev re-seeds.
- Reworking the auto-assigner's algorithm or coverage objective. Only the demand-cell shape changes.
- Reworking the publication state machine, audit emission rules, or shift-change semantics beyond the mechanical rename of `slot.weekday` → `assignment.weekday` / `submission.weekday`.

## Decisions

### D-1. Schema shape

**Tables after migration:**

```
template_slots
    id BIGSERIAL PK
    template_id BIGINT FK → templates(id) ON DELETE CASCADE
    start_time TIME NOT NULL
    end_time TIME NOT NULL CHECK (end_time > start_time)
    created_at, updated_at TIMESTAMPTZ

template_slot_weekdays                                   -- NEW
    slot_id BIGINT FK → template_slots(id) ON DELETE CASCADE
    weekday INTEGER CHECK (weekday BETWEEN 1 AND 7)
    PRIMARY KEY (slot_id, weekday)
    UNIQUE          (slot_id, weekday)                   -- composite for FK targeting (see below)

template_slot_positions                                  -- unchanged
    id BIGSERIAL PK
    slot_id BIGINT FK → template_slots(id) ON DELETE CASCADE
    position_id BIGINT FK → positions(id) ON DELETE RESTRICT
    required_headcount INTEGER NOT NULL CHECK (required_headcount > 0)
    UNIQUE (slot_id, position_id)

availability_submissions
    id BIGSERIAL PK
    publication_id BIGINT FK ... ON DELETE CASCADE
    user_id BIGINT FK ... ON DELETE CASCADE
    slot_id BIGINT
    weekday INTEGER NOT NULL CHECK (weekday BETWEEN 1 AND 7)         -- NEW
    created_at TIMESTAMPTZ
    UNIQUE (publication_id, user_id, slot_id, weekday)               -- widened
    FOREIGN KEY (slot_id, weekday)
        REFERENCES template_slot_weekdays(slot_id, weekday)
        ON DELETE CASCADE                                            -- closes the loop

assignments
    id BIGSERIAL PK
    publication_id BIGINT FK ... ON DELETE CASCADE
    user_id BIGINT FK ... ON DELETE CASCADE
    slot_id BIGINT
    weekday INTEGER NOT NULL CHECK (weekday BETWEEN 1 AND 7)         -- NEW
    position_id BIGINT FK → positions(id) ON DELETE RESTRICT
    created_at TIMESTAMPTZ
    UNIQUE (publication_id, user_id, slot_id, weekday)               -- widened
    FOREIGN KEY (slot_id, weekday)
        REFERENCES template_slot_weekdays(slot_id, weekday)
        ON DELETE CASCADE                                            -- closes the loop
```

The composite FK from submissions/assignments into `template_slot_weekdays(slot_id, weekday)` is the tightest expression of "you can only submit/assign for `(slot, weekday)` cells the slot actually claims." It also handles the eviction case automatically: removing a weekday from a slot's set cascades and drops referencing submissions/assignments. (This is the same semantic as today's `ON DELETE CASCADE` from `template_slots`, just at the finer granularity the new model demands.)

Existing `assignment_overrides`, `shift_change_requests`, `leaves`, `assignments_position_belongs_to_slot` trigger, and `user_positions` are unchanged. The `assignments_position_belongs_to_slot` trigger checks `(slot_id, position_id) ∈ template_slot_positions`, which is still correct — composition is per-slot, not per-`(slot, weekday)`.

**Rejected alternative — `weekdays INT[]` on `template_slots`:**
Compact, but the `[]int → set` validation has to be hand-rolled (uniqueness, range, ordering), and the FK from submissions/assignments cannot point at an array element. Loses the cascade-on-weekday-removal semantic.

**Rejected alternative — leave weekday on `template_slots` but allow composition to be NULL when shared between weekdays:**
Effectively a soft normalization that doesn't change the storage shape. The duplication remains; the UI stays accordioned. No improvement.

### D-2. Overlap exclusion (trigger replaces GIST)

The current `template_slots_no_overlap_excl` GIST exclusion lives on `template_slots(template_id, weekday, tsrange)`. After the migration, weekday isn't on the same table, so a single-table GIST can't encode the constraint. We replace it with a trigger function on `template_slot_weekdays` insert/update that does the lookup explicitly.

**Trigger logic:**

```sql
CREATE FUNCTION template_slot_weekday_no_overlap()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    conflict_slot_id BIGINT;
BEGIN
    -- For the slot the new weekday-row references, check whether any other slot
    -- in the same template has overlapping start/end and also claims this weekday.
    SELECT other.id INTO conflict_slot_id
    FROM template_slots me
    JOIN template_slots other
        ON other.template_id = me.template_id
       AND other.id <> me.id
       AND tsrange(
               ('2000-01-01'::date + me.start_time)::timestamp,
               ('2000-01-01'::date + me.end_time)::timestamp,
               '[)'
           ) &&
           tsrange(
               ('2000-01-01'::date + other.start_time)::timestamp,
               ('2000-01-01'::date + other.end_time)::timestamp,
               '[)'
           )
    JOIN template_slot_weekdays other_wd
        ON other_wd.slot_id = other.id
       AND other_wd.weekday = NEW.weekday
    WHERE me.id = NEW.slot_id
    LIMIT 1;

    IF conflict_slot_id IS NOT NULL THEN
        RAISE EXCEPTION 'overlapping slot weekday'
            USING ERRCODE = '23P01';   -- exclusion_violation
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER template_slot_weekdays_no_overlap_trg
BEFORE INSERT OR UPDATE ON template_slot_weekdays
FOR EACH ROW EXECUTE FUNCTION template_slot_weekday_no_overlap();
```

We deliberately raise SQLSTATE `23P01` (`exclusion_violation`) — the same SQLSTATE GIST exclusions raise — so the existing repository's pq-error translation (`pqErr.Code == "23P01"` → `ErrTemplateSlotOverlap`) keeps working without any handler-layer churn. The handler still maps to HTTP 409 / `TEMPLATE_SLOT_OVERLAP`.

**Rejected alternative — runtime check at the service layer:**
Putting the overlap check in Go service code instead of the database means concurrent inserts (admin A and admin B editing the same template) can race past the check. Database-level enforcement is the only way to keep the constraint correctness-airtight.

**Rejected alternative — moving start_time/end_time onto the join table:**
Would let us use a single GIST exclusion on `(template_id, weekday, tsrange)` again. But it duplicates the time range across all 7 weekday rows of a slot, defeating the whole point of this change.

### D-3. Migration is destructive

`migrations/00016_decouple_weekday_from_slot.sql` (goose):

```sql
-- +goose Up
-- Pre-prod: discard all rows in tables coupled to the old slot shape.
TRUNCATE
    leaves,
    shift_change_requests,
    assignment_overrides,
    assignments,
    availability_submissions,
    template_slot_positions,
    template_slots
RESTART IDENTITY CASCADE;

-- New table.
CREATE TABLE template_slot_weekdays (
    slot_id  BIGINT  NOT NULL,
    weekday  INTEGER NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    PRIMARY KEY (slot_id, weekday),
    CONSTRAINT template_slot_weekdays_slot_id_fkey
        FOREIGN KEY (slot_id) REFERENCES template_slots (id) ON DELETE CASCADE
);

-- Drop weekday + GIST from template_slots, add a template/start-time lookup index.
ALTER TABLE template_slots DROP CONSTRAINT template_slots_template_weekday_start_end_key;
ALTER TABLE template_slots DROP CONSTRAINT template_slots_no_overlap_excl;
DROP INDEX  IF EXISTS template_slots_template_weekday_idx;
ALTER TABLE template_slots DROP COLUMN weekday;
CREATE INDEX template_slots_template_start_idx
    ON template_slots (template_id, start_time);

-- Add weekday column + composite FK to availability_submissions and assignments.
ALTER TABLE availability_submissions
    ADD COLUMN weekday INTEGER NOT NULL CHECK (weekday BETWEEN 1 AND 7);
ALTER TABLE availability_submissions
    DROP CONSTRAINT availability_submissions_publication_user_slot_key;
ALTER TABLE availability_submissions
    ADD CONSTRAINT availability_submissions_publication_user_slot_weekday_key
    UNIQUE (publication_id, user_id, slot_id, weekday);
ALTER TABLE availability_submissions
    ADD CONSTRAINT availability_submissions_slot_weekday_fkey
    FOREIGN KEY (slot_id, weekday)
    REFERENCES template_slot_weekdays (slot_id, weekday)
    ON DELETE CASCADE;
DROP INDEX IF EXISTS availability_submissions_publication_slot_idx;
CREATE INDEX availability_submissions_publication_slot_weekday_idx
    ON availability_submissions (publication_id, slot_id, weekday);

ALTER TABLE assignments
    ADD COLUMN weekday INTEGER NOT NULL CHECK (weekday BETWEEN 1 AND 7);
ALTER TABLE assignments
    DROP CONSTRAINT assignments_publication_user_slot_key;
ALTER TABLE assignments
    ADD CONSTRAINT assignments_publication_user_slot_weekday_key
    UNIQUE (publication_id, user_id, slot_id, weekday);
ALTER TABLE assignments
    ADD CONSTRAINT assignments_slot_weekday_fkey
    FOREIGN KEY (slot_id, weekday)
    REFERENCES template_slot_weekdays (slot_id, weekday)
    ON DELETE CASCADE;

-- Install the trigger.
CREATE FUNCTION template_slot_weekday_no_overlap() RETURNS trigger ... ; -- per D-2
CREATE TRIGGER template_slot_weekdays_no_overlap_trg
    BEFORE INSERT OR UPDATE ON template_slot_weekdays
    FOR EACH ROW EXECUTE FUNCTION template_slot_weekday_no_overlap();

-- Friendly hint to the operator.
DO $$ BEGIN
    RAISE NOTICE 'decouple-weekday-from-slot: tables truncated; run "make seed SCENARIO=…" to repopulate.';
END $$;

-- +goose Down
-- Restore the old shape. (Down is provided for completeness; it also TRUNCATEs.)
DROP TRIGGER IF EXISTS template_slot_weekdays_no_overlap_trg ON template_slot_weekdays;
DROP FUNCTION IF EXISTS template_slot_weekday_no_overlap();

ALTER TABLE assignments DROP CONSTRAINT assignments_slot_weekday_fkey;
ALTER TABLE assignments DROP CONSTRAINT assignments_publication_user_slot_weekday_key;
ALTER TABLE assignments DROP COLUMN weekday;
ALTER TABLE assignments
    ADD CONSTRAINT assignments_publication_user_slot_key
    UNIQUE (publication_id, user_id, slot_id);

ALTER TABLE availability_submissions DROP CONSTRAINT availability_submissions_slot_weekday_fkey;
ALTER TABLE availability_submissions DROP CONSTRAINT availability_submissions_publication_user_slot_weekday_key;
ALTER TABLE availability_submissions DROP COLUMN weekday;
ALTER TABLE availability_submissions
    ADD CONSTRAINT availability_submissions_publication_user_slot_key
    UNIQUE (publication_id, user_id, slot_id);
DROP INDEX IF EXISTS availability_submissions_publication_slot_weekday_idx;
CREATE INDEX availability_submissions_publication_slot_idx
    ON availability_submissions (publication_id, slot_id);

DROP INDEX IF EXISTS template_slots_template_start_idx;
ALTER TABLE template_slots
    ADD COLUMN weekday INTEGER NOT NULL DEFAULT 1
    CHECK (weekday BETWEEN 1 AND 7);
ALTER TABLE template_slots ALTER COLUMN weekday DROP DEFAULT;
ALTER TABLE template_slots
    ADD CONSTRAINT template_slots_template_weekday_start_end_key
    UNIQUE (template_id, weekday, start_time, end_time);
ALTER TABLE template_slots
    ADD CONSTRAINT template_slots_no_overlap_excl
    EXCLUDE USING gist (
        template_id WITH =,
        weekday WITH =,
        tsrange(
            ('2000-01-01'::date + start_time)::timestamp,
            ('2000-01-01'::date + end_time)::timestamp,
            '[)'
        ) WITH &&
    );
CREATE INDEX template_slots_template_weekday_idx
    ON template_slots (template_id, weekday, start_time);

DROP TABLE template_slot_weekdays;

TRUNCATE
    leaves,
    shift_change_requests,
    assignment_overrides,
    assignments,
    availability_submissions,
    template_slot_positions,
    template_slots
RESTART IDENTITY CASCADE;
```

The Down isn't a true reversal of *content* (data is gone in either direction), but it is a true reversal of *schema*, which is what `goose down` exists to provide.

### D-4. API surface

**Template slot endpoints** (`POST /templates/{id}/slots`, `PATCH /templates/{id}/slots/{slot_id}`):

Old body: `{ weekday: int, start_time: "HH:MM", end_time: "HH:MM" }`.
New body: `{ weekdays: int[], start_time: "HH:MM", end_time: "HH:MM" }`. The `weekdays` array MUST be non-empty, contain only integers in `[1,7]`, and be deduplicated server-side. Empty array → HTTP 400 / `INVALID_REQUEST`.

`PATCH` semantics: `weekdays`, if present, replaces the slot's weekday set entirely (atomic). Removing a weekday cascades and drops referencing submissions/assignments via the FK — admin gets a warning toast on the frontend ("dropping N existing assignments tied to weekday W"); the backend doesn't pre-flight, the DB does the cascade.

`GET /templates/{id}` slot rows: each slot now carries `weekdays: int[]` (sorted ascending) instead of `weekday: int`. Sort order across slots becomes `(start_time, end_time, id)` — weekday is no longer a sort key.

**Availability endpoints** (`POST /publications/{id}/submissions`, `DELETE /publications/{id}/submissions/{slot_id}/{weekday}`):

`POST` body: `{ slot_id: int, weekday: int }`. Old body `{ slot_id }` — rejected with HTTP 400 / `INVALID_REQUEST`.

`DELETE` URL pattern: `/publications/{id}/submissions/{slot_id}/{weekday}` (was `/{slot_id}`). Adds a path segment; cleaner than smuggling weekday in the body for a DELETE.

`GET /publications/{id}/shifts/me`: the response shape stays — each row already carries `weekday`, `slot_id`, `start_time`, `end_time`, `composition` — but the *meaning* of a row changes from "slot S (which has weekday W baked in)" to "(slot S, weekday W)". For a slot whose weekday set has 5 entries, `shifts/me` returns 5 rows — one per applicable weekday — instead of 5 distinct slot rows that happen to share a time. Numerically identical.

`GET /publications/{id}/submissions/me`: returns `[ { slot_id, weekday } ]` instead of `[ slot_id ]`.

**Assignment endpoints** (`POST /publications/{id}/assignments`, board reads):

`POST` body: `{ user_id, slot_id, weekday, position_id }`. Old `{ user_id, slot_id, position_id }` — rejected with HTTP 400 / `INVALID_REQUEST`.

`GET /publications/{id}/assignment-board`: the cell key on the response becomes `{ slot_id, weekday, position_id }`. Each cell still carries `assignments[]`, `candidates[]`, `non_candidate_qualified[]`. No semantics change beyond the key widening.

**Shift-change endpoints**: bodies that today reference `{ slot_id, occurrence_date }` continue to do so unchanged. The `(slot_id, occurrence_date)` validation now reads `weekday` from the *assignment* being modified rather than from the slot row, but that's an internal mechanical change, not an API change.

**Roster (`GET /publications/{id}/roster?week=...`)**: response payload stays — each cell already carries `weekday` and the rest of the schema. The sort key inside the response moves from `(slot.weekday, slot.start_time)` to `(weekday, start_time)`.

**Error codes**: no new codes; `TEMPLATE_SLOT_OVERLAP`, `INVALID_REQUEST`, `INVALID_OCCURRENCE_DATE`, `NOT_QUALIFIED`, `PUBLICATION_NOT_COLLECTING`, `PUBLICATION_NOT_ASSIGNING`, `ASSIGNMENT_USER_ALREADY_IN_SLOT`, `USER_DISABLED`, `PUBLICATION_NOT_ACTIVE`, `INVALID_PUBLICATION_WINDOW`, `SHIFT_CHANGE_INVALID_TYPE` all continue to apply with their existing HTTP statuses.

### D-5. Auto-assigner refactor

The MCMF graph stays the same shape:

- `s → seat_i(user) → employee(user) → (user, slot, weekday) → (slot, weekday, position) → t`

The only change is that the `(slot, position)` cell becomes `(slot, weekday, position)` and the per-weekday overlap groups (today computed as "of slots a user submitted for, group by `slot.weekday` and find time-overlap clusters") become "group by `submission.weekday`". Same algorithm, different field source.

Concretely in [autoassign.go:40](backend/internal/service/autoassign.go#L40), the `weekday` field stays on the `slotPosition`-equivalent struct; it just gets sourced from the submission's `weekday` column instead of the slot's. The candidate-pool query joins `availability_submissions` to `template_slot_weekdays` (via the FK) and to `template_slot_positions`, returning `(submission.id, slot_id, weekday, position_id, required_headcount)`.

### D-6. Occurrence validation

The "occurrence weekday must match slot weekday" predicate lives in [model/publication.go:131](backend/internal/model/publication.go#L131). It compares `weekdayToSlotValue(date.Weekday())` against `slot.Weekday`.

Post-change, the comparand is the *assignment's* weekday (or, for shift-change creation, the *slot's weekday set membership*: `weekday ∈ slot.weekdays`). The `IsValidOccurrence(publication, slot, occurrence_date)` predicate updates to `weekdayToSlotValue(occurrence_date.Weekday()) ∈ slot_weekdays(slot)` — i.e., the date's weekday must be in the slot's applicable set.

For assignment-tied occurrence checks (shift-change against an existing assignment), the comparand is `assignment.weekday` directly. The signature evolves from `IsValidOccurrence(publication, slot, occurrence_date)` to `IsValidOccurrenceForAssignment(publication, assignment, occurrence_date)` where the latter checks `weekdayToSlotValue(occurrence_date.Weekday()) == assignment.Weekday`. Both helpers coexist.

### D-7. Frontend template page

Single page rewrite at `frontend/src/routes/templates.$id.tsx` (or the equivalent file pinpointed during apply).

**New layout:**

```
┌─ Realistic Rota  [已锁定]                    [克隆] [删除] ┐
│  description …                                              │
├─ 时段                                       [新增时段]      │
│  ┌─ 09:00–10:00          weekdays: [✓✓✓✓✓✗✗]              │
│  │   composition:                                           │
│  │     前台负责人 × 1                                       │
│  │     前台助理 × 2                                         │
│  │   [编辑] [删除]                                          │
│  └─ 10:00–12:00          weekdays: [✓✓✓✓✓✗✗]              │
│      …                                                      │
└────────────────────────────────────────────────────────────┘
```

- Slot rows are flat, sorted by `start_time`. No weekday accordion.
- Weekday set is rendered as 7 chips Mon-Sun; toggling persists via PATCH.
- Composition is rendered once per slot.
- Locked-template path: hide all per-slot edit/delete buttons (this also fixes the locked-UI redundancy that started this conversation).

The availability page (`/availability/...`) keeps its current weekday × time grid; each cell now corresponds to a `(slot_id, weekday)` pair instead of a slot id. The wire payload changes; the visual stays. The assignment board, shift-change UI, and roster do the same: the wire shape evolves, the visual stays.

### D-8. Spec rewrite mechanics

Most of `scheduling/spec.md` is textual surgery rather than logical surgery. Categories of edit:

| Edit | Where | Example |
|---|---|---|
| `slot.weekday` → `submission.weekday` / `assignment.weekday` | Availability + assignment requirements | "the slot's `weekday`" → "the submission's `weekday`" |
| Drop "weekday" from slot field list | *Template and shift data model* | `weekday`, `start_time` → `start_time` |
| Add `template_slot_weekdays` table prose | *Template and shift data model* | New paragraph |
| Replace GIST exclusion language with trigger | *Template and shift data model* | Language change, scenario-text identical |
| Widen unique key | *Availability submission*, *Assignment* data model | `UNIQUE(publication_id, user_id, slot_id)` → `…, weekday` |
| Update endpoint body shape | API requirements | `{ slot_id }` → `{ slot_id, weekday }` |
| Update sort order | *Template detail*, *Roster* | `(weekday, start_time)` → `(start_time, end_time)` for templates; roster keeps `(weekday, start_time)` |
| Shift `IsValidOccurrence` signature | *Occurrence concept* | Replace text per D-6 |
| Update row-count assertions | `dev-tooling` realistic + full scenarios | `35 template_slots` → `5 template_slots`; etc. |

**`shifts/me` response shape**: currently each row carries `slot_id, weekday, start_time, end_time, composition`; this stays. Only the row count semantic shifts (one row per `(slot, weekday)` pair instead of one row per slot, but since every slot today only has one weekday, the count is numerically identical).

### D-9. Test fixture migration

Integration test fixtures under `backend/internal/service/*_integration_test.go` and the seed scenarios all build slots in the old shape (`(weekday, start_time, end_time)` directly inserted). Each fixture is rewritten to:

1. Insert one `template_slots` row per logical `(start_time, end_time)` group.
2. Insert N `template_slot_weekdays` rows for the slot's weekday set.
3. Insert `template_slot_positions` once per slot (instead of N times).

The MCMF integration test (`autoassign_integration_test.go`) is the largest mechanical churn — it has `insertTemplateSlot(template, weekday, start, end)` helpers that need to become `insertTemplateSlot(template, weekdays []int, start, end)`. Helper signature change ripples through ~50 call sites. Mechanical, no logic changes.

The seed scenarios:

- `realistic.go` is regenerated by the existing `generate_realistic_seed.py` — the script's Go-emitter is updated to produce the new shape, no archetype-assignment logic changes.
- `full.go` and `stress.go` are hand-written; their slot-definition lists collapse same-time-different-weekday entries into single slots with weekday sets. `fullSlotDefinitions()` shrinks from ~10 entries to whatever the deduped count is (probably 6-7).

### D-10. Audit and shift-change `expires_at`

`shift_change_requests.expires_at` derivation today: `publication.planned_active_from + (slot.weekday - 1) * INTERVAL '1 day' + slot.start_time`.

Post-change: `publication.planned_active_from + (assignment.weekday - 1) * INTERVAL '1 day' + slot.start_time`. Logic identical; field source moves.

Audit metadata (e.g., `assignment.create` events) currently includes `slot_id` and infers weekday via the slot row. New events include `weekday` directly to keep the audit log self-contained without back-joining to a now-pluralized slot.

## Risks / Trade-offs

- **Risk:** trigger-based overlap exclusion is slower than GIST. → Mitigation: templates are edited rarely; the trigger does an indexed `(template_id, weekday)` lookup per row. Profile if it ever bites; switching to a covering index on `template_slot_weekdays(slot_id, weekday)` and `template_slots(template_id, start_time, end_time)` is straightforward.
- **Risk:** auto-assigner unit tests + integration tests are large and fixture-heavy; mechanical churn risk. → Mitigation: keep the fixture rewrites in a single commit per test file so review is per-file diff-only; do not bundle logic changes with fixture moves.
- **Risk:** the composite FK from `availability_submissions(slot_id, weekday)` → `template_slot_weekdays(slot_id, weekday)` requires the latter's PK ordering to match. The PK is `(slot_id, weekday)`; FKs reference it in that order. → No mitigation needed; documenting so the migration author doesn't write `(weekday, slot_id)`.
- **Risk:** dev forgets to re-seed after `make migrate-up` and hits empty tables. → Mitigation: migration's `RAISE NOTICE` hint; apply task runs `make seed SCENARIO=basic` immediately after `make migrate-up` to verify the round-trip.
- **Trade-off:** `PATCH /templates/{id}/slots/{slot_id}` with a shrunk `weekdays` array silently cascades-deletes referencing submissions and assignments. The frontend should warn the admin before submitting. → Acceptable: locked templates can't be patched (the locking invariant covers any active publication's slots), and unlocked templates have no published assignments yet.

## Migration Plan

Single shipping unit:

1. Apply `migrations/00016_decouple_weekday_from_slot.sql` in `make migrate-up`.
2. Re-seed dev databases with `make seed SCENARIO=…`.
3. Backend rebuilt and tested (`go build`, `go vet`, `go test ./...`, integration tests).
4. Frontend rebuilt (`pnpm build`, `pnpm test`, `pnpm lint`).

Rollback = revert the change → `make migrate-down 1` (the Down restores the old schema and TRUNCATEs again) → re-seed.

## Open Questions

None.
