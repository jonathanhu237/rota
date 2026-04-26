## Context

This change wires a `leaves` entity on top of the occurrence-level `shift_change_requests` introduced by `scheduling-occurrence-model` (Phase 1). It is deliberately thin: the heavy lifting (per-week occurrence modeling, override application, qualification checks, optimistic locks, cascade-invalidation, email) is already in the SCRT layer. Leaves add a name, a category, a free-form reason, and a self-service entry-point — nothing more.

The change is the second half of a stacked pair. Phase 1 ships the architectural primitives; Phase 2 surfaces the user-visible product. AGENTS.md is updated in parallel with this proposal to record the stacked-branches workflow that makes the parallel apply possible.

## Goals / Non-Goals

**Goals:**

- A `leaves` row records "user U requested coverage for occurrence O on (assignment A, date D), under category C, with reason R, via mechanism M (give_direct or give_pool)".
- Leave state is fully derived from the underlying SCRT — no second state machine.
- Counterpart UX (approve / reject) reuses the existing SCRT endpoints; only the entry-point and the wrapper differ.
- Cancellation is leave-level and idempotent: `POST /leaves/:id/cancel` cancels the underlying SCRT iff `pending`; if the SCRT is already terminal, the call is a no-op success.
- `share_url = /leaves/:id` is shareable to anyone in the system.

**Non-Goals:**

- Admin approval flow.
- Leave quotas, attendance computation, or `affects_attendance` semantics.
- Admin-managed `leave_categories` table.
- Cross-publication leaves.
- Atomic batch submission. The frontend issues one POST per occurrence.

## Decisions

### D-1. `leaves` is 1:1 with a single `shift_change_request`

**Decision:** every leave has exactly one underlying SCRT, exposed via `leaves.shift_change_request_id BIGINT NOT NULL UNIQUE`. Conversely, an SCRT either is a leave (`leave_id` non-NULL) or is a regular shift-change (`leave_id` NULL).

**Alternatives considered:**

- *1:N — one leave wraps several occurrences*: the user explicitly preferred 1:1 in the explore session ("一次请假就对应一个班，只是我们可以给多个 URL 给用户，这样逻辑就简单"). 1:N forces a third state machine on top of the per-SCRT one (what does it mean if 2 of 3 SCRTs approved and 1 expired?), which is exactly the complication 1:1 dodges.
- *Embed leave fields directly into shift_change_requests*: pollutes the SCRT table for callers that don't care, complicates listing leaves separately, and entangles audit. Rejected.

The 1:1 join is cheap. State derivation is a simple SQL `CASE` over `shift_change_requests.state`.

### D-2. Leave state is derived, not stored

**Decision:** `leaves` has no `state` column. Reads compute it from the joined SCRT via the table:

| `shift_change_requests.state` | derived `leave.state` |
|---|---|
| `pending` | `pending` |
| `approved` | `completed` |
| `expired` | `failed` |
| `rejected` | `failed` |
| `cancelled` | `cancelled` |
| `invalidated` | `cancelled` |

`approved` becomes `completed` (the leave succeeded — someone took the shift). `expired` and `rejected` both surface as `failed` (the slot stayed unfilled when the occurrence arrived, OR the targeted counterpart said no). `cancelled` and `invalidated` collapse to `cancelled` because admin-driven invalidation is functionally equivalent to "the leave didn't happen".

**Alternative rejected:** stored `state` synced via trigger or service-layer write-through. Adds a sync-bug class for zero benefit since the join is cheap and the truth is unambiguously the SCRT.

### D-3. The PUBLISHED-vs-ACTIVE gate split

**Decision:** the ShiftChangeService is split into two surfaces:

- `ShiftChangeService.CreateRequest` — public, gated on `effective_state = PUBLISHED`, asserts `leave_id IS NULL`. This is the existing endpoint behavior.
- `ShiftChangeService.CreateRequestTx` — internal (service-package only), takes `leave_id *int64` argument and a `*sql.Tx`. Performs all SCRT validation but lets the caller own the gate and the surrounding transaction.

`LeaveService.Create` opens the transaction, asserts `effective_state = ACTIVE`, calls `CreateRequestTx` with `leave_id` set, then inserts the leaves row.

This split is the cleanest way to keep the gate logic associated with each entry-point — regular shift-change owns PUBLISHED, leave owns ACTIVE — without duplicating the rest of the SCRT validation.

