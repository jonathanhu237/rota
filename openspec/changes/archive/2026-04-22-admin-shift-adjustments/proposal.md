## Why

Today, once a publication leaves the `ASSIGNING` state, administrators lose all ability to adjust assignments through the application — `CreateAssignment` and `DeleteAssignment` return `PUBLICATION_NOT_ASSIGNING`. Real operational needs routinely arrive after that window closes: an employee who forgot to submit availability still needs to be scheduled, an employee who falls ill during `ACTIVE` needs to be replaced, an employee who resigns needs to be swapped out. Employees can initiate swap / give / pool requests during `PUBLISHED`, but during `ACTIVE` even that escape hatch is shut — leaving no recovery path. The admin's only current workaround is to edit the database directly, which bypasses the audit log and the optimistic-lock integrity checks the system relies on.

This change restores the admin's authority to correct the schedule at any time before the publication ends, while keeping the rest of the system's guarantees (audit trail, shift-change optimistic locking, qualification rules) intact.

## What Changes

- Widen the mutable window for admin assignment edits from `ASSIGNING` only to `{ASSIGNING, PUBLISHED, ACTIVE}`. `AutoAssignPublication` stays `ASSIGNING`-only because it replaces the entire assignment set.
- Introduce a new domain error `ErrPublicationNotMutable` (HTTP 409 `PUBLICATION_NOT_MUTABLE`) for individual assignment edits rejected in `DRAFT`, `COLLECTING`, or `ENDED`. `PUBLICATION_NOT_ASSIGNING` stays in place for the auto-assign path.
- When an admin deletes an assignment, cascade-invalidate any pending shift-change request that references the deleted assignment (as requester's side or swap counterpart's side). Each affected request transitions `pending → invalidated`, emits one audit event per request, and triggers one email to the original requester.
- Extend the assignment board response with a per-shift `non_candidate_qualified` list: employees qualified for the position who did not submit availability. Admins can toggle visibility on the frontend.
- New audit action `shift_change.invalidate.cascade`.
- New email outcome `invalidated` reusing the existing shift-change resolved template.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `scheduling`: widens the admin-mutable state window for assignment edits, adds `non_candidate_qualified` to the assignment board contract, and formalizes cascade-invalidation of shift-change requests on admin assignment deletions.
- `audit`: adds `shift_change.invalidate.cascade` to the action taxonomy.

## Non-goals

The following are deliberately excluded from this change:

- **Unpublish path (`PUBLISHED → ASSIGNING` rollback)** — admins who want to undo a publish step must end the publication and create a new one. A true reversal would require re-syncing the whole shift-change request pool and is not justified by demand.
- **Time-off / leave request workflow** — admins can reassign, but the formal "I am unavailable from X to Y" flow is a separate, larger feature tracked independently.
- **Dashboard of stuck assignments from disabled users** — when a user is disabled, their assignments stay in place and admins see them via the normal assignment board. A dedicated "orphan assignments" view is not built here.
- **Admin-approves-on-behalf-of-employee for shift-change requests** — admins continue to manipulate the underlying assignments directly; they do not step into an employee's pending request.
- **Auto-assign during `PUBLISHED` or `ACTIVE`** — intentionally unchanged. Auto-assign's contract is "start from scratch and optimize," which is incompatible with an already-communicated schedule.

## Impact

- **Backend service layer**: `publication_pr4.go` (state-guard widening, cascade-invalidation on delete), `publication_pr4.go` `GetAssignmentBoard` (attaches `non_candidate_qualified`).
- **Backend repository layer**: new helper `ListQualifiedUsersForPositions` (or similar); new helper on shift-change repo to mark pending requests as invalidated given an `assignment_id`.
- **Backend error taxonomy**: new sentinel `ErrPublicationNotMutable` and HTTP mapping `PUBLICATION_NOT_MUTABLE` at 409.
- **Backend audit**: new action constant `ActionShiftChangeInvalidateCascade`.
- **Backend email**: new outcome value `invalidated` in the shift-change resolved template.
- **Backend handler**: `assignmentBoardShiftResponse` gains `non_candidate_qualified`.
- **Frontend**: assignment board mutation predicate widens to `PUBLISHED` and `ACTIVE`, adds a "Show all qualified employees" toggle, adds state-specific warning banners, and surfaces the cascade reason on invalidated shift-change cards in `/requests` history.
- **Frontend i18n**: new error code, toggle copy, banner copy, cascade-reason copy in `en` and `zh`.
- **No migration**. Schema is unchanged.
