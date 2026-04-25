## Context

The current scheduling model treats `assignments` as a single row that says "user U fills (slot S, position P) for publication P". Approving any shift-change rewrites `assignments.user_id`, which permanently transfers every weekly occurrence of that slot for the rest of the publication. This is correct for "permanent transfer" semantics but wrong for the everyday "swap a class" / "cover next Monday only" semantics.

Separately, `publications` has only `planned_active_from` (no end date). The duration is implicit in admin-issued `POST /end` calls. Without an end date, the model cannot enumerate concrete week occurrences, which is exactly what occurrence-level shift-changes need.

This design introduces (a) a planned end date on publications and (b) an occurrence layer on top of the existing baseline `assignments`. No new functionality is exposed to users beyond the corresponding UI surface for navigating concrete weeks; the user-visible feature unlocked by this change is `add-leave`, which is a separate proposal.

## Goals / Non-Goals

**Goals:**

- Publications carry an explicit `planned_active_until`. Admin extends/shortens it via `PATCH /publications/{id}`.
- Shift-change requests target a specific calendar week (`occurrence_date`). Approving a request writes an `assignment_overrides` row, not a mutation to `assignments`.
- Roster reads layer overrides on top of baseline assignments, so the UI shows correct concrete-week assignments.
- D2 invariant (single non-ENDED publication) is preserved without background jobs.
- `POST /publications/{id}/end` continues to work — reframed as sugar over the new PATCH path.
- No production data migration (confirmed: no production publications exist).

**Non-Goals:**

- Adding `add-leave` itself.
- Materializing per-occurrence rows into `assignments`.
- Auto-assign behavior changes (still produces baseline weekly schedule).
- Cross-publication shift-changes.
- Background jobs / cron / pollers.
- Per-occurrence audit replay for the unchanged baseline.

## Decisions

### D-1. Per-occurrence overrides as a separate table, not nullable column on `assignments`

**Decision:** new `assignment_overrides` table. `assignments` stays exactly as-is.

**Alternatives considered:**

- *Add nullable `occurrence_date` to `assignments` with `UNIQUE(publication_id, user_id, slot_id, occurrence_date)`*: conflates the baseline (1 row per `(pub, slot)` pair, sparse) with deltas (1 row per occurrence-exception, dense). Queries become "is this row baseline or override?" everywhere. Rejected.
- *Materialize all occurrences as `assignments` rows*: a 16-week publication with ~80 weekly slots becomes 1280 rows on creation; auto-assign needs to either operate on baseline-only (defeating the purpose) or write 1280 rows; admin edits become "edit one cell or fan out to all weeks?" UX. Rejected.

The chosen shape is clean: `assignments` answers "what's the weekly baseline?", `assignment_overrides` answers "what individual occurrences differ from the baseline?". Reading a concrete week = `LEFT JOIN` overrides on baseline.

### D-2. `occurrence_date` is `DATE`, not `TIMESTAMPTZ`

**Decision:** `DATE`.

The slot already encodes the time-of-day (`slot.start_time`, `slot.end_time` as `TIME`). Adding TIMESTAMPTZ would let callers specify the wrong time and surface confusing validation errors. The calendar date is the only thing that varies week-to-week. DATE is timezone-free; the system does not handle multi-timezone (declared non-goal in `auth` capability), so a DATE in UTC is unambiguous.

### D-3. `planned_active_until` is `NOT NULL`

**Decision:** required at create.

**Alternatives considered:**

- *Nullable, NULL means "open-ended"*: leaks NULL semantics into every effective-state read; admin can forget to set an end date and the publication runs forever; "open-ended" was never an actual product requirement. Rejected.

Admin must commit to an end at create. Always extensible via `PATCH`.

### D-4. ENDED is time-driven; lazy write-through on next mutating action

**Decision:** the effective-state resolution gains an "ended by clock" rule:

```
DRAFT (stored)
  │ time: NOW >= submission_start_at
  ▼
COLLECTING (effective; stored may still be DRAFT
            until first submission write-through)
  │ time: NOW >= submission_end_at
  ▼
ASSIGNING (effective)
  │ admin: POST /publish
  ▼
PUBLISHED (stored)
  │ admin: POST /activate
  ▼
ACTIVE (stored)
  │ time: NOW >= planned_active_until
  ▼
ENDED (effective; stored may still be ACTIVE
       until the on-create sweep advances it)
```

Stored state advances to `ENDED` only when a fresh `POST /publications` performs a sweep step (single `UPDATE … WHERE state='ACTIVE' AND planned_active_until <= NOW()`). This preserves the "no background job" rule and parallels the existing lazy `COLLECTING` write-through.

**Alternative rejected:** lazy write on every read. Reads must stay read-only; introducing writes breaks caching and complicates request semantics.