**Alternative considered:** add a `mode` parameter to the public `CreateRequest`. Rejected because then the public method needs to interrogate the parameter and switch behavior, mixing two product flows in one signature.

### D-4. `Activation bulk-expires` only touches `leave_id IS NULL`

**Decision:** the existing requirement *Activation bulk-expires pending shift-change requests* is amended to:

```sql
UPDATE shift_change_requests
   SET state = 'expired'
 WHERE publication_id = $1
   AND state = 'pending'
   AND leave_id IS NULL
```

In practice this is a no-op change today: leaves are created during ACTIVE, after activation has already happened, so `leave_id IS NULL` is always true at the time the activation runs. The clause is defensive — it guards against a future flow that creates leave-bearing rows in PUBLISHED, and it makes the intent explicit in the SQL.

### D-5. Cancel is leave-level, not occurrence-level

**Decision:** `POST /leaves/:id/cancel` is the only cancellation endpoint that the leave UI uses. It cascades to `ShiftChangeService.Cancel` on the underlying SCRT. If the SCRT is already terminal (e.g., counterpart already approved), the leave is a no-op success — the leave's derived state will reflect the underlying terminal SCRT.

**Alternative rejected:** require the user to cancel the SCRT directly. The product affordance is "cancel my leave"; surfacing the SCRT layer to the employee is a leaky abstraction.

### D-6. Share URL is `/leaves/:id`, visible to any logged-in user

**Decision:** GET `/leaves/:id` is `RequireAuth`-gated only. The body returns leave metadata + the underlying SCRT (slot info, type, counterpart if any, current state). The frontend then decides which buttons to show based on the SCRT's authorization rules (existing logic).

This intentionally mirrors `give_pool` exposure: a `give_pool` request was always visible to any qualified user in the system. Leave doesn't change that posture.

### D-7. Preview endpoint shape

**Decision:** `GET /users/me/leaves/preview?from=YYYY-MM-DD&to=YYYY-MM-DD`. Response:

```json
{
  "occurrences": [
    {
      "assignment_id": 42,
      "occurrence_date": "2026-05-04",
      "slot": { "id": 7, "weekday": 1, "start_time": "09:00", "end_time": "12:00" },
      "position": { "id": 3, "name": "Position A" },
      "occurrence_start": "2026-05-04T09:00:00Z",
      "occurrence_end":   "2026-05-04T12:00:00Z"
    },
    ...
  ]
}
```

Resolution:

1. Look up the current ACTIVE publication (D2 invariant — exactly one).
2. Find all `assignments` rows where `user_id = viewer.id`.
3. For each assignment, enumerate its occurrences (per Phase 1's *Occurrence concept and computation*) within `[from, to]`, filtered to occurrences whose `occurrence_start > NOW()`.
4. Return them sorted by `occurrence_start` ascending.

The endpoint is read-only and idempotent.

**Edge cases:**

- No active publication → empty `occurrences` array, HTTP 200.
- `from > to` → HTTP 400 `INVALID_REQUEST`.
- `from` or `to` outside the publication's window → silently clamps to the window (the user filtered too widely; serve only what's possible).
- Past-only range → empty `occurrences`, HTTP 200.

### D-8. Schema migration

```sql
-- +goose Up
-- +goose StatementBegin

CREATE TABLE leaves (
    id                       BIGSERIAL PRIMARY KEY,
    user_id                  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    publication_id           BIGINT NOT NULL REFERENCES publications(id) ON DELETE CASCADE,
    shift_change_request_id  BIGINT NOT NULL UNIQUE
                                 REFERENCES shift_change_requests(id) ON DELETE CASCADE,
    category                 TEXT NOT NULL CHECK (category IN ('sick','personal','bereavement')),
    reason                   TEXT NOT NULL DEFAULT '',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX leaves_user_id_idx        ON leaves(user_id);
CREATE INDEX leaves_publication_id_idx ON leaves(publication_id);

ALTER TABLE shift_change_requests
    ADD COLUMN leave_id BIGINT REFERENCES leaves(id) ON DELETE SET NULL;

CREATE INDEX shift_change_requests_leave_id_idx ON shift_change_requests(leave_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX shift_change_requests_leave_id_idx;
ALTER TABLE shift_change_requests DROP COLUMN leave_id;

DROP INDEX leaves_publication_id_idx;
DROP INDEX leaves_user_id_idx;
DROP TABLE leaves;

-- +goose StatementEnd
```

