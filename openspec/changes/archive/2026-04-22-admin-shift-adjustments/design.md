## Context

Today the publication state machine gates admin assignment mutations to `ASSIGNING` only (see `@openspec/specs/scheduling.md` §State machine and §Access and authorization). Once a publication transitions to `PUBLISHED`, `ACTIVE`, or `ENDED`, calls to `CreateAssignment` / `DeleteAssignment` return `ErrPublicationNotAssigning`. Employees can propose swaps / gives / pool releases during `PUBLISHED`, but during `ACTIVE` they cannot. There is no path left for an admin to correct the schedule after activation, which does not match operational reality.

Two related gaps shape this design:

1. **Forgotten-availability case.** Admins have no way via the UI to assign an employee who did not submit availability. The assignment board's per-shift `candidates` list only contains employees who ticked; anyone qualified-but-silent is invisible.
2. **Stale-request fan-out.** When an admin edits an assignment that is referenced by a pending shift-change request, the system currently relies on the approver-time optimistic lock in `ApplySwap` / `ApplyGive` to detect the mismatch. The request sits in `pending` until someone tries to act on it, polluting the "received" and "pool" lists for every eligible viewer.

Admins today work around these gaps by editing the database directly. Every direct edit bypasses the audit log, so there is no forensic trail for disputes.

## Goals / Non-Goals

**Goals:**

- Admins can add or remove assignments while the publication is in `ASSIGNING`, `PUBLISHED`, or `ACTIVE`.
- Admins can see every qualified employee in the assignment board, not only those who submitted availability.
- Admin deletions immediately invalidate any pending shift-change request that references the deleted assignment, with one email and one audit event per affected request.
- No silent bypass of the audit trail; every admin edit in the widened window continues to go through `audit.Record`.

**Non-Goals:**

- Reintroducing a `PUBLISHED → ASSIGNING` rollback path.
- Giving admins the ability to approve or reject shift-change requests on an employee's behalf.
- Widening `AutoAssignPublication` to run outside `ASSIGNING`.
- Adding any new database column or migration.
- Extending schema validation beyond what already exists.

## Decisions

### Widen the admin-mutable state to `{ASSIGNING, PUBLISHED, ACTIVE}`

The effective-state guard in `CreateAssignment` and `DeleteAssignment` changes from `effective == ASSIGNING` to `effective ∈ {ASSIGNING, PUBLISHED, ACTIVE}`.

```
        effective state         admin CreateAssignment   admin DeleteAssignment
        ───────────────         ──────────────────────   ──────────────────────
        DRAFT                   reject NOT_MUTABLE       reject NOT_MUTABLE
        COLLECTING              reject NOT_MUTABLE       reject NOT_MUTABLE
        ASSIGNING               allow                    allow
        PUBLISHED               allow                    allow + cascade
        ACTIVE                  allow                    allow + cascade
        ENDED                   reject NOT_MUTABLE       reject NOT_MUTABLE
```

Rejected alternative: use a single widened `PUBLICATION_NOT_ASSIGNING` code to cover all non-mutable states. Rejected because the frontend needs to distinguish "auto-assign is not available here" from "the publication is no longer editable" — different user-facing copy, different next actions.

### New error code `PUBLICATION_NOT_MUTABLE` (HTTP 409)

Individual-edit rejections in `DRAFT`, `COLLECTING`, or `ENDED` return `ErrPublicationNotMutable` → `PUBLICATION_NOT_MUTABLE` at HTTP 409. `AutoAssignPublication` keeps its `PUBLICATION_NOT_ASSIGNING` code because its precondition is genuinely narrower.

| Error code                 | HTTP status | Emitted by                                  |
| -------------------------- | ----------- | ------------------------------------------- |
| `PUBLICATION_NOT_MUTABLE`  | 409         | `CreateAssignment`, `DeleteAssignment` outside `{ASSIGNING, PUBLISHED, ACTIVE}` |
| `PUBLICATION_NOT_ASSIGNING`| 409         | `AutoAssignPublication` outside `ASSIGNING` (unchanged) |

### Cascade-invalidate on `DeleteAssignment`, best-effort not transactional

After `repo.DeleteAssignment` returns success, the service runs:

1. `repo.ShiftChange.InvalidateRequestsForAssignment(ctx, assignmentID, now)` — a single `UPDATE ... RETURNING id` that transitions matching pending rows to `invalidated`, returning the IDs.
2. For each returned ID: one `audit.Record` with action `shift_change.invalidate.cascade`, one email to the requester via `BuildShiftChangeResolvedMessage` with outcome `invalidated`.

