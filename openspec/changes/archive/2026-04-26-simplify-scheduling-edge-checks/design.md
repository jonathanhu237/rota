## Context

`tighten-scheduling-edges` (commit `05336ab`) added an in-tx time-conflict check inside a new repository helper `LockAndCheckUserSchedule`, plus pre-tx fast-fail checks in the service layer (`ensureSwapFitsSchedule`, `ensureGiveFitsSchedule`, `hasAssignmentTimeConflict`). These were intended to defend against concurrent shift-change applies (or admin `CreateAssignment` operations) that race past the existing optimistic locks and produce time-overlapping assignments on the same user.

After CI exposed broken setup in the new concurrent integration tests, we re-examined the data model and noticed that the `template_slots_no_overlap_excl` GIST exclusion constraint shipped earlier (`refactor-to-slot-position-model`) **already prevents the precondition for the race**. Two slots in the same `(template_id, weekday)` cannot overlap in time, so two assignments in the same publication referencing those slots cannot overlap either. There is no concurrent path that produces an invalid weekly schedule under the GIST.

The new conflict-check code is therefore dead code defending against an impossible scenario. The tests for that scenario fail at setup because the GIST refuses to materialize the precondition. Two spec scenarios describe situations the data model rules out.

The S2 case (in-tx `users.status` re-check on apply / create) remains real and necessary — `users.status` is mutated independently of `template_slots` and has no equivalent guard at the database level.

## Goals / Non-Goals

**Goals:**

- Remove the dead time-conflict code added by `tighten-scheduling-edges`.
- Remove the unreachable spec scenarios.
- Remove the broken tests that target the unreachable scenarios.
- Preserve the still-useful parts of the helper (the `users.status` FOR UPDATE check).
- Preserve A3 (qualification filter at read) and `SCHEDULING_RETRYABLE` deadlock mapping.

**Non-Goals:**

- Touching the GIST exclusion constraint.
- Touching A3 or auto-assign-related logic.
- Removing `SCHEDULING_RETRYABLE`.
- Re-introducing checks at any other layer ("just in case GIST is removed someday"). Defense-in-depth without a real threat is just dead code that grows over time.

## Decisions

### The GIST exclusion is the canonical enforcement

The `template_slots_no_overlap_excl` constraint is `EXCLUDE USING gist (template_id WITH =, weekday WITH =, tsrange(start_time, end_time, '[)') WITH &&)`. It rejects any insertion of a row in `template_slots` that overlaps an existing row of the same `(template_id, weekday)`.

Every assignment row references one slot, and a publication's assignments are all scoped to one template (the publication's locked template). Therefore two assignments in the same publication referencing two slots — both slots are in the same template — and those two slots cannot have overlapping `(weekday, time)` by the GIST. So no user can ever hold two same-weekday time-overlapping assignments in the same publication.

This is **stronger** than the service-layer conflict check could ever be: the GIST is enforced atomically by the database on every `template_slots` insert, including admin slot creation, template cloning, and any future code path that writes the table. It cannot be bypassed by application bugs, missing call sites, or concurrent transactions.

**Rejected**: keep the conflict checks "for defense in depth." Rejected because (a) the GIST cannot be bypassed without explicit removal; (b) maintaining dead code rots — the next reader will be unsure why the check exists and may invest time confirming it's still needed; (c) the cost of bringing back the check, if GIST is ever weakened, is small (the helper signature change is a one-commit revert).

### Helper simplifies to status-only

Before:

```go
func LockAndCheckUserSchedule(
    ctx context.Context,
    tx *sql.Tx,
    publicationID, userID int64,
    additions []SlotTimeWindow,
    excludeAssignmentIDs []int64,
) error {
    // 1. advisory lock on (publication, user)
    // 2. SELECT ... FOR UPDATE assignments → existing slot-time set
    // 3. filter excludes, append additions
    // 4. run overlap predicate → ErrTimeConflict on overlap
    // 5. SELECT status FROM users WHERE id = $userID FOR UPDATE
    // 6. ErrUserDisabled if status != active
    return nil
}
```

After:

```go
func LockAndCheckUserStatus(
    ctx context.Context,
    tx *sql.Tx,
    publicationID, userID int64,
) error {
    // 1. advisory lock on (publication, user)
    // 2. SELECT status FROM users WHERE id = $userID FOR UPDATE
    // 3. ErrUserDisabled if status != active
    return nil
}
```

The advisory lock stays — it serializes apply transactions per-user, which is mildly useful for the `users.status` check semantics and costs almost nothing.

