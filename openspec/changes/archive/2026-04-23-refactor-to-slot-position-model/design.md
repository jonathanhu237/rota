## Context

The current `template_shifts` row-shape is `(id, template_id, weekday, start_time, end_time, position_id, required_headcount)`. It conflates two concepts that are structurally independent:

1. A **time slot** within a weekly template: one contiguous interval `(weekday, start_time, end_time)` that represents "a work slot on this day."
2. A **position requirement** within that slot: "this slot needs N people in position P."

Because both live on the same row, expressing a realistic schedule — one where each day contains multiple distinct time slots, and each slot has a fixed team composition with several positions at different headcounts — requires N×M rows (one per (slot-grouping, position) pair) with no concept tying the slot-grouping together. Concretely, our own current production schedule expands to ~74 `template_shifts` rows sharing only implicit `(weekday, start_time, end_time)` tuples.

Three consequences are the load-bearing problems:

- **Bugs that the schema cannot prevent**: same user holding two positions in the same time slot; same user in two slots whose times overlap; template rows defining two slots that overlap. All are either the admin's fault today (no check exists on the `CreateAssignment` path) or expressible by construction in the data.
- **Every "slot-level" operation is synthesized**: copy a day, move a slot, summarize a slot's team, enforce "slot fully staffed." Each is a group-by over `(weekday, start_time, end_time)` on the application side.
- **The spec has already drifted** from the code. `Assignment window` still asserts the `ASSIGNING`-only write window for `POST/DELETE /assignments`, but `admin-shift-adjustments` widened this to `{ASSIGNING, PUBLISHED, ACTIVE}`. The old requirement was not updated (we added the new one only). This refactor has to rewrite that requirement anyway — same pass fixes the drift.

We have no production data, no external users, and no contracts that lock the old shape. This window closes as soon as either exists.

## Goals / Non-Goals

**Goals:**

- Replace `template_shifts` with a clean two-level structure: `template_slots` (time container) + `template_slot_positions` (per-slot composition).
- Make assignments reference `(slot_id, position_id)` so one user per slot is the natural key.
- Push two classes of time-overlap integrity into database constraints, not code: slots of the same `(template, weekday)` cannot overlap; one user cannot hold two positions in the same slot.
- Add the remaining time-overlap check — "same user, two different slots that overlap" — at the service layer on `CreateAssignment`, using the same predicate that `ApplySwap` / `ApplyGive` already use.
- Rewrite the `scheduling` and `audit` spec sections whose wording is shaped around `template_shift`; use the same pass to correct the already-drifted `Assignment window` requirement.
- Do the whole migration big-bang: one goose migration, one PR, one cutover.

**Non-Goals:**

- Non-weekly recurrence. `template_slots.weekday` is still 1..7, same as today.
- Date-specific overrides to a template (e.g., "this week only, move Mon 09:00 slot to 10:00").
- Slot library / cross-template slot reuse.
- Blocking publications that have unstaffed slot positions — understaffing remains allowed.
- Admin bulk-operation UI (copy day, duplicate slot). The new model makes these structurally possible; the UI is a separate change.
- Dual-writing / phased migration. Big-bang only.

## Decisions

### Two-level schema: `template_slots` + `template_slot_positions`

```
   template                   template_slot                  template_slot_position
   ────────────               ─────────────────────          ──────────────────────
   id                         id                             id
   name                       template_id  ────────────┐     slot_id ─────────────┐
   is_locked                  weekday (1..7)            │    position_id          │
   description                start_time                │    required_headcount   │
                              end_time                  │                         │
                              UNIQUE(template,weekday,  │    UNIQUE(slot_id,      │
                                    start_time,end_time)│          position_id)   │
                              CHECK end > start         │                         │
                              EXCLUDE USING gist (      │                         │
                                template_id WITH =,     │                         │
                                weekday WITH =,         │                         │
                                tsrange(start, end)     │                         │
                                  WITH &&               │                         │
                              )                         │                         │
                                                        │                         │
                                                        └──────1 : N──────────────┘
```

`template_slots` owns the time window; `template_slot_positions` owns "this slot needs 1 of position P, 2 of position Q" as independent rows. Slot deletion cascades to its slot-positions.

**Rejected**: keep slot and slot_position in one wide row with JSON `positions`. Rejected because PostgreSQL-side referential integrity (position FK `ON DELETE RESTRICT`) disappears, and every slot-positions query becomes a JSON scan.

### GIST exclusion constraint for slot overlap

Postgres has `btree_gist`; we use it to forbid within-`(template, weekday)` time overlap at the database level. The constraint predicate uses `tsrange(start_time::timestamp, end_time::timestamp, '[)')` — half-open ranges, matching the inclusive-exclusive convention used by `ApplySwap` / `ApplyGive` (`a.start < b.end && b.start < a.end`).

