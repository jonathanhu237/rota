## Why

A boundary audit of the existing scheduling flows (auto-assign, admin assignment edits, and the three shift-change paths) surfaced four concrete code-vs-spec gaps. None corrupt data, none crash; all four put the system into states the spec already promises will not happen:

1. **Auto-assign can place a user in a position they are no longer qualified for.** `ListAssignmentCandidates` reads `availability_submissions` joined to `users`, but does NOT join `user_positions`. If an admin removes a position from a user's qualifications between the COLLECTING window and auto-assign time, the user's stale submission rows stay valid in this read, so MCMF can assign them to a position that is no longer in their `user_positions`. The spec's "Qualification gates employee actions" requirement is violated at the point auto-assign writes the row.
2. **Shift-change apply has a TOCTOU race on the time-conflict check.** `ensureSwapFitsSchedule` and `ensureGiveFitsSchedule` read `ListPublicationAssignments` outside the apply transaction. Between the read and `ApplySwap` / `ApplyGive`'s `lockAssignment`, another shift-change involving the same user can commit, mutating the user's effective schedule. The apply does not re-check; the lock only protects the row's `user_id` integer, not the receiver's broader weekly schedule. Result: a user can end up with two same-weekday assignments whose slots overlap, contradicting the spec's "Time-conflict check before applying" requirement.
3. **Admin `CreateAssignment` has the identical TOCTOU race.** Same shape, different entry point. The overlap check loads the user's existing weekday assignments, releases that read, and INSERTs. A concurrent shift-change committing the same user's row in the gap between read and INSERT defeats the check. Spec's "Admin CreateAssignment rejects same-weekday slot overlap" is bypassable under concurrency.
4. **Apply paths and `CreateAssignment` do not re-check `user.status` at write time.** A user disabled by an admin after an approve operation is authorized but before the apply transaction's status check can complete the approval. Same shape: admin can `CreateAssignment` for a user who was disabled microseconds earlier in another tx. Spec's "Reject assignment of disabled users" requirement is met at the read but not at the write.

The unifying observation: **time-conflict, qualification, and user-status checks all currently happen outside the apply transaction**. Any concurrent mutation of the same user's assignments or status defeats them. The fix is to push these three checks into the apply transaction's locked region.

## What Changes

- **A3 — qualification filter at read time**: `ListAssignmentCandidates` adds `INNER JOIN user_positions ON (user_id, position_id)` so the candidate set returned to auto-assign reflects the *current* qualification state, not whatever it was when the submission was created.
- **#2 + #3 — tx-internal time-conflict check**: introduce a shared repository helper that, given `(tx, publication_id, user_id, additions[])`, locks every existing assignment row for that user in that publication (via `SELECT ... FOR UPDATE`) and re-runs the time-conflict predicate against `additions ∪ existing`. Wire it into `ApplySwap`, `ApplyGive`, and `CreateAssignment` *inside* their existing transactions, replacing the current outside-tx `ensureSwap` / `ensureGive` and the outside-tx `hasAssignmentTimeConflict`.
- **S2 — tx-internal user-status check**: in the same locked region, re-load `user.status` for every user being mutated (receiver of give, both sides of swap, and the target of CreateAssignment). Reject with `ErrUserDisabled` if any participant is disabled. Spec already promises disabled users cannot receive assignments; this closes the timing gap.
- **Service-layer cleanup**: the existing `ensureSwapFitsSchedule` / `ensureGiveFitsSchedule` service methods become thin wrappers that pass through to the new repo helper, or are removed entirely if the apply tx covers all callers. The service-layer `hasAssignmentTimeConflict` in `publication_pr4.go` likewise consolidates onto the shared helper.
- **Spec compliance**: existing requirements ("Time-conflict check before applying", "Admin CreateAssignment rejects same-weekday slot overlap", "Reject assignment of disabled users", "Qualification gates employee actions") are all *strengthened* with scenarios that explicitly cover the concurrent path.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `scheduling`:
  - `Auto-assign replaces the full assignment set via MCMF` — clarify candidate filtering: `availability_submissions` rows are filtered through `user_positions` at read time, so users no longer qualified for a position do not appear as candidates. Stale submission rows remain in the table (admin re-adding the position re-enables them).
  - `Time-conflict check before applying` — strengthen to require the check is performed inside the apply transaction after locking all the user's existing assignments, so concurrent mutations cannot defeat it.
  - `Admin CreateAssignment rejects same-weekday slot overlap` — same strengthening applied to the admin path.
  - `Reject assignment of disabled users` — the check SHALL be re-evaluated inside the apply transaction (in addition to the existing pre-tx check). Adds a scenario covering the "active at request creation, disabled before apply status check" race.

