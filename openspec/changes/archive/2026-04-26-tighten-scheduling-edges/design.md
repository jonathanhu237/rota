## Context

Four spec-vs-code drifts were uncovered by a boundary audit of the scheduling subsystem. Each is a "the spec already promises this; the code only honors it under serial execution" gap.

| # | Spec promise | Where it leaks |
|---|---|---|
| A3 | Employees only assigned to positions in their `user_positions` | `ListAssignmentCandidates` reads stale submissions without joining `user_positions` |
| #2 | Apply does not create an overlapping weekly schedule | `ensureSwapFitsSchedule` / `ensureGiveFitsSchedule` read outside the apply tx |
| #3 | Admin `CreateAssignment` does not create an overlap | `hasAssignmentTimeConflict` reads outside the insert tx |
| S2 | Disabled users cannot receive assignments | `user.status` read pre-tx; not re-checked at apply |

A3 is independent — it's a missing JOIN at one read site. #2, #3, S2 are the same pattern: "check outside tx, mutate inside tx." The unifying fix is a shared helper that locks the user's relevant rows + re-loads status, called inside the existing transaction at all three entry points.

## Goals / Non-Goals

**Goals:**

- Auto-assign honours current qualification, not historical.
- Time-conflict check on apply (swap and give) is concurrency-safe — no two paths can race past it.
- Admin `CreateAssignment` overlap check has the same concurrency safety.
- Disabled-user check on apply and admin `CreateAssignment` is concurrency-safe.

**Non-Goals:**

- Cleanup of stale `availability_submissions` rows when `user_positions` change (we filter at read instead).
- Session invalidation on user disable (L3, separate change).
- New error codes; existing `SHIFT_CHANGE_TIME_CONFLICT`, `ASSIGNMENT_TIME_CONFLICT`, `ASSIGNMENT_USER_ALREADY_IN_SLOT`, `USER_DISABLED` cover all cases.

## Decisions

### A3 — qualification filter at read time, not cascade-delete on user_positions change

`ListAssignmentCandidates` SQL becomes:

```sql
SELECT asub.slot_id, asub.position_id, u.id, u.name, u.email
FROM availability_submissions asub
INNER JOIN users u ON u.id = asub.user_id
INNER JOIN user_positions up
  ON up.user_id = asub.user_id
  AND up.position_id = asub.position_id
WHERE asub.publication_id = $1
  AND u.status = 'active'        -- bonus: active-only at read time
ORDER BY asub.slot_id ASC, asub.position_id ASC, u.id ASC;
```

Stale submission rows are NOT deleted — they linger in the table. If admin re-adds the position to the user's qualifications, the join re-includes them. This is **read-time filtering**, not write-time cleanup.

**Rejected**: cascading-delete `availability_submissions` when `user_positions` changes. Rejected because (a) admin fat-fingering a qualification edit would silently destroy the user's availability data; (b) the user already ticked, so the system "remembering" their preference is correct semantically; (c) the join is one extra clause, no measurable cost.

**Bonus**: we also filter `u.status = 'active'` at this read. A disabled user's submissions become invisible to auto-assign automatically; one less surface for S2-like races.

### #2 + #3 + S2 — single tx-internal helper

New repository function:

```go
// lockAndCheckUserAssignmentSchedule takes a per-(publication,user)
// transaction-scoped advisory lock, locks every assignment row this user has
// in the publication (FOR UPDATE), joins template_slots to get time windows,
// computes the resulting weekly schedule including the proposed additions,
// and returns ErrTimeConflict if any two share a weekday and overlap. It also
// verifies users.status is 'active' for the user and returns ErrUserDisabled
// if not.
//
// Callers MUST hold the row-level lock for the lifetime of their
// transaction; otherwise the lock is released on Commit/Rollback as usual.
func lockAndCheckUserSchedule(
    ctx context.Context,
    tx *sql.Tx,
    publicationID, userID int64,
    additions []SlotTimeWindow,
    excludeAssignmentIDs []int64,
) error
```

**Why `excludeAssignmentIDs`**: a swap removes one assignment row from the user's schedule even as it adds another. The old row is captured in `excludeAssignmentIDs`, the new in `additions`. For ApplyGive, `excludeAssignmentIDs` is empty (give just transfers an existing row's `user_id`). For CreateAssignment, both `additions` has the new slot's window and `excludeAssignmentIDs` is empty.

**Why an advisory lock in addition to row locks**: when a user has zero existing
assignment rows in a publication, `SELECT ... FOR UPDATE` has nothing to lock.
A per-(publication,user) transaction advisory lock closes that empty-schedule
hole. Row locks are still taken for concrete assignment rows once they exist.

**SQL skeleton** for the lock+read step:

```sql
SELECT a.id, ts.weekday, ts.start_time, ts.end_time
FROM assignments a
INNER JOIN template_slots ts ON ts.id = a.slot_id
WHERE a.publication_id = $1
  AND a.user_id = $2
FOR UPDATE OF a;
```

