## Context

The `availability_submissions` table today is keyed by `(publication_id, user_id, slot_id, position_id)`. This was the right shape when the model assumed an employee might want to declare role preferences ("I'm available Mon 09-12 but only as 助理, not 负责人"). In practice no one uses it that way — employees mark "free" / "not free" per time block, period. The product owner identified this during the realistic-seed prep: senior employees end up with two checkboxes per slot, neither of which maps to a real choice they'd make.

This change strips position out of the availability layer. Submissions become per-slot. Auto-assign keeps using `user_positions` to pick the role at assignment time. The `assignments` table is unchanged — the actual job allocation still carries `(slot_id, position_id)`.

## Goals / Non-Goals

**Goals:**

- One availability submission row per `(publication_id, user_id, slot_id)`.
- Frontend grid: one checkbox per slot (with a composition caption for context).
- Auto-assign rederives candidacy from `user_positions ∩ slot.composition`.
- Five `scheduling` spec requirements reword cleanly without semantic regressions.

**Non-Goals:**

- Adding a per-role preference signal anywhere.
- Touching `assignments`, `assignment_overrides`, shift-change, or leave models.
- Live data migration (no production rows).
- Restructuring how the assignment board surfaces candidates (the assignment-board endpoint exposes `(slot, position)` cells with their own candidate lists; those lists are now derived per slot, but the cell-shaped output stays).

## Decisions

### D-1. Schema

```sql
-- Before:
CREATE TABLE availability_submissions (
    id             BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL,
    user_id        BIGINT NOT NULL,
    slot_id        BIGINT NOT NULL,
    position_id    BIGINT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
    -- composite FK (slot_id, position_id) → template_slot_positions
    -- UNIQUE (publication_id, user_id, slot_id, position_id)
);

-- After:
CREATE TABLE availability_submissions (
    id             BIGSERIAL PRIMARY KEY,
    publication_id BIGINT NOT NULL REFERENCES publications(id) ON DELETE CASCADE,
    user_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    slot_id        BIGINT NOT NULL REFERENCES template_slots(id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (publication_id, user_id, slot_id)
);

CREATE INDEX availability_submissions_publication_slot_idx
    ON availability_submissions (publication_id, slot_id);
```

The composite FK to `template_slot_positions(slot_id, position_id)` is gone — no per-position membership is checked at the submission layer anymore. The slot-level FK is enough: a submission for a non-existent slot is rejected.

### D-2. Auto-assign MCMF candidate pool

The current spec derives candidates from submissions that already pin `(user, slot, position)`. After this change:

```
For each slot S:
  qualifying users U(S) = { user u : u submitted availability for S 
                            AND user_positions(u) ∩ composition(S) ≠ ∅ }

For each (slot, position) cell (S, P):
  candidates(S, P) = { user u ∈ U(S) : P ∈ user_positions(u) }
```

Graph construction:

- Source `s`.
- One per-user "employee" node `e_u`.
- For each user `u` with at least one candidacy, weighted seat edges from `s` to `e_u` (preserving the existing spreading mechanism).
- Per-weekday overlap groups still apply: a user takes at most one slot per overlap group, regardless of position.
- For each slot `S` with `u ∈ U(S)`, a `(u, S)` intermediate node of capacity 1.
- Edges from `(u, S)` to each `(S, P)` cell where `P ∈ user_positions(u)`. Cost: same as today (no per-cell cost change).
- `(S, P)` cells connect to sink `t` with capacity = `required_headcount` and the existing coverage bonus.

The graph shape simplifies from "submissions are pre-resolved to cells" to "submissions are at the slot level; cells are dynamically chosen via the user's qualifications". The flow solver does the cell selection. Concrete change: where the old graph created a candidate edge for the exact `(slot, position)` the user submitted, the new graph fan-outs from `(u, S)` to every cell of `S` whose position is in `user_positions(u)`.