The assignment row-level lock goes away — it was only there to cover the conflict check.

The `SlotTimeWindow` type is kept if any other code uses it; otherwise removed. We'll discover during apply.

`ErrTimeConflict` repository sentinel is removed; no caller will produce or consume it after this change. Service-layer aliases (`ErrShiftChangeTimeConflict`, `ErrAssignmentTimeConflict`) remain because they're still used for OTHER reasons:

- Wait — are they? They were used as the *response* of the conflict check. With the check gone, what produces these sentinels? Nothing.

OK so service-layer `ErrShiftChangeTimeConflict` and `ErrAssignmentTimeConflict` are also dead. Their handler mappings (`SHIFT_CHANGE_TIME_CONFLICT`, `ASSIGNMENT_TIME_CONFLICT`) become unused. Frontend i18n entries for those codes remain (harmless if never triggered).

**Rejected**: keep the service-layer sentinels and handler mappings "for defense in depth." Same rejection rationale as above. Dead code rots.

**Rejected**: also remove the frontend i18n entries. Out of scope for this backend cleanup. They are inert and can be cleaned up by any future i18n audit.

### Spec re-cast: assert the GIST guarantee, not a service check

The two affected requirements are reframed:

- `Time-conflict check before applying` → renamed to `No same-user time-conflict in a publication` (or kept name, body re-written). Body: "By the `template_slots_no_overlap_excl` GIST constraint, two same-weekday slots in a template cannot have overlapping time ranges. Therefore two assignments in the same publication, both referencing slots in the same template, cannot overlap. The shift-change apply paths SHALL rely on this database guarantee and SHALL NOT add a redundant service-layer time-conflict check." Scenarios kept: "Overlap with existing weekly assignment" — but rewritten to assert the GIST rejects the offending slot's *creation*, not the apply.

- `Admin CreateAssignment rejects same-weekday slot overlap` → similar reframing.

Effectively both requirements become assertions that the GIST is the enforcement point. This is a real change in spec semantics: we drop the promise of a service-layer check. The contract still holds — admin `CreateAssignment` of an "overlapping assignment" cannot succeed — but the failure mode changes: admin fails at *defining* a slot that overlaps another, not at *assigning* a user to a pre-existing overlapping slot (which can't exist).

**Rejected**: keep service-layer wording for "future-proofing." Rejected; spec should describe what the system does, not what we might want it to do under hypothetical future schemas.

### Tests that survive

Kept:
- `TestApplyGiveDisabledReceiverMidFlight` (S2 race for give)
- `TestApplySwapDisabledCounterpartMidFlight` (S2 race for swap)
- `TestCreateAssignmentDisabledMidFlight` (S2 race for admin create)
- The repository unit test for the helper is renamed and trimmed to status-only assertions.

Removed:
- `TestConcurrentApplyGive` — unreachable scenario, drop.
- `TestConcurrentApplySwap` — unreachable scenario, drop.
- `TestConcurrentCreateAssignment` — unreachable scenario, drop.

## Risks / Trade-offs

- **Risk**: We are committing to "the GIST is correct and won't be weakened." If a future change drops the GIST (e.g., to support templates with deliberately-overlapping slot definitions), the time-conflict race comes back. Mitigation: that future change would have to add a spec note "we now allow overlapping slots" and would naturally reintroduce conflict checks at the apply layer. The cost is paid by the change that breaks the assumption, which is the right ownership.
- **Risk**: We're now relying on database constraints for invariants that some shops prefer to enforce at the application layer. Trade-off: this codebase already relies on DB-level constraints elsewhere (the `assignments_position_belongs_to_slot` trigger, FK cascades) and treats them as sources of truth. Consistent with the project's "let the DB enforce what only the DB can enforce atomically" stance.
- **Risk**: Frontend keeps `SHIFT_CHANGE_TIME_CONFLICT` / `ASSIGNMENT_TIME_CONFLICT` in `api-error.ts` and i18n. These codes will never fire after this change. Trade-off: cleaning up frontend i18n / types in the same PR widens scope; we accept the harmless stale entries for now. A follow-up i18n audit can sweep them.

## Migration Plan

No data migration. Deployment:

1. Deploy backend (helper renamed and trimmed, dead service-layer code removed). Existing apply calls continue to work; the helper does less now but surfaces the same set of errors that callers already handle.
2. Frontend: zero change.
3. Rollback: revert the deployment; the previous behavior (with redundant checks) returns. No data loss.

## Open Questions

None.