Then in Go: filter out `excludeAssignmentIDs`, append `additions`, run the existing time-conflict predicate. Then a separate `SELECT status FROM users WHERE id = $2 FOR UPDATE` serializes the apply with admin status changes before accepting the write.

**Concurrency analysis**:

Two concurrent shift-change applies, both involving Alice:
- Apply A: tx_A starts. `lockAndCheckUserSchedule(tx_A, pub, Alice, ...)` acquires Alice's per-publication advisory lock, then row-level locks on every Alice assignment in pub.
- Apply B: tx_B starts. Calls the same helper for Alice. The advisory lock blocks until tx_A commits or rolls back.
- Apply A commits or rolls back. Locks released.
- Apply B's lock acquires; re-reads Alice's assignment set (now reflects whatever A did), re-checks conflict.

The serialization point is per-(publication, user). Two unrelated users' applies don't block each other.

**Rejected**: `SET TRANSACTION ISOLATION LEVEL SERIALIZABLE`. Rejected because (a) requires broad retry semantics in the application layer, (b) the exact race can be closed with scoped per-user transaction locks plus row-level locks, and (c) the resulting contention boundary remains explicit.

### Service layer collapses pre-tx check into a fast-fail

Today `ensureSwapFitsSchedule` runs in the service layer before `ApplySwap`. We **keep it** as a non-locking fast-fail — it rejects 99% of bad requests with a single read, before the tx is even started. The in-tx helper is the correctness floor for the 1% that races through.

Same for `hasAssignmentTimeConflict` in `CreateAssignment`: pre-tx fast-fail is preserved; in-tx re-check is the floor.

This double-check is intentional: the pre-tx check gives quick feedback to the API caller (no need to wait for transaction overhead on obvious conflicts); the in-tx check defends against the race.

**Rejected**: removing the pre-tx checks entirely. Rejected because (a) marginal correctness improvement (the in-tx check would still catch all races), (b) marginal latency cost on the common-case "already conflicting" rejection path.

### S2's user-status re-check at apply

In the same `lockAndCheckUserSchedule` helper, `SELECT status FROM users WHERE id = $userID FOR UPDATE` is done inside the tx. If `status != 'active'`, return `ErrUserDisabled`.

Why lock `users`: the production smoke path can admit a request while the user is still active, then have an admin disable the user before the apply reaches its status check. Locking the user row makes the apply/status-change ordering explicit: if the disable commits first, apply returns `ErrUserDisabled`; if apply has already locked and observed the active user, the later disable waits for the apply to finish.

**Rejected**: non-locking `SELECT status`. Rejected because it leaves the HTTP in-flight disable race observable in production smoke.

**Rejected**: only checking pre-tx. Rejected because that's exactly what's broken today (S2).

### CreateAssignment uses the same helper

The admin `CreateAssignment` path:

1. Pre-tx: validate input, resolve publication state, resolve slot, resolve slot-position composition, fast-fail overlap check (preserved for UX latency).
2. Begin tx.
3. `lockAndCheckUserSchedule(tx, publication, target_user, additions=[new slot's window], exclude=[])`.
4. INSERT assignment.
5. Audit emit.
6. Commit.

Step 3 is the new addition. It catches both the time-conflict race and the user-status race in a single helper call.

**Rejected**: only adding the lock to apply paths. Rejected because the admin path has the same race shape; fixing only the employee paths would be inconsistent.

## Risks / Trade-offs

- **Risk**: row-level locks on every user assignment could deadlock under unusual concurrency patterns (two apply transactions both involving users A and B in different orders).
  **Mitigation**: the helper sorts assignment IDs before issuing `FOR UPDATE` to enforce a consistent lock order. Postgres deadlock detector handles edge cases by aborting one tx with a serialization-failure-like error; the service should map this to a 503 Retry. (Add this mapping as a small bonus task.)
- **Risk**: pre-tx fast-fail and in-tx check disagree (e.g., pre-tx accepts, in-tx rejects). Already an accepted concession — pre-tx is best-effort, in-tx is the floor. No rollback semantics needed because the in-tx error is the single source of truth.
- **Trade-off**: `lockAndCheckUserSchedule` is a relatively chunky helper — it touches three concerns (locking, time-conflict, status). We accept this cohesion as preferable to spreading three near-identical helpers across three call sites.
- **Trade-off**: `users.status` is read `FOR UPDATE` inside the tx. Admin status changes for the same user can briefly wait behind an apply path, but the contention boundary is per user and only on mutating scheduling operations.

## Migration Plan

No data migration. Deployment:

1. Deploy backend (repository helper + service-layer call sites). Existing transactions retain their read-then-write structure but gain the in-tx check. No DB schema change.
2. Frontend: add copy for the transient scheduling retry error.
3. Rollback: revert backend; everything reverts to current "outside-tx check" behavior.

## Open Questions

None.