**Goose migration** will `CREATE EXTENSION IF NOT EXISTS btree_gist;` as part of the same file that creates `template_slots`. We already run as a Postgres superuser in dev and in the production container.

**Rejected**: enforce non-overlap only in service code. Rejected because slots are low-frequency writes (admin only, template definition time) but the integrity matters across admins, across clients, and across time — DB-level is correct here. The cost is one extension, one constraint; there is no realistic traffic concern.

### Assignments become `(publication_id, user_id, slot_id, position_id)` with natural key `UNIQUE(publication_id, user_id, slot_id)`

```
   assignments
   ─────────────────────────────────────────
   id                       BIGSERIAL
   publication_id           FK → publications       ON DELETE CASCADE
   user_id                  FK → users              ON DELETE CASCADE
   slot_id                  FK → template_slots     ON DELETE CASCADE
   position_id              FK → positions          ON DELETE RESTRICT
   created_at               TIMESTAMP
   UNIQUE(publication_id, user_id, slot_id)                -- one user per slot
   UNIQUE(publication_id, slot_id, position_id, user_id)   -- dedup (redundant but cheap)
   CHECK (position_id IN (                                  -- position must belong to slot
     SELECT position_id FROM template_slot_positions WHERE slot_id = assignments.slot_id
   ))   -- enforced via trigger; inline CHECK cannot subquery
```

**Why the slot-level uniqueness**: closes the "same user, two positions, same slot" bug without any service code. Enforced at write time, not checked at read time.

**Why the position-belongs-to-slot trigger**: without it, `assignments.position_id` can reference a position that is not part of the slot's composition. That is a semantically invalid row. Postgres does not allow subqueries in `CHECK`, so we use a row-level trigger fired on `INSERT OR UPDATE`.

**Rejected**: drop `position_id` from `assignments` and derive it from the slot-position link the user was matched against. Rejected because auto-assign's MCMF has to output `(user, slot, position)` tuples — the position is part of the decision, not a derivation from an abstract assignment.

### Overlap check on admin `CreateAssignment`

The DB covers same-slot overlap and within-template slot overlap. What remains: "the same user is assigned to two different, time-overlapping slots." This is a service-layer check because the two slots can be in different templates (via separate publications) — but in practice all assignments within one publication share one template, so the check is per-publication.

**Predicate** (reuses the exact shape `ApplySwap` uses):

```
  An insert of assignment(user=U, slot=S_new) is rejected iff there exists
  another assignment A in the same publication for user U, where
    slot(A).weekday == slot(S_new).weekday
    AND slot(A).start_time < slot(S_new).end_time
    AND slot(S_new).start_time < slot(A).end_time
```

Error: `ErrAssignmentTimeConflict` → HTTP 409 `ASSIGNMENT_TIME_CONFLICT`. Distinct from `SHIFT_CHANGE_TIME_CONFLICT` (used on the shift-change apply path) so clients can distinguish "I tried to create a conflicting assignment" from "I tried to apply a swap that would produce a conflict." Same human meaning, different origin.

**Rejected**: inline the check in the repository SQL via a subquery with `ON CONFLICT`. Rejected because the error needs to surface a service-layer sentinel with meaningful metadata (which assignment conflicts?), not a generic constraint violation.

### Auto-assign (MCMF) graph retargeting

Current MCMF: `source → candidates (availability rows) → template_shifts → sink`. Capacities come from `template_shifts.required_headcount`.

New MCMF: `source → candidates → slot-position nodes (one per `template_slot_positions` row) → sink`. Capacities come from `template_slot_positions.required_headcount`. Edge from candidate to slot-position iff the user is qualified for that position AND submitted availability for that slot. The same-user/same-slot constraint becomes "a user has capacity 1 to each distinct slot" — encoded as an intermediate `(user, slot)` node with capacity 1 between the user and any slot-position of that slot.

**Rejected**: keep the graph keyed on template_shifts. Rejected because the new "one-user-per-slot" invariant has no home in the old graph — it's a natural LP constraint that falls out of the intermediate `(user, slot)` node.

### Handler response shape: group by slot, list positions inside

Today:

```
assignment_board.shifts[i] = {
  shift: { id, weekday, start_time, end_time, position_id, position_name, required_headcount },
  candidates: [...],
  non_candidate_qualified: [...],
  assignments: [...]
}
```

New:

```
assignment_board.slots[i] = {
  slot: { id, weekday, start_time, end_time },
  positions: [
    {
      position: { id, name, required_headcount },
      candidates: [...],
      non_candidate_qualified: [...],
      assignments: [...]
    },
    ...
  ]
}
```