The unique key `UNIQUE(publication_id, user_id, slot_id)` on `assignments` already enforces "one user, one position per slot, per publication" — that constraint is unchanged and the graph respects it via the `(u, S)` cap-1 node.

**Alternative considered — keep submissions per-`(slot, position)` but make the UI emit them in bulk per slot.** Rejected: it doesn't fix the model, just hides it. The DB and the auto-assigner still see per-`(slot, position)` rows, and any future feature reading submissions will be misled by the redundancy.

### D-3. Migration

```sql
-- +goose Up
-- +goose StatementBegin

-- Drop the composite FK referencing template_slot_positions.
ALTER TABLE availability_submissions
    DROP CONSTRAINT IF EXISTS availability_submissions_slot_position_fkey;

-- Drop the existing unique constraint / index that includes position_id.
DROP INDEX IF EXISTS availability_submissions_publication_user_slot_position_uidx;

-- Drop the column itself.
ALTER TABLE availability_submissions DROP COLUMN IF EXISTS position_id;

-- Recreate the unique constraint and a slot-grouped read index.
ALTER TABLE availability_submissions
    ADD CONSTRAINT availability_submissions_publication_user_slot_uidx
        UNIQUE (publication_id, user_id, slot_id);

CREATE INDEX IF NOT EXISTS availability_submissions_publication_slot_idx
    ON availability_submissions (publication_id, slot_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- The Down direction is best-effort: position information cannot be
-- reconstructed once dropped. Recreate the column with NULLs and the
-- old unique-key shape so the schema round-trips structurally; existing
-- post-migration rows will all carry NULL position_id, which is fine
-- because there are no production rows on this branch.
DROP INDEX IF EXISTS availability_submissions_publication_slot_idx;
ALTER TABLE availability_submissions
    DROP CONSTRAINT IF EXISTS availability_submissions_publication_user_slot_uidx;

ALTER TABLE availability_submissions
    ADD COLUMN position_id BIGINT;

ALTER TABLE availability_submissions
    ADD CONSTRAINT availability_submissions_slot_position_fkey
        FOREIGN KEY (slot_id, position_id)
        REFERENCES template_slot_positions (slot_id, position_id)
        ON DELETE CASCADE
        DEFERRABLE INITIALLY DEFERRED;

CREATE UNIQUE INDEX availability_submissions_publication_user_slot_position_uidx
    ON availability_submissions (publication_id, user_id, slot_id, position_id);

-- +goose StatementEnd
```

The Down direction's NULL-fill is acceptable per the project's "no production data" stance; CI's migrations-roundtrip exercises Up→Down→Up structurally.

### D-4. API surface

**Request body** for `POST /publications/{id}/submissions`:

```json
// Before
{ "slot_id": 12, "position_id": 5 }

// After
{ "slot_id": 12 }
```

**DELETE URL**:

```
// Before
DELETE /publications/{id}/submissions/{slot_id}/{position_id}

// After
DELETE /publications/{id}/submissions/{slot_id}
```

**`GET /publications/{id}/shifts/me`** response shape:

```jsonc
// Before: one entry per (slot, position) the viewer can fill
[
  { "slot_id": 12, "position_id": 5, "weekday": 1, "start_time": "09:00", "end_time": "10:00", "required_headcount": 1 },
  { "slot_id": 12, "position_id": 6, "weekday": 1, "start_time": "09:00", "end_time": "10:00", "required_headcount": 2 }
]

// After: one entry per slot the viewer can fill in, with composition for display
[
  {
    "slot_id": 12,
    "weekday": 1,
    "start_time": "09:00",
    "end_time": "10:00",
    "composition": [
      { "position_id": 5, "position_name": "前台负责人", "required_headcount": 1 },
      { "position_id": 6, "position_name": "前台助理",   "required_headcount": 2 }
    ]
  }
]
```

A slot is included iff `composition ∩ viewer.user_positions ≠ ∅` — i.e., at least one position the viewer is qualified for.

**`GET /publications/{id}/submissions/me`** returns `[ { "slot_id": ... } ]` instead of `[ { "slot_id": ..., "position_id": ... } ]`.