The cascade is **best-effort**. If the `UPDATE` fails (DB transient error), the admin's delete still succeeds; the approver-time optimistic lock in `ApplySwap` / `ApplyGive` already handles the edge case. If email sending fails, it is logged at `WARN` and the cascade continues.

Rejected alternative: run the delete and cascade inside the same DB transaction. Rejected because the existing `repo.DeleteAssignment` is already a single DDL-free `DELETE`, and bundling a multi-row `UPDATE` into a caller-held transaction forces the service to take a `*sql.Tx` everywhere or to duplicate the delete SQL. The correctness floor (stale request cannot actually apply) is already guaranteed by the optimistic lock; the cascade only improves UX by not showing zombie pending rows.

Rejected alternative: invalidate on `CreateAssignment` as well. Rejected because creating a new assignment does not falsify any existing request — requests always reference a specific `assignment_id`, and creating a new row with a different id leaves those references valid. Only delete and user-replacement touch existing `assignment_id` values.

### Assignment board gains `non_candidate_qualified`

The handler response shape for `GetAssignmentBoard` adds, per shift:

```
"non_candidate_qualified": [{ "user_id", "name", "email" }, ...]
```

Population rule: for each shift with `position_id = P`, `non_candidate_qualified` contains every user whose `user_positions` set includes `P` AND who is NOT in that shift's `candidates` list (i.e., did not submit an availability row). Employees currently `assigned` to the shift are included only if they are also qualified-but-silent, which is the point of the list — it represents "who could still be assigned."

Data comes from a new repo method `ListQualifiedUsersForPositions(ctx, positionIDs)` returning `map[int64][]UserLite`, called once per `GetAssignmentBoard` invocation.

Rejected alternative: frontend toggle that issues a separate API call. Rejected because the board already makes one request per page load; doubling it for an optional toggle is worse than shipping the extra bytes once.

### New audit action and email outcome

- `audit.ActionShiftChangeInvalidateCascade = "shift_change.invalidate.cascade"` in `backend/internal/audit/audit.go`. Metadata: `{ request_id, reason: "assignment_deleted", assignment_id }`.
- `email.ShiftChangeOutcomeInvalidated` added to `backend/internal/email/shift_change.go`, selectable by `BuildShiftChangeResolvedMessage`. Body text: "Your shift change request is no longer applicable because an administrator edited the referenced shift."

### No new dependency, no migration

Every change is a code-level adjustment against existing tables and existing packages. No goose migration. No new Go module. No new npm package.

## Risks / Trade-offs

- **Risk**: an admin mass-deletes assignments during `ACTIVE`, triggering a flood of emails to affected shift-change requesters. **Mitigation**: the existing rate limit on email delivery protects infrastructure; within the application, the cascade is one email per affected request, which aligns with the user's actionable state change. No bulk-dedupe is added here.
- **Risk**: race between admin `DeleteAssignment` and a concurrent `ApproveShiftChangeRequest`. **Mitigation**: the `ApplySwap` / `ApplyGive` optimistic lock already rejects the approval with `ErrShiftChangeAssignmentMiss` → `ErrShiftChangeInvalidated`. If the admin's cascade transitions the row to `invalidated` first and the approver arrives second, the approver's `lockPendingRequest` returns `ErrShiftChangeNotPending`. Both orderings terminate safely with the request in `invalidated` state and no assignment modified.
- **Risk**: employees see rapidly-flipping assignment cards on `/roster` as admins edit during `ACTIVE`. **Mitigation**: existing TanStack Query invalidation triggers refetch; stale-time on the roster query is short. A frontend warning banner on `ACTIVE` assignment board page makes the "changes take effect immediately" contract explicit.
- **Trade-off**: allowing admins to edit during `ACTIVE` means the schedule employees trust can change at any time. Accepted because the alternative (no recovery path) is worse.

## Migration Plan

No data migration. Deployment steps:

1. Deploy backend. New error code is additive; frontend returns generic "INTERNAL_ERROR" handling until the frontend deploys, which is a brief graceful degradation.
2. Deploy frontend. Non-candidate toggle appears; assignment board mutations enable in `PUBLISHED`/`ACTIVE`; `/requests` history surfaces the cascade reason.
3. Rollback: revert frontend first (mutations disabled re-client-side), then revert backend. No data rollback needed.

## Open Questions

None.