### D-5. `POST /end` reframed as sugar for `PATCH … {planned_active_until: NOW()}`

**Decision:** keep the endpoint shape (existing UI/tests don't break) but reroute through PATCH internally. Spec text removes "manual ACTIVE → ENDED" as a state machine arrow and replaces it with "admin can short-circuit by setting until to now".

The `ended_at` column on `publications` is **dropped**. It was always populated to NOW() by `POST /end`, which is now equivalent to `planned_active_until` after the admin shortens it. The audit log already records when the admin acted; the column was never queried for anything other than display, and display can read `planned_active_until` directly.

`activated_at` stays — it is still the only record of when admin manually transitioned PUBLISHED → ACTIVE.

### D-6. Shift-change `expires_at` is per-occurrence

**Decision:** `expires_at = publication.planned_active_from + (slot.weekday - 1) * INTERVAL '1 day' + slot.start_time`, computed at request creation.

**Alternative rejected:** `expires_at = publication.planned_active_until`. A request offering "5/04 Monday 9-12" should expire when 5/04 9:00 arrives, not when the entire publication ends. The current per-publication value was acceptable when there was no occurrence concept; with one, the new derivation is the obvious choice.

The existing "lazy expiry on read" behavior is unchanged in intent — only the value of `expires_at` differs.

### D-7. Approving a shift-change writes overrides; baseline `assignments` is untouched

**Decision:** the in-tx apply path:

- For `give_direct` / `give_pool`: insert one row in `assignment_overrides` recording `(requester_assignment_id, occurrence_date, counterpart_user_id)`.
- For `swap`: insert two rows recording the cross-assignment for both occurrences (requester's and counterpart's, which may be the same date or different dates).
- The in-tx optimistic-lock check now compares `(assignment_id, occurrence_date)` pairs against the request's captured values (vs the previous `(assignment_id, publication_id, user_id)` triple). `ErrShiftChangeAssignmentMiss` is returned if the baseline assignment row no longer exists (deleted by admin) — overrides are derived from baselines, so a missing baseline invalidates the request.

**Alternatives considered:**

- *Approve mutates `assignments`, then immediately writes a "reverse" override for all subsequent weeks*: clever but fragile; the publication's end date may shift, the baseline change pollutes audit trails, and admins reading the assignment row see "Bob" for a slot Bob holds for one week only. Rejected.

### D-8. Cascade-invalidate spans every pending request for the deleted assignment

**Decision:** when admin deletes an assignment, every pending request that names that `requester_assignment_id` *or* `counterpart_assignment_id` (regardless of `occurrence_date`) is transitioned to `invalidated`, with one cascade audit event and one resolution email per request — same shape as today, just iterating over potentially-multiple rows for the same baseline.

### D-9. `assignment_overrides` does not require a position_id

**Decision:** the override carries only `(assignment_id, occurrence_date, user_id)`. The position is derived from the baseline `assignments.position_id`.

A swap or give never changes the position — it only changes "who is filling this `(slot, position)` for this week". If the receiver is qualified for the slot's position, the swap/give is allowed; the override row therefore does not need to repeat the position.

### D-10. New error code

| Code | HTTP | When |
|---|---|---|
| `INVALID_OCCURRENCE_DATE` | 400 | Request specifies an `occurrence_date` outside `[planned_active_from, planned_active_until)`, or whose computed start time is `<= NOW()` at creation, or whose weekday does not match the slot's weekday. |

Existing codes that change behavior:

| Code | HTTP | Updated meaning |
|---|---|---|
| `INVALID_PUBLICATION_WINDOW` | 400 | Now also rejected if `planned_active_from >= planned_active_until`. |
| `SHIFT_CHANGE_INVALIDATED` | 409 | Triggered also when the override path observes the baseline assignment no longer exists or no longer belongs to the captured user. |

### D-11. Migration

Single migration adds the schema changes. The `description` column is included
because the PATCH endpoint accepts it, and `counterpart_occurrence_date` is
included because swap requests require a concrete counterpart occurrence. Goose
Up:

```sql
-- +goose Up
-- +goose StatementBegin

ALTER TABLE publications
    ADD COLUMN description TEXT NOT NULL DEFAULT '',
    DROP COLUMN ended_at,
    ADD COLUMN planned_active_until TIMESTAMPTZ;

UPDATE publications
   SET planned_active_until = planned_active_from + INTERVAL '7 days'
 WHERE planned_active_until IS NULL;

ALTER TABLE publications
    ALTER COLUMN planned_active_until SET NOT NULL,
    DROP CONSTRAINT publications_submission_window_check,
    ADD CONSTRAINT publications_submission_window_check
        CHECK (submission_start_at < submission_end_at
               AND submission_end_at <= planned_active_from
               AND planned_active_from < planned_active_until);

CREATE TABLE assignment_overrides (
    id              BIGSERIAL PRIMARY KEY,
    assignment_id   BIGINT NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
    occurrence_date DATE NOT NULL,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (assignment_id, occurrence_date)
);

CREATE INDEX assignment_overrides_user_id_idx ON assignment_overrides (user_id);

ALTER TABLE shift_change_requests
    ADD COLUMN occurrence_date DATE,
    ADD COLUMN counterpart_occurrence_date DATE;

UPDATE shift_change_requests AS s
   SET occurrence_date = (SELECT planned_active_from::date
                            FROM publications p WHERE p.id = s.publication_id)
 WHERE occurrence_date IS NULL;

ALTER TABLE shift_change_requests
    ALTER COLUMN occurrence_date SET NOT NULL;

-- +goose StatementEnd
```

The `UPDATE` blocks set sane defaults for any rows that exist in development databases, then the `SET NOT NULL` enforces the invariant going forward. There is no production data, so these defaults never harm anyone real.

Goose Down:

```sql
-- +goose Down
-- +goose StatementBegin

ALTER TABLE shift_change_requests
    DROP COLUMN counterpart_occurrence_date,
    DROP COLUMN occurrence_date;

DROP INDEX assignment_overrides_user_id_idx;
DROP TABLE assignment_overrides;

ALTER TABLE publications
    DROP CONSTRAINT publications_submission_window_check,
    ADD CONSTRAINT publications_submission_window_check
        CHECK (submission_start_at < submission_end_at
               AND submission_end_at <= planned_active_from);

ALTER TABLE publications
    ADD COLUMN ended_at TIMESTAMPTZ,
    DROP COLUMN planned_active_until,
    DROP COLUMN description;

-- +goose StatementEnd
```

The migrations-roundtrip CI exercises Down→Up→Down, so any data in `assignment_overrides` or any `occurrence_date` value would be lost on roll-back; we accept this because there is no production data and roll-back is a developer / CI concern.

### D-12. UI: roster week selection drives shift-change occurrence

**Decision:** the roster gains week-by-week navigation over the publication's valid active window. Give/swap dialogs are opened from a concrete roster week and carry that slot's `occurrence_date` into the create request; the dialog displays the chosen occurrence alongside the slot so the user can verify the week before submitting. Approve/list UI shows the date alongside the slot.

This is the only user-visible UI change in this proposal; without it, occurrences are invisible. Roster view also gains week-by-week navigation.

## Risks / Trade-offs

- **Risk: roster read becomes a `LEFT JOIN`.** → Mitigation: overrides are sparse; modest data volumes; index on `(assignment_id, occurrence_date)` and `user_id` covers the lookups.
- **Risk: D2 sweep at create-time fails to advance an ACTIVE publication whose until is in the past, blocking the new create.** → Mitigation: sweep runs in the same transaction as the create; the sweep is a single conditional UPDATE; if it fires, the partial unique index then admits the new row. If the sweep finds zero rows (no active publications) or an unrelated error, the failure mode is surfaced as `PUBLICATION_ALREADY_EXISTS` or `INTERNAL_ERROR`. No silent data corruption.
- **Risk: cascade-invalidate now iterates instead of single-update.** → Mitigation: still bounded (occurrences ≤ ~52 per publication for a 1-year publication), and the cascade is best-effort (existing requirement).
- **Risk: occurrence-date validation is subtle.** → Mitigation: a single `IsValidOccurrence(publication, slot, date)` helper that handles weekday-match, in-window, and not-in-past in one place; reused by every endpoint that accepts `occurrence_date`.
- **Trade-off: admin UI reads "Bob" in baseline `assignments` even when the next 4 weeks have an override to Carol.** → Acceptable: admin views see baseline + overrides combined; the raw `assignments` table is rarely surfaced directly.
- **Trade-off: dropping `ended_at` is a schema break for any reader.** → Mitigation: code search confirms `ended_at` is only read by the publication serializer and the `POST /end` handler; both are updated in this change.

## Migration Plan

No staged rollout; this is a single shipping unit on the `change/scheduling-occurrence-model` branch. The CI's `migrations-roundtrip` job validates Up/Down. After merge:

1. Local devs: `make migrate-down && make migrate-up && make seed` to refresh local DBs.
2. No external API surface changes break; the SCRT request body adds a required field, but no external clients exist.
3. Docs (`README.md`) get a one-line note about `planned_active_until`; user-facing release note is deferred to `add-leave` (which is what users will actually see).

## Open Questions

None. The leave-feature scope question, week-navigation pagination, and per-week notifications all belong to `add-leave`, not here.
