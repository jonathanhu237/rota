## Why

The just-shipped `tighten-scheduling-edges` (commit `05336ab`) closed four spec-vs-code drift cases. On post-merge CI we discovered a deeper insight that invalidates two of those four cases:

**Two of the four "races" we built defenses against are unreachable under the `template_slots_no_overlap_excl` GIST exclusion constraint shipped in `refactor-to-slot-position-model`.** The GIST forbids any two slots in the same `(template_id, weekday)` from overlapping in time. Because every assignment in a publication references a slot in that publication's locked template, the spec invariant "no user holds two time-overlapping assignments in the same publication" is **already enforced at the database layer** at slot-definition time. There is no concurrent shift-change apply or concurrent admin `CreateAssignment` that can produce overlapping assignments — the slots required to even attempt this configuration cannot coexist in `template_slots`.

Concretely:

- The 3 concurrent integration tests added by `tighten-scheduling-edges` (`TestConcurrentApplyGive`, `TestConcurrentApplySwap`, `TestConcurrentCreateAssignment`) all attempt to seed two overlapping slots in one template as setup. The GIST rejects the second seed, and the tests fail at setup. They cannot be re-written to exercise the race they describe — the race is a phantom.
- The new in-tx time-conflict check inside `LockAndCheckUserSchedule` (the loop over `additions ∪ existing` running the overlap predicate) is dead code: it cannot return `ErrTimeConflict` against a database state that GIST allows.
- The pre-tx `ensureSwapFitsSchedule`, `ensureGiveFitsSchedule`, and admin `hasAssignmentTimeConflict` are by the same reasoning redundant — they check for a conflict that the data model forbids by construction.
- Two spec scenarios ("Concurrent shift-change apply cannot bypass the conflict check", "Concurrent shift-change cannot defeat admin's overlap check") describe situations the data model rules out.

The other two `tighten-scheduling-edges` cases — A3 (auto-assign filters by current `user_positions` and `users.status`) and S2 (in-tx `users.status` FOR UPDATE check on apply) — remain valid and necessary; they are not affected by the GIST observation.

This change removes the dead time-conflict code, removes the unreachable spec scenarios, removes the broken tests, and keeps everything that's actually doing work. It is the cleanup that should have been part of `tighten-scheduling-edges` had we caught the GIST interaction before merging. CI is green after this change because the impossible tests are gone, not stubbed.

## What Changes

- **Simplify `LockAndCheckUserSchedule`** in `backend/internal/repository/assignment.go`:
  - Remove the `additions []SlotTimeWindow` and `excludeAssignmentIDs []int64` parameters.
  - Remove the `SELECT ... FOR UPDATE OF a` over assignments and the subsequent overlap-predicate loop.
  - Keep the `SELECT status FROM users WHERE id = $userID FOR UPDATE` and the per-(publication, user) advisory lock (the latter still serves the `users.status` check serialization purpose).
  - Rename to `LockAndCheckUserStatus(ctx, tx, publicationID, userID)` to reflect what's left.
  - The repository sentinel `ErrTimeConflict` is removed (no caller returns it any more).
- **Remove pre-tx time-conflict checks** in service layer:
  - `ensureSwapFitsSchedule` / `ensureGiveFitsSchedule` in `backend/internal/service/shift_change.go` are deleted.
  - `hasAssignmentTimeConflict` and the call site in `backend/internal/service/publication_pr4.go` `CreateAssignment` are deleted.
  - The repository helper `ListUserAssignmentsOnWeekdayInPublication` (used only by the deleted pre-tx check) is removed.
- **Update the three call sites** to call the simplified helper:
  - `ApplyGive` and `ApplySwap` in `backend/internal/repository/shift_change.go` keep calling for the status check; drop the conflict-related arguments.
  - `CreateAssignment` in `backend/internal/service/publication_pr4.go` keeps calling for the status check; drops the conflict-related arguments.
- **Remove the 3 broken integration tests** in `backend/internal/service/scheduling_edges_integration_test.go`:
  - `TestConcurrentApplyGive`, `TestConcurrentApplySwap`, `TestConcurrentCreateAssignment` are deleted (the scenarios they describe are unreachable).
  - The user-status race tests in the same file (e.g., `TestApplyGiveDisabledReceiverMidFlight`) are kept and continue to exercise the surviving helper.
- **Remove unreachable spec scenarios** in `openspec/specs/scheduling/spec.md`:
  - From `Time-conflict check before applying`: remove the in-tx-strengthening clause and the "Concurrent shift-change apply cannot bypass" scenario; revert the requirement body to its pre-`tighten-scheduling-edges` shape (single pre-tx check).
  - From `Admin CreateAssignment rejects same-weekday slot overlap`: remove the in-tx-strengthening clause and the "Concurrent shift-change cannot defeat admin's overlap check" scenario.
  - Wait — the pre-tx check itself is also being removed. Both requirements become: "the database GIST exclusion guarantees this; no service-layer check is required." Re-cast as cross-references to the GIST requirement rather than asserting service-layer behavior that doesn't exist.
- **Keep**: `Reject assignment of disabled users` (S2) requirement and its scenarios, the `Auto-assign skips submissions whose qualification was revoked / from disabled users` scenarios on `Auto-assign...MCMF`, the `SCHEDULING_RETRYABLE` 503 mapping for Postgres deadlock retries (still possible from the `users` row FOR UPDATE).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `scheduling`:
  - `Time-conflict check before applying` — replace with a requirement that asserts the GIST exclusion on `template_slots` makes overlapping assignments structurally impossible; no service-layer time-conflict check is required.
  - `Admin CreateAssignment rejects same-weekday slot overlap` — same reframing: the GIST exclusion is the enforcement; service-layer rejection happens at slot creation, not at assignment creation.
  - `Reject assignment of disabled users` — unchanged in body, scenarios preserved.

## Non-goals

- **Changing the GIST exclusion**, removing it, or weakening it. The constraint is the guarantee this change relies on.
- **Reverting A3** (the auto-assign qualification filter). A3 is independent of GIST and still needed.
- **Reverting `SCHEDULING_RETRYABLE`** error code mapping. The `users` FOR UPDATE check can still produce deadlock under unusual concurrency, and the retry signaling is genuinely useful.
- **Adding new functionality**. This is purely a removal / simplification.
- **Reverting the `tighten-scheduling-edges` archive directory**. Its proposal/design/specs/tasks remain as historical record.

## Impact

- **Backend repository layer**:
  - `backend/internal/repository/assignment.go`: helper renamed and simplified; `ErrTimeConflict` sentinel removed; `ListUserAssignmentsOnWeekdayInPublication` removed; `SlotTimeWindow` removed (or trimmed if used elsewhere).
- **Backend service layer**:
  - `backend/internal/service/shift_change.go`: `ensureSwapFitsSchedule`, `ensureGiveFitsSchedule` deleted. Apply-Approve service paths simplified.
  - `backend/internal/service/publication_pr4.go`: `hasAssignmentTimeConflict` and its call site in `CreateAssignment` deleted. Helper call updated.
- **Backend tests**:
  - 3 broken concurrent tests deleted from `scheduling_edges_integration_test.go`.
  - Existing user-status race tests kept and adapted to the simpler helper signature.
- **Frontend**: zero change.
- **Specs**: `openspec/specs/scheduling/spec.md` — 2 requirements modified (re-cast around GIST), 0 added, 0 removed.
- **No schema migration**.
- **No new dependency**.
