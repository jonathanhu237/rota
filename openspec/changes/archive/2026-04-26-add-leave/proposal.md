## Why

Once `scheduling-occurrence-model` lands, the `shift_change_requests` table can address a single calendar week of an assignment, and approving a request writes an `assignment_overrides` row instead of permanently transferring the slot. This unlocks the long-pending leave workflow: an employee can ask the system to "find me a sub for next Monday only", with give_direct/give_pool semantics inherited from the existing shift-change machinery.

The motivation is product-driven. Today, an employee who needs to skip a single shift has to either (a) go to admin and have the assignment manually edited, or (b) use the shift-change UI which lacks any concept of "leave" — no record of who was off, when, or why. Manager-level visibility into "who is missing what" requires manual bookkeeping and is error-prone at scale. The fix is to introduce a first-class `leaves` entity that wraps a single occurrence-level shift-change request with leave-specific metadata (category, reason). State of a leave derives from its underlying request; no new state machine.

## What Changes

- New `leaves` table: `(id, user_id, publication_id, shift_change_request_id UNIQUE, category, reason, created_at, updated_at)`. `category` is a hard-coded enum: `{sick, personal, bereavement}`. `reason` is free-form `TEXT` — employees use it to label sub-cases (e.g., "考试", "葬礼具体情况") that admins read when interpreting attendance.
- `shift_change_requests` gains a nullable `leave_id BIGINT` column. `NULL` means a regular shift-change (gated on `PUBLISHED`); non-NULL means a leave (gated on `ACTIVE`). The `Activation bulk-expires` requirement is updated to expire only `leave_id IS NULL` rows; leaves created during `ACTIVE` are inherently safe from this since activation already happened.
- New endpoints, all `RequireAuth`:
  - `POST /leaves` — single-leave creation; body carries `{ assignment_id, occurrence_date, type, counterpart_user_id?, category, reason? }`. Calls into the existing shift-change service for the underlying request, wrapped in one transaction with the leaves insert.
  - `GET /leaves/{id}` — details. Visible to any logged-in user (consistent with how `give_pool` already exposes claim opportunities to "anyone qualified").
  - `POST /leaves/{id}/cancel` — requester cancels; cascades to `cancel` on the underlying request iff still `pending`.
  - `GET /users/me/leaves` — self-history.
  - `GET /users/me/leaves/preview?from=YYYY-MM-DD&to=YYYY-MM-DD` — given a date range, returns the viewer's future occurrences in the current ACTIVE publication, so the UI can offer "select which shifts to request leave for".
  - `GET /publications/{id}/leaves` — admin-only list of every leave in a publication.
- Frontend leave flow:
  - Employee picks a date range → frontend hits the preview endpoint → renders one row per occurrence.
  - Employee picks a `type` (`give_direct` with counterpart, or `give_pool`) per occurrence and a category, then clicks "submit" — frontend issues one `POST /leaves` per row.
  - On success: list of `share_url`s (one per leave) is shown, each is `/leaves/{id}`. Employee shares however they want; counterparts visit and approve through existing shift-change UI.
- The shift-change UI stays unchanged for the regular (PUBLISHED-only) flow. Leave is a separate entry-point.
- No new state machine: `leave.state` is derived from the underlying SCRT — `pending` → `pending`, `approved` → `completed`, `expired`/`rejected` → `failed`, `cancelled`/`invalidated` → `cancelled`.
- New audit actions: `leave.create`, `leave.cancel`. The underlying SCRT events (`shift_change.create`, `shift_change.approve`, etc.) continue to fire as today; leave events are not duplicates — they record leave-specific metadata (category, reason).
- Email notifications: inherited from the SCRT layer — `give_direct` emails the counterpart on creation; `give_pool` does not. Leave-specific emails are out of scope.

## Capabilities

### New Capabilities

None. Leaves live inside the `scheduling` capability — they are a thin product wrapper around shift-change requests, not a new concern.

### Modified Capabilities

- `scheduling`: introduces the `leaves` table and its endpoints, reroutes `shift_change_requests` to carry an optional `leave_id`, and changes the activation-expire rule to skip leave-bearing rows. Adds `leave.*` audit actions to the catalog.

## Non-goals