Every client that renders the board walks `slots[].positions[]` instead of a flat `shifts[]`. The same shape propagates to the roster response and to shift-change request JSON (where references use `slot_id` + `position_id` instead of `template_shift_id`).

**Rejected**: keep the flat `shifts[]` shape and add a redundant `slot_id` field for grouping. Rejected because the whole point of the refactor is to make slot a first-class concept; a flat list with a grouping key is the worst of both worlds.

### Audit metadata

Assignment-related audit events (`assignment.create`, `assignment.delete`, `shift_change.*`) currently emit `template_shift_id` in metadata. They change to emit both `slot_id` and `position_id`. `assignment_id` is unchanged.

Archived admin-shift-adjustments audit action `shift_change.invalidate.cascade` and its metadata fields `request_id`, `reason`, `assignment_id` are unchanged.

### The `Assignment window` drift fix

Today the `scheduling` spec has two requirements pointing in opposite directions:

- Old `Assignment window`: "Creating or deleting an assignment and running auto-assign SHALL require the publication's effective state to be `ASSIGNING`."
- New `Admin may edit assignments during PUBLISHED and ACTIVE` (added by the previous change): widens create/delete to `{ASSIGNING, PUBLISHED, ACTIVE}`.

The code matches the new one. The old one is stale. This refactor modifies `Assignment window` to:

- Remove the "create / delete require ASSIGNING" clause.
- Keep the "auto-assign requires ASSIGNING" clause (which remains true).
- Update the error-code scenario to reference `ASSIGNMENT_TIME_CONFLICT` on the new overlap-rejection path.

### Goose migration: one file, Up and Down both runnable

Up:

```sql
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE template_slots (
  id BIGSERIAL PRIMARY KEY,
  template_id BIGINT NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
  weekday SMALLINT NOT NULL CHECK (weekday BETWEEN 1 AND 7),
  start_time TIME NOT NULL,
  end_time TIME NOT NULL CHECK (end_time > start_time),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (template_id, weekday, start_time, end_time),
  EXCLUDE USING gist (
    template_id WITH =,
    weekday WITH =,
    tsrange(('2000-01-01'::date + start_time)::timestamp,
            ('2000-01-01'::date + end_time)::timestamp, '[)') WITH &&
  )
);

CREATE INDEX template_slots_template_weekday_idx
  ON template_slots (template_id, weekday, start_time);

CREATE TABLE template_slot_positions (
  id BIGSERIAL PRIMARY KEY,
  slot_id BIGINT NOT NULL REFERENCES template_slots(id) ON DELETE CASCADE,
  position_id BIGINT NOT NULL REFERENCES positions(id) ON DELETE RESTRICT,
  required_headcount INTEGER NOT NULL CHECK (required_headcount > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (slot_id, position_id)
);

-- New assignments shape
DROP TABLE assignments;
CREATE TABLE assignments (
  id BIGSERIAL PRIMARY KEY,
  publication_id BIGINT NOT NULL REFERENCES publications(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  slot_id BIGINT NOT NULL REFERENCES template_slots(id) ON DELETE CASCADE,
  position_id BIGINT NOT NULL REFERENCES positions(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (publication_id, user_id, slot_id)
);

CREATE OR REPLACE FUNCTION assignments_position_belongs_to_slot()
RETURNS TRIGGER AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM template_slot_positions
    WHERE slot_id = NEW.slot_id AND position_id = NEW.position_id
  ) THEN
    RAISE EXCEPTION 'position % is not part of slot %', NEW.position_id, NEW.slot_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER assignments_position_belongs_to_slot_trigger
  BEFORE INSERT OR UPDATE ON assignments
  FOR EACH ROW EXECUTE FUNCTION assignments_position_belongs_to_slot();

-- Other tables that reference template_shift_id (availability, shift_change_requests)
-- Availability: replace template_shift_id with (slot_id, position_id).
ALTER TABLE availability_submissions DROP COLUMN template_shift_id;
ALTER TABLE availability_submissions ADD COLUMN slot_id BIGINT NOT NULL REFERENCES template_slots(id) ON DELETE CASCADE;
ALTER TABLE availability_submissions ADD COLUMN position_id BIGINT NOT NULL REFERENCES positions(id) ON DELETE RESTRICT;
ALTER TABLE availability_submissions ADD UNIQUE (publication_id, user_id, slot_id, position_id);

-- Shift-change requests reference assignment_id (unchanged); no template_shift_id on them today, so nothing to change there.

DROP TABLE template_shifts;
```