## Non-goals

The following are deliberately excluded:

- **Cleaning up `availability_submissions` rows when admin removes a user's position.** A3 is fixed by filtering at read time, not by cascading deletes. Rationale: if admin re-adds the position the next day, the user's submission should "come back" (they already expressed availability); cascade-deletion would force them to re-tick.
- **Session invalidation when a user is disabled (L3).** Out of scope for this change and already governed by the auth capability. The race we close here is "request was authorized before disable, but apply reaches its status check after disable."
- **Bootstrap admin status-edit reflexivity** (admin disabling themselves) — not specifically addressed; existing role checks suffice.
- **Race between admin `DeleteAssignment` and the same admin's concurrent `CreateAssignment`** — admin operations on the same publication serialize through Caddy + Go's HTTP handler runtime; same-admin concurrent edits are not a realistic pattern.
- **Revisiting `ensureSwapFitsSchedule`'s "no understaffing rejection at apply" rule.** Spec already accepts understaffing post-give; this change preserves that.
- **Changing normal conflict error codes.** Existing `SHIFT_CHANGE_TIME_CONFLICT` and `ASSIGNMENT_TIME_CONFLICT` cover semantic scheduling conflicts. A separate retryable code is only used for rare database deadlock retry cases.

## Impact

- **Backend repository layer**:
  - `backend/internal/repository/assignment.go`: `ListAssignmentCandidates` SQL adds `INNER JOIN user_positions`. New helper `lockUserAssignmentsForPublication(ctx, tx, publication_id, user_id) ([]*AssignmentSlotView, error)` that does `SELECT ... FOR UPDATE` on all the user's assignment rows in the publication (plus joining `template_slots` to get the time window).
  - `backend/internal/repository/shift_change.go`: `ApplySwap` and `ApplyGive` call the new helper inside their tx, before the `UPDATE assignments`; they re-evaluate time-conflict against the locked snapshot. They also re-load `users.status` for receiver / both swap participants.
- **Backend service layer**:
  - `backend/internal/service/shift_change.go`: `ensureSwapFitsSchedule` and `ensureGiveFitsSchedule` either remove or simplify into pure-function predicates the repo can call. The pre-tx call site is preserved as a fast-fail (it still rejects 99% of bad requests cheaply); the in-tx call is the correctness floor.
  - `backend/internal/service/publication_pr4.go`: `CreateAssignment` similarly consolidates the overlap check into the new helper, called inside the existing tx. The user-status check moves from "before tx" to "inside tx (re-load)".
- **Backend errors**: no new sentinels; existing `ErrShiftChangeTimeConflict`, `ErrAssignmentTimeConflict`, `ErrUserDisabled` cover all cases.
- **Backend tests**:
  - Service-layer unit tests for the new failure modes: concurrent give→ApplyGive sees the new state and rejects.
  - Integration tests proving the lock-then-check pattern under real concurrent load (run two concurrent inserts via `go test -tags=integration` with goroutines).
- **Spec deltas**: `openspec/specs/scheduling/spec.md` — ~4 modified requirements with new scenarios.
- **Frontend**: add copy for the rare retryable scheduling error.
- **No schema migration**.
- **No new dependency**.