- **Admin approval of leave.** The product principle is: employees self-serve, admins audit after the fact and adjust attendance manually if abuse is found. No approval flow is built.
- **`affects_attendance` flag, leave quotas, or system-managed attendance computation.** Attendance reasoning happens entirely in the admin's head when reading the leave list. The system records the leave; it does not interpret it.
- **Admin-managed `leave_categories` table.** The three categories (`sick`, `personal`, `bereavement`) are encoded as a database CHECK enum. Adding a category requires a migration. Sub-cases like "考试请假" go into `reason TEXT`.
- **Cross-publication leaves.** A single leave references exactly one occurrence in exactly one publication. By D2 (single non-ENDED publication invariant) there is at most one ACTIVE publication at any given time, so this isn't a meaningful restriction.
- **Bulk submission as a single transaction.** The frontend issues one `POST /leaves` per occurrence and surfaces partial failures inline. Backend stays simple — single-leave create only.
- **Leave-specific emails or push notifications beyond what SCRT already does.**
- **Editing a leave after creation.** Cancel-and-recreate is the only mutation path.

## Impact

- **Backend code**:
  - New migration: `leaves` table + `leave_id` column on `shift_change_requests`.
  - Service layer: new `LeaveService` with `Create`, `GetByID`, `Cancel`, `ListForUser`, `ListForPublication`, `PreviewOccurrences`. `LeaveService.Create` opens the transaction, calls `ShiftChangeService.CreateRequestTx` with `leave_id` set, then inserts the leaves row.
  - `ShiftChangeService.CreateRequest` is split into the public method (PUBLISHED gate, `leave_id` enforced NULL) and the internal `CreateRequestTx` taking an optional `leave_id`. The ACTIVE gate when `leave_id` is non-NULL lives in `LeaveService.Create`.
  - `ShiftChangeService.ActivatePublication` (or wherever the bulk-expire lives) gains a `leave_id IS NULL` clause.
  - Repository layer: new `LeaveRepository`. `ShiftChangeRepository` gains `leave_id` in its insert/select.
  - Handler layer: new `LeaveHandler` for the six endpoints listed in *What Changes*.
  - Audit: register the new `leave.create` and `leave.cancel` actions in the audit capability's action catalog.
- **Frontend code**:
  - New `Leave` page (`/leave`): date range picker → preview → per-occurrence form → submit-and-show-share-URLs.
  - New `LeaveDetail` page (`/leaves/:id`): renders leave metadata + the underlying SCRT, surfacing approve/cancel buttons by viewer role.
  - New "我的请假记录" page reading `/users/me/leaves`.
  - Navigation entry-point in employee menu.
  - No change to the existing shift-change UI.
- **Test impact**:
  - New service tests (happy path + each rejection path per `LeaveService` method).
  - New integration tests for `LeaveRepository` and the cross-table create transaction.
  - New handler tests covering authorization (cancel by non-requester, GET by any logged-in).
  - Existing SCRT tests adapted: `leave_id NULL` becomes the explicit case.
- **No new third-party dependencies.**
- **Audit capability**: small action-catalog update only; no schema change.

## Risks / safeguards

- **Risk:** `LeaveService.Create` straddles two service layers (LeaveService + ShiftChangeService) inside one transaction; it's easy to drift the gates between them. **Mitigation:** the gate split is documented in *design.md* (D-3) and a service-layer test exercises every (publication state × leave_id) combination explicitly.
- **Risk:** the share URL exposes leave detail to any logged-in user; sensitive employees might object. **Mitigation:** the URL exposes only what `give_pool` already exposes (the slot, the requester, the offered shift). Reason text is the only addition, and reason is employee-authored — they decide what to put there. If a user wants reason to stay private, they leave it blank.
- **Risk:** preview endpoint races with admin assignment edits. **Mitigation:** preview is a read; on submit, the SCRT create path re-validates the assignment ownership in transaction. A leave that fails to create due to a stale preview returns the same error code the SCRT layer already emits.
- **Risk:** depends on `scheduling-occurrence-model` (Phase 1) merging first. **Mitigation:** explicit stacked-branches workflow (see updated AGENTS.md). Phase 2 archive is gated on Phase 1 archive landing in main specs.