Down: reverse order. The `assignments` table will lose all rows on rollback (acceptable; no production data). Down SQL is for the common "oops my local DB is stuck" case, not for production recovery.

### Error taxonomy additions

| Error code                        | HTTP status | Emitted by                                              |
| --------------------------------- | ----------- | ------------------------------------------------------- |
| `ASSIGNMENT_TIME_CONFLICT`        | 409         | `CreateAssignment` when the new assignment's slot overlaps an existing same-weekday assignment of the same user (service-layer check, defense-in-depth against cross-template cases) |
| `ASSIGNMENT_USER_ALREADY_IN_SLOT` | 409         | `CreateAssignment` when `(publication_id, user_id, slot_id)` already has an assignment row — the DB-level `UNIQUE` constraint fires and the repository translates the `pq` unique-violation into a sentinel. |
| `TEMPLATE_SLOT_OVERLAP`           | 409         | `CreateSlot` / `UpdateSlot` when two slots of the same `(template_id, weekday)` would overlap — DB-level GIST exclusion fires and the repository translates the `pq` exclusion-violation into a sentinel. |
| `SHIFT_CHANGE_TIME_CONFLICT`      | 409         | `ApplySwap` / `ApplyGive` (unchanged)                   |
| `PUBLICATION_NOT_MUTABLE`         | 409         | (unchanged)                                             |
| `PUBLICATION_NOT_ASSIGNING`       | 409         | `AutoAssignPublication` (unchanged)                     |

**DB-constraint mapping rule**: every DB-level integrity constraint introduced by this refactor (the `UNIQUE(publication_id, user_id, slot_id)` index, the `template_slots` GIST exclusion, the `assignments_position_belongs_to_slot` trigger) SHALL be mapped to a meaningful domain sentinel + HTTP 409 code at the repository layer. The repository detects `*pq.Error` and switches on `Code`:

- `23505` (unique_violation) on `assignments_publication_user_slot_key` → `ErrAssignmentUserAlreadyInSlot`
- `23P01` (exclusion_violation) on `template_slots` → `ErrTemplateSlotOverlap`
- `P0001` (raise_exception, from the trigger) with text `position % is not part of slot %` → `ErrTemplateSlotPositionNotFound`

Letting a `pq.Error` surface as `INTERNAL_ERROR` violates the contract that every user-visible integrity rule has a dedicated error code. The spec's scenarios for these constraints include an **"AND the handler returns HTTP 409 with error code X"** clause specifically to make this contract testable end-to-end. (The first smoke test of this change revealed the gap; Section 15 of `tasks.md` fixes it.)

## Risks / Trade-offs

- **Risk**: big-bang migration means there is no in-between state on disk. Running the goose `Up` on a DB that had real assignments would lose them.
  **Mitigation**: we have none. Migration test run against a seeded integration DB confirms the goose script works end-to-end. Devs warned in the tasks.md preamble to nuke their local DB.
- **Risk**: MCMF rewrite has subtle arithmetic — it is easy to introduce a flow-graph that silently produces invalid or suboptimal assignments.
  **Mitigation**: keep the auto-assign test suite's existing golden-case fixtures; verify output shape (user, slot, position); add one new fixture with the "same user cannot appear in two overlapping slots of the same weekday" invariant and assert MCMF respects it.
- **Risk**: handler response reshape breaks every frontend page that renders assignments. A frontend that compiles against the old type cannot cope with the new one — we ship a combined backend+frontend PR or we break the app for whoever has the new backend and old frontend.
  **Mitigation**: single PR, single cutover. No API versioning — acceptable because no external consumers.
- **Trade-off**: the `assignments_position_belongs_to_slot` trigger introduces a non-trivial bit of plpgsql that has to be maintained and reviewed. Alternative was a lookup-table FK, which doesn't express the same invariant. Trigger accepted as the cost of the invariant being correct at the database level.
- **Trade-off**: `UNIQUE(publication_id, user_id, slot_id)` is strict: a user literally cannot hold two positions in the same slot, even if the business some day wants "tech lead ALSO on standby." We are committing to the stricter rule; if the business reverses, we drop the constraint later, but we do not pre-pay optionality.

## Migration Plan

Single-step cutover, no rollback beyond `goose down` on a developer's local DB:

1. **Branch**: one PR containing migration + backend + frontend + spec delta.
2. **Local verification**: `make migrate-down && make migrate-up` cleanly applies; integration tests pass; `pnpm build` clean.
3. **Deploy order**: goose up on production DB → restart backend → deploy frontend. Because there is no production data, the goose up is effectively instantaneous.
4. **Rollback**: `goose down` then redeploy the previous container. Acceptable only because we have no data; document this clearly in the PR description.

## Open Questions

None at this time.