### D-5. Frontend grid collapse

```tsx
// availability-grid.tsx loop becomes:
weekdayShifts.map((shift) => (
  <label key={shift.slot_id}>
    <Checkbox
      checked={selectedSlots.has(shift.slot_id)}
      onChange={(e) => onToggle(shift.slot_id, e.currentTarget.checked)}
    />
    <div>
      <strong>{timeRange(shift)}</strong>
      <span className="caption">
        {shift.composition.map(c => `${c.position_name} × ${c.required_headcount}`).join(' / ')}
      </span>
    </div>
  </label>
))
```

The caption preserves the visual signal "this slot has these roles" — admins designing templates still want operators to know the staffing shape — but the role doesn't determine whether the user can or should tick the box. They tick if they're free.

`selectedSlotPositions` set in component state collapses to `selectedSlotIds`.

### D-6. Backend candidate-pool refactor

`AutoAssignPublication` builds the MCMF graph by:

1. Listing all submissions for the publication: `(user_id, slot_id)` rows.
2. Joining each to `user_positions(user_id)` and `template_slot_positions(slot_id)`.
3. Filtering: a `(user, slot)` is viable iff `user_positions ∩ composition(slot)` is non-empty AND `users.status = 'active'`.
4. For each viable `(user, slot)`, emit a `(u, S)` graph node with an edge to every `(S, P)` cell where `P ∈ user_positions(u)`.

The "submission references a position the user is no longer qualified for" filter from the old design simplifies: if the user has lost *all* of the slot's positions, they drop out of `U(S)`; if they've lost some but kept others, they remain a candidate but only for the surviving cells.

### D-7. Spec text updates

Five requirements modify in `scheduling`:

1. *Availability submission data model* — schema rewords; FK story simplifies.
2. *Availability window* — DELETE URL form changes.
3. *Employee availability endpoints* — body shape, DELETE URL, response shape.
4. *Auto-assign replaces the full assignment set via MCMF* — candidate-pool wording.
5. *Qualification gates employee actions* — scenario rewords from "(slot, position) pair whose position_id is not in user_positions" to "slot whose composition has no overlap with user_positions".

No new requirements; nothing removed.

### D-8. Test rewrites

The column drop cascades through:

- Repository tests building submission rows with explicit position_id → drop the field.
- Service tests asserting submission counts after auto-assign rebuild → recount under the new derivation.
- Handler tests sending `{slot_id, position_id}` body → update.
- Frontend tests asserting per-(slot, position) checkbox rendering → rewrite to per-slot.

The compiler points at every leftover `PositionID` reference on the submission struct after the model change.

## Risks / Trade-offs

- **Risk:** auto-assign behavior subtly changes for users qualified for multiple positions in the same slot. **Today** they had to explicitly tick each desired role. **After** they're considered for any role they're qualified for. → Mitigation: this is precisely the intended behavior change; there are no production users to surprise. Auto-assign's flow solver handles the multi-cell candidacy via the existing per-slot uniqueness node.
- **Risk:** assignment-board's `non_candidate_qualified` (users qualified but not having submitted) is currently per-`(slot, position)`. After this change a user submits per-slot, so "candidate" / "non-candidate" become per-slot too. → Mitigation: the assignment-board response keeps cell-shaped output (`(slot, position)` per cell with candidates/assignments/non_candidate_qualified lists), but each cell's `candidates` is rederived as "users who submitted for this slot AND are qualified for this position". The shape on the wire is unchanged; only the derivation is.
- **Trade-off:** if someone ever wants per-role preference, this change has to be partially undone or sit alongside a new `availability_preferences` table. → Acceptable; YAGNI for now.

## Migration Plan

Single shipping unit. After merge: `make migrate-up` drops the column and reshapes the unique key. Frontend / backend land together so the API surface change doesn't break existing dev sessions mid-flight (and again, no production traffic to worry about).

## Open Questions

None.