The `ON DELETE SET NULL` on `shift_change_requests.leave_id` is intentional: deleting a leave (rare; admin-only path, not built in this change) leaves the underlying SCRT intact. Conversely, `leaves.shift_change_request_id` cascades — if the SCRT is gone, the leave has no meaning.

Goose migration roundtrip is exercised by CI's existing `migrations-roundtrip` job.

### D-9. Audit actions

Register two new actions in the audit capability's action catalog:

- `leave.create` — emitted in the same transaction as the leaves insert, metadata `{ leave_id, user_id, publication_id, shift_change_request_id, category }`. `reason` is intentionally NOT in metadata: it's free-form and may contain personal information.
- `leave.cancel` — emitted on `POST /leaves/:id/cancel`, metadata `{ leave_id }`.

The underlying SCRT operations (`shift_change.create`, `shift_change.approve`, etc.) continue to fire as today.

### D-10. Error codes

| Code | HTTP | Reused / new | Trigger |
|---|---|---|---|
| `INVALID_REQUEST` | 400 | reused | malformed body, `from > to`, etc. |
| `INVALID_OCCURRENCE_DATE` | 400 | reused | `occurrence_date` validation (delegated to SCRT layer) |
| `PUBLICATION_NOT_ACTIVE` | 409 | reused | leave create when no ACTIVE publication exists, or wrong effective state |
| `LEAVE_NOT_FOUND` | 404 | new | `GET /leaves/:id` or `POST /leaves/:id/cancel` for a missing row |
| `LEAVE_NOT_OWNER` | 403 | new | `POST /leaves/:id/cancel` by a non-requester |
| `NOT_QUALIFIED` | 403 | reused | counterpart not qualified, or requester not qualified for own offered slot |
| `SHIFT_CHANGE_INVALID_TYPE` | 400 | reused | `type ∉ {give_direct, give_pool}` (swap is rejected for leaves) |
| `SHIFT_CHANGE_SELF` | 400 | reused | `give_direct` with `counterpart_user_id = viewer.id` |
| `USER_DISABLED` | 409 | reused | counterpart disabled (give_direct) |

`type = swap` is explicitly rejected at leave-create — swap doesn't make sense as a leave (the requester wants to *not work*, not exchange). Returning `SHIFT_CHANGE_INVALID_TYPE` is consistent with how the SCRT layer already handles bad types.

### D-11. No swap for leaves

`type` for leaves is restricted to `{give_direct, give_pool}`. The handler rejects `swap` at the body-validation step.

## Risks / Trade-offs

- **Risk: leaves and SCRT diverge in audit.** A reader inspecting the audit log sees both `leave.create` and `shift_change.create` for the same operation; could be confused as duplicates. **Mitigation:** the metadata makes the relationship explicit (`leave.create` has `shift_change_request_id`; `shift_change.create` has `leave_id`).
- **Risk: derived state hides edge cases.** A future feature might want a leave that does *not* mirror its SCRT — e.g., "approved by admin override". **Mitigation:** crossing that bridge when we get to it; the schema can grow a stored-state column in a future migration without breaking the derived-state callers.
- **Risk: stacked-branches rebase pain.** Phase 1 review can introduce changes that conflict with Phase 2 spec deltas (especially in `specs/scheduling/spec.md`). **Mitigation:** rebase early and often; AGENTS.md update names Claude as the rebaser.
- **Trade-off: 1:1 means the UI does N round-trips for an N-shift leave block.** Latency-wise N is small (typically 2-5); UX-wise the per-shift submit gives the employee a chance to fix individual rejections without losing the others.

## Migration Plan

Single shipping unit. The migration runs after Phase 1's migration (number sequencing handled by goose). Order:

1. Phase 1 archived → main has `assignment_overrides` + `occurrence_date`.
2. Phase 2 worktree rebased onto main.
3. CI green → archive add-leave → merge to main.

Rollback is `goose down` on the leaves migration; SCRT rows with `leave_id` set are detached cleanly via the `SET NULL` rule.

## Open Questions

None blocking. Pre-emptively asked-and-answered in the explore session: category list is fixed at three; preview is single-publication; share URL is logged-in-only-but-otherwise-public; cancel is leave-level; no batch atomic.
