# Scheduling Capability

## Purpose

Templates, publications, availability, assignments, shift changes, and the weekly roster — the "produce a weekly rota" half of Rota. Related specs: `auth` covers sessions, admin/employee roles, and invitation and password-reset flows. `audit` covers the append-only audit log that every mutating action in this subsystem writes to.

## Requirements

### Requirement: Qualification management

The system SHALL maintain a many-to-many link between users and positions in `user_positions`. Administrators SHALL be able to list a user's qualifications via `GET /users/{id}/positions` and replace them atomically via `PUT /users/{id}/positions`. Both endpoints require `RequireAdmin`.

#### Scenario: Admin replaces a user's qualifications

- **GIVEN** an authenticated admin
- **WHEN** the admin issues `PUT /users/{id}/positions` with a new list of position IDs
- **THEN** the user's qualification set is replaced atomically with exactly that list

#### Scenario: Non-admin cannot manage qualifications

- **WHEN** an authenticated employee issues `GET /users/{id}/positions` or `PUT /users/{id}/positions`
- **THEN** the request is rejected by the `RequireAdmin` middleware

### Requirement: Qualification gates employee actions

An employee SHALL only be permitted to submit availability, create a shift-change request, accept a direct give, approve a swap, or claim a pool request for a shift whose `position_id` is in their `user_positions` set. Admins bypass this check when creating assignments directly.

#### Scenario: Employee submits availability for an unqualified position

- **WHEN** an employee submits availability for a `template_shift` whose `position_id` is not in their `user_positions`
- **THEN** the response is HTTP 403 with error code `NOT_QUALIFIED`

#### Scenario: Admin assigns regardless of qualification check path

- **WHEN** an admin creates an assignment for a user/shift pair
- **THEN** the qualification check is enforced against the target user's `user_positions`, and the admin's own qualifications are irrelevant

### Requirement: Template and shift data model

`templates` rows SHALL store `id`, `name`, `description`, `is_locked`, `created_at`, `updated_at`. `template_shifts` rows SHALL store `id`, `template_id`, `weekday`, `start_time`, `end_time`, `position_id`, `required_headcount`, `created_at`, `updated_at`. Database CHECK constraints SHALL enforce `weekday BETWEEN 1 AND 7` (Monday=1 through Sunday=7), `end_time > start_time`, and `required_headcount > 0`. `template_shifts` SHALL be indexed on `(template_id, weekday, start_time)` to back the canonical week-grid sort order. Deleting a `position` that is referenced by any `template_shift` SHALL be blocked by `ON DELETE RESTRICT`.

Template `name` SHALL be trimmed and limited to 100 code points; `description` to 500. Shift times SHALL be stored as `TIME` and serialized over the wire as `HH:MM`. Shifts SHALL NOT cross midnight; a span over midnight is represented by two rows.

#### Scenario: Invalid weekday is rejected at the database

- **WHEN** an insert of a `template_shift` sets `weekday = 0` or `weekday = 8`
- **THEN** the database CHECK rejects the row

#### Scenario: Overlong name is trimmed to the limit

- **WHEN** an admin creates a template with a name longer than 100 code points
- **THEN** the request is rejected with `INVALID_REQUEST`

#### Scenario: Position referenced by a shift cannot be deleted

- **GIVEN** a `template_shift` that references `position_id = P`
- **WHEN** an admin attempts to delete position `P`
- **THEN** the delete is blocked by the `ON DELETE RESTRICT` foreign key

### Requirement: Template CRUD and shift CRUD

Administrators SHALL be able to list, create, get, update, and delete templates (`GET /templates`, `POST /templates`, `GET /templates/{id}`, `PUT /templates/{id}`, `DELETE /templates/{id}`) and create, update, and delete a template's shifts (`POST /templates/{id}/shifts`, `PATCH /templates/{id}/shifts/{shift_id}`, `DELETE /templates/{id}/shifts/{shift_id}`). All of these endpoints SHALL require `RequireAdmin`. `GET /templates` SHALL be paginated. `GET /templates/{id}` SHALL include the template's shifts.

#### Scenario: Admin lists templates

- **WHEN** an admin calls `GET /templates`
- **THEN** a paginated list of templates is returned

#### Scenario: Non-admin cannot access template endpoints

- **WHEN** an employee calls any `/templates*` endpoint
- **THEN** the request is rejected by `RequireAdmin`

### Requirement: Template locking

A template with `is_locked = true` SHALL be immutable: the server SHALL reject template update, template delete, and any shift create, update, or delete on that template. Locking SHALL happen atomically on first publication reference, regardless of that publication's state (including `DRAFT`). A locked template MAY be referenced by additional publications.

#### Scenario: First publication reference locks the template

- **GIVEN** an unlocked template
- **WHEN** a publication that references the template is created (even in `DRAFT`)
- **THEN** the template's `is_locked` is set to `true` atomically as part of the same operation

#### Scenario: Update of a locked template is refused

- **GIVEN** a template with `is_locked = true`
- **WHEN** an admin calls `PUT /templates/{id}`, `DELETE /templates/{id}`, or any of the `…/shifts…` mutating endpoints
- **THEN** the request is refused

#### Scenario: Additional publications may reference an already-locked template

- **GIVEN** a locked template
- **WHEN** an admin creates another publication that references the same template
- **THEN** the creation is permitted (subject to the single-non-ENDED invariant)

### Requirement: Template cloning

`POST /templates/{id}/clone` SHALL create a new, unlocked template whose name is `<original> (copy)`, truncated to fit 100 code points. The new template SHALL be an independent copy with its own shifts.

#### Scenario: Cloning produces an unlocked template

- **WHEN** an admin clones a locked template named `Weekday Rota`
- **THEN** a new template named `Weekday Rota (copy)` is created with `is_locked = false`

#### Scenario: Name truncation on clone

- **GIVEN** a template whose name is long enough that appending ` (copy)` would exceed 100 code points
- **WHEN** the admin clones it
- **THEN** the resulting name is truncated to fit within 100 code points while ending in `(copy)`

### Requirement: Publication data model and window invariant

`publications` rows SHALL store `id`, `template_id`, `name`, `state`, `submission_start_at`, `submission_end_at`, `planned_active_from`, `activated_at` (nullable), `ended_at` (nullable), `created_at`, `updated_at`. A database CHECK SHALL enforce `state ∈ { DRAFT, COLLECTING, ASSIGNING, PUBLISHED, ACTIVE, ENDED }`. A database CHECK SHALL enforce `submission_start_at < submission_end_at <= planned_active_from`. `template_id` SHALL use `ON DELETE RESTRICT`.

#### Scenario: Invalid window rejected by CHECK

- **WHEN** a publication row is written with `submission_start_at >= submission_end_at` or with `submission_end_at > planned_active_from`
- **THEN** the database CHECK rejects the row
- **AND** the handler maps the failure to HTTP 400 with error code `INVALID_PUBLICATION_WINDOW`

#### Scenario: Template with publications cannot be deleted

- **GIVEN** a template referenced by at least one publication (in any state)
- **WHEN** an admin attempts to delete the template
- **THEN** the delete is blocked by the `ON DELETE RESTRICT` foreign key

### Requirement: Single non-ENDED publication invariant (D2)

At most one publication row SHALL have `state != 'ENDED'` at any time. This SHALL be enforced both in the service layer and by a partial unique index `WHERE state != 'ENDED'`. A create request that would violate this invariant SHALL be rejected with HTTP 409 and error code `PUBLICATION_ALREADY_EXISTS`.

#### Scenario: Second non-ENDED publication is rejected

- **GIVEN** an existing publication whose state is not `ENDED`
- **WHEN** an admin calls `POST /publications` to create another
- **THEN** the response is HTTP 409 with error code `PUBLICATION_ALREADY_EXISTS`

#### Scenario: New publication permitted after ending the previous one

- **GIVEN** the only existing publication has just transitioned to `ENDED`
- **WHEN** an admin calls `POST /publications`
- **THEN** the creation succeeds

### Requirement: Publication creation locks the referenced template

Creating a publication SHALL atomically set the referenced template's `is_locked = true` if it was not already locked. The locking SHALL happen in the same transaction that inserts the publication row.

#### Scenario: Creation locks a previously unlocked template

- **GIVEN** an unlocked template `T`
- **WHEN** an admin creates a publication referencing `T`
- **THEN** both the publication row is inserted and `T.is_locked` becomes `true` atomically

### Requirement: Publication deletion is DRAFT-only

A publication SHALL be deletable only while its effective state is `DRAFT`. Delete requests in any other state SHALL be rejected with HTTP 409 and error code `PUBLICATION_NOT_DELETABLE`.

#### Scenario: Delete in DRAFT succeeds

- **WHEN** an admin calls `DELETE /publications/{id}` for a publication whose effective state is `DRAFT`
- **THEN** the publication is deleted

#### Scenario: Delete outside DRAFT is refused

- **WHEN** an admin calls `DELETE /publications/{id}` for a publication whose effective state is `COLLECTING`, `ASSIGNING`, `PUBLISHED`, `ACTIVE`, or `ENDED`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_DELETABLE`

### Requirement: Publication state transitions

The state machine SHALL be `DRAFT → COLLECTING → ASSIGNING → PUBLISHED → ACTIVE → ENDED`. Transitions from `DRAFT → COLLECTING` and `COLLECTING → ASSIGNING` SHALL be time-driven (effective-state resolution). Transitions from `ASSIGNING → PUBLISHED`, `PUBLISHED → ACTIVE`, and `ACTIVE → ENDED` SHALL be manual admin actions via `POST /publications/{id}/publish`, `POST /publications/{id}/activate`, and `POST /publications/{id}/end` respectively.

The manual transitions SHALL be implemented as single-row conditional `UPDATE`s; `sql.ErrNoRows` SHALL be folded into a domain "not in expected state" error so concurrent clicks never double-transition.

#### Scenario: Publish succeeds from ASSIGNING

- **GIVEN** a publication whose effective state is `ASSIGNING`
- **WHEN** an admin calls `POST /publications/{id}/publish`
- **THEN** the stored state becomes `PUBLISHED`

#### Scenario: Publish outside ASSIGNING is rejected

- **WHEN** an admin calls `POST /publications/{id}/publish` while the effective state is not `ASSIGNING`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_ASSIGNING`

#### Scenario: Activate outside PUBLISHED is rejected

- **WHEN** an admin calls `POST /publications/{id}/activate` while the effective state is not `PUBLISHED`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_PUBLISHED`

#### Scenario: End outside ACTIVE is rejected

- **WHEN** an admin calls `POST /publications/{id}/end` while the effective state is not `ACTIVE`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_ACTIVE`

#### Scenario: Concurrent manual transitions do not double-apply

- **GIVEN** a publication in `ASSIGNING`
- **WHEN** two admins concurrently click publish
- **THEN** exactly one conditional `UPDATE` affects a row and the other is rejected as "not in expected state"

### Requirement: Effective state resolution on read

Effective state SHALL be computed on every publication read according to: if `pub.state ∈ (PUBLISHED, ACTIVE, ENDED)` then `pub.state`; else if `now >= pub.submission_end_at` then `ASSIGNING`; else if `now >= pub.submission_start_at` then `COLLECTING`; else `DRAFT`. No background job SHALL advance the stored state; stored state SHALL be advanced only when a state-gated write arrives that carries a lazy write-through.

#### Scenario: DRAFT is observed as COLLECTING after submission_start_at

- **GIVEN** a stored state of `DRAFT` and `NOW() >= submission_start_at < submission_end_at`
- **WHEN** any reader resolves effective state
- **THEN** the effective state is `COLLECTING` while the stored state remains `DRAFT` until a submission write-through occurs

#### Scenario: COLLECTING is observed as ASSIGNING after submission_end_at

- **GIVEN** `NOW() >= submission_end_at` and a stored state of `DRAFT` or `COLLECTING`
- **WHEN** any reader resolves effective state
- **THEN** the effective state is `ASSIGNING`

#### Scenario: Terminal-ish stored states override the clock

- **GIVEN** a stored state of `PUBLISHED`, `ACTIVE`, or `ENDED`
- **WHEN** a reader resolves effective state
- **THEN** the effective state equals the stored state regardless of the current clock

### Requirement: Lazy stored-state write-through on submission

`UpsertSubmission` and `DeleteSubmission` SHALL accept a caller-supplied `PublicationState` override and SHALL set the publication's stored `state = 'COLLECTING'` in the same transaction as the submission write when the stored state was still `DRAFT`.

#### Scenario: First submission during the window advances stored state

- **GIVEN** a publication whose stored state is `DRAFT` but whose effective state is `COLLECTING`
- **WHEN** an employee submits availability
- **THEN** the submission is persisted and the publication's stored state becomes `COLLECTING` in the same transaction

### Requirement: Activation bulk-expires pending shift-change requests

Activating a publication SHALL, inside the same transaction that transitions the publication from `PUBLISHED` to `ACTIVE`, perform `UPDATE shift_change_requests SET state='expired' WHERE publication_id = $1 AND state='pending'`.

#### Scenario: Pending requests are expired atomically on activate

- **GIVEN** a `PUBLISHED` publication with two `pending` shift-change requests
- **WHEN** an admin calls `POST /publications/{id}/activate`
- **THEN** the publication's state becomes `ACTIVE` and both requests have state `expired` as a result of the same transaction

### Requirement: Availability submission data model

`availability_submissions` rows SHALL store `id`, `publication_id`, `user_id`, `template_shift_id`, `created_at`. There SHALL be a unique constraint on `(publication_id, user_id, template_shift_id)`. Rows SHALL be `ON DELETE CASCADE` from publication, user, and template_shift.

#### Scenario: Duplicate tick is idempotent at the database

- **GIVEN** an existing `availability_submissions` row for `(pub, user, shift)`
- **WHEN** another insert is attempted for the same tuple
- **THEN** the database's unique constraint rejects it

### Requirement: Availability window

The system SHALL permit creation and deletion of `availability_submissions` only when the publication's *effective* state is `COLLECTING`. Writes outside that window SHALL be rejected with HTTP 409 and error code `PUBLICATION_NOT_COLLECTING`.

#### Scenario: Tick during COLLECTING is accepted

- **GIVEN** a publication whose effective state is `COLLECTING`
- **WHEN** a qualified employee calls `POST /publications/{id}/submissions` for one of their qualified shifts
- **THEN** the submission is persisted

#### Scenario: Tick outside COLLECTING is refused

- **WHEN** an employee calls `POST /publications/{id}/submissions` or `DELETE /publications/{id}/submissions/{shift_id}` while the effective state is not `COLLECTING`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_COLLECTING`

### Requirement: Employee availability endpoints

The system SHALL expose the following employee-facing endpoints, each requiring `RequireAuth`: `GET /publications/{id}/shifts/me` (shifts the viewer is qualified for; gated on effective state `COLLECTING`), `GET /publications/{id}/submissions/me` (viewer's own ticked shift IDs), `POST /publications/{id}/submissions` (tick; gated on `COLLECTING`), and `DELETE /publications/{id}/submissions/{shift_id}` (un-tick; gated on `COLLECTING`).

`GET /publications/{id}/shifts/me` SHALL only return `template_shifts` whose `position_id` is in the viewer's `user_positions`.

#### Scenario: shifts/me filters by qualification

- **GIVEN** a template with shifts in positions `P1` and `P2`, and a viewer qualified only for `P1`
- **WHEN** the viewer calls `GET /publications/{id}/shifts/me` during `COLLECTING`
- **THEN** the response contains only shifts whose `position_id = P1`

### Requirement: Assignment data model

`assignments` rows SHALL store `id`, `publication_id`, `user_id`, `template_shift_id`, `created_at`. There SHALL be a unique constraint on `(publication_id, user_id, template_shift_id)`. Rows SHALL be `ON DELETE CASCADE` from publication, user, and template_shift.

The number of assignments for a given `(publication_id, template_shift_id)` SHOULD equal the shift's `required_headcount` but SHALL NOT be hard-enforced: understaffed shifts are permitted.

#### Scenario: Understaffed shifts are permitted

- **GIVEN** a shift with `required_headcount = 3`
- **WHEN** only two assignments exist for that shift in a publication
- **THEN** the publication may still transition to `PUBLISHED` and `ACTIVE` without server-side rejection

### Requirement: Assignment window

Creating or deleting an assignment and running auto-assign SHALL require the publication's effective state to be `ASSIGNING`. The assignment-board read SHALL additionally accept effective state `ACTIVE` so admins can see who works what once the publication is live.

#### Scenario: Assignment write outside ASSIGNING is rejected

- **WHEN** an admin calls `POST /publications/{id}/assignments`, `DELETE /publications/{id}/assignments/{assignment_id}`, or `POST /publications/{id}/auto-assign` while the effective state is not `ASSIGNING`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_ASSIGNING`

#### Scenario: Assignment board read during ACTIVE is allowed

- **WHEN** an admin calls `GET /publications/{id}/assignment-board` while the effective state is `ASSIGNING` or `ACTIVE`
- **THEN** the request succeeds

### Requirement: Reject assignment of disabled users

The system SHALL reject an admin attempt to assign a disabled user with HTTP 409 and error code `USER_DISABLED`.

#### Scenario: Admin tries to assign a disabled user

- **GIVEN** a user whose account is disabled
- **WHEN** an admin creates an assignment with `user_id` set to that user
- **THEN** the response is HTTP 409 with error code `USER_DISABLED`

### Requirement: Admin assignment endpoints

The system SHALL expose `GET /publications/{id}/assignment-board`, `POST /publications/{id}/auto-assign`, `POST /publications/{id}/assignments`, and `DELETE /publications/{id}/assignments/{assignment_id}`, all requiring `RequireAdmin` and the state gates described above.

#### Scenario: Non-admin cannot access assignment endpoints

- **WHEN** an employee calls any of the admin assignment endpoints
- **THEN** the request is rejected by `RequireAdmin`

### Requirement: Auto-assign replaces the full assignment set via MCMF

`POST /publications/{id}/auto-assign` SHALL run a min-cost max-flow solver over the candidate pool and SHALL replace the entire assignment set for the publication inside one transaction, so a partial result is never observed.

The graph SHALL be constructed as follows: a source `s`; for each user with at least one candidacy, per-weekday maximal overlap groups of shifts the user ticked (a user may take at most one shift per overlap group); up to `min(#groups, total_demand)` per-user "slot" nodes between `s` and a central "employee" node; one node per shift; shifts connected to sink `t` with capacity `required_headcount` and a negative coverage bonus; all user-side edges of capacity 1; slot edges with costs that grow linearly with the slot index so work is spread across employees. The coverage bonus SHALL be large and negative (`-2 * total_demand`) so demand fill dominates spreading.

The solver SHALL NOT optimise for fairness over time, seniority, or preference weighting; those are out of scope. Admins MAY hand-edit any assignment afterward.

#### Scenario: Auto-assign is atomic

- **GIVEN** a publication with an existing assignment set
- **WHEN** an admin calls `POST /publications/{id}/auto-assign`
- **THEN** the response reflects the new assignment set with the previous set fully replaced, or an error with the previous set untouched — no partial replacement is observed

#### Scenario: Auto-assign does not double-book within an overlap group

- **GIVEN** a user who ticked two shifts that overlap on the same weekday
- **WHEN** auto-assign runs
- **THEN** the user is assigned to at most one of those shifts

### Requirement: Shift-change request data model

`shift_change_requests` rows SHALL carry: `id BIGSERIAL`, `publication_id BIGINT` (FK with `ON DELETE CASCADE`), `type TEXT` with CHECK `IN ('swap', 'give_direct', 'give_pool')`, `requester_user_id BIGINT` (FK to `users.id`), `requester_assignment_id BIGINT` (the offered assignment; no FK), `counterpart_user_id BIGINT NULL` (required for `swap` and `give_direct`, null for `give_pool`), `counterpart_assignment_id BIGINT NULL` (required for `swap` only; no FK), `state TEXT` with CHECK `IN ('pending', 'approved', 'rejected', 'cancelled', 'expired', 'invalidated')`, `decided_by_user_id BIGINT NULL`, `created_at`, `decided_at TIMESTAMPTZ NULL` (null until terminal), and `expires_at TIMESTAMPTZ` set to `publication.planned_active_from` at creation.

Indexes SHALL cover `(publication_id, state, created_at DESC)`, `(requester_user_id, state, created_at DESC)`, and `(counterpart_user_id, state, created_at DESC)`.

Assignment ID columns SHALL NOT be FK-enforced so an admin edit that deletes a referenced assignment does not cascade-delete pending rows; staleness is detected lazily at approval time.

#### Scenario: Unknown type is rejected at the database

- **WHEN** an insert is attempted with `type = 'borrow'`
- **THEN** the database CHECK rejects the row

#### Scenario: Invalid state is rejected at the database

- **WHEN** an UPDATE or INSERT sets `state` to a value outside the allowed enum
- **THEN** the database CHECK rejects the change

### Requirement: Shift-change endpoints

All shift-change endpoints SHALL require `RequireAuth`. The endpoints SHALL be:
`POST /publications/{id}/shift-changes` (create; gated on `PUBLISHED`),
`GET /publications/{id}/shift-changes` (list, filtered by audience),
`GET /publications/{id}/shift-changes/{request_id}` (detail),
`POST /publications/{id}/shift-changes/{request_id}/approve` (counterpart approve or pool claim; gated on `PUBLISHED`),
`POST /publications/{id}/shift-changes/{request_id}/reject` (counterpart reject; `swap` / `give_direct` only),
`POST /publications/{id}/shift-changes/{request_id}/cancel` (requester cancel),
`GET /users/me/notifications/unread-count` (pending count for viewer as counterpart).

#### Scenario: Create outside PUBLISHED is rejected

- **WHEN** an employee calls `POST /publications/{id}/shift-changes` while the publication's effective state is not `PUBLISHED`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_PUBLISHED`

#### Scenario: Requester must own the offered assignment

- **WHEN** an employee calls `POST /publications/{id}/shift-changes` with a `requester_assignment_id` that does not belong to them
- **THEN** the request is rejected

### Requirement: No self-target on shift changes

For `swap` and `give_direct`, the system SHALL reject `counterpart_user_id = requester_user_id` up-front with error code `SHIFT_CHANGE_SELF`. For `give_pool`, the system SHALL reject at approval time if the caller claiming the pool offer is the requester.

#### Scenario: Self counterpart on swap is rejected at creation

- **WHEN** an employee creates a `swap` whose `counterpart_user_id` equals the requester
- **THEN** the response is HTTP 400 with error code `SHIFT_CHANGE_SELF`

#### Scenario: Requester cannot claim their own pool offer

- **GIVEN** a `give_pool` request created by user `U`
- **WHEN** user `U` calls the approve endpoint on their own pool request
- **THEN** the response is HTTP 400 with error code `SHIFT_CHANGE_SELF`

### Requirement: Mutual qualification on apply

Approving a swap SHALL require each party to be qualified for the other party's position. Accepting a `give_direct` or claiming a `give_pool` SHALL require the receiver to be qualified for the offered shift's position. Qualification failures SHALL surface as HTTP 403 with error code `SHIFT_CHANGE_NOT_QUALIFIED` (for swap/give counterpart qualification) or `NOT_QUALIFIED` (for employee position checks on submission/assignment paths).

#### Scenario: Swap approval fails when counterpart is not qualified for requester's position

- **GIVEN** a pending swap where the counterpart lacks qualification for the requester's offered shift's position
- **WHEN** the counterpart approves
- **THEN** the response is HTTP 403 with error code `SHIFT_CHANGE_NOT_QUALIFIED`

#### Scenario: Swap approval fails when requester is not qualified for counterpart's position

- **GIVEN** a pending swap where the requester lacks qualification for the counterpart's offered shift's position
- **WHEN** the counterpart approves
- **THEN** the response is HTTP 403 with error code `SHIFT_CHANGE_NOT_QUALIFIED`

### Requirement: Time-conflict check before applying

Before applying a swap or a give, the service SHALL recompute the receiver's full weekly assignment set as it would be after applying and SHALL reject with `SHIFT_CHANGE_TIME_CONFLICT` (HTTP 409) if any two assignments would share a weekday and overlap in time (using the standard overlap predicate `a.start < b.end && b.start < a.end`).

Understaffing SHALL NOT cause rejection at this step — it is acceptable for the receiver to take a shift that leaves the original position short-handed, because `required_headcount` is advisory.

#### Scenario: Overlap with existing weekly assignment rejects the apply

- **GIVEN** a pending `give_direct` whose acceptance would place the receiver in two shifts that overlap on the same weekday
- **WHEN** the receiver accepts
- **THEN** the response is HTTP 409 with error code `SHIFT_CHANGE_TIME_CONFLICT`

#### Scenario: Leaving the origin position understaffed does not block apply

- **GIVEN** the origin shift would fall below `required_headcount` after the give is applied
- **WHEN** the receiver accepts and no other rule is violated
- **THEN** the apply succeeds

### Requirement: Optimistic lock on apply (cascade-invalidate)

`ApplySwap` and `ApplyGive` SHALL run inside a single transaction that re-reads both the request row and the referenced assignment row(s). If either referenced assignment's `(id, publication_id, user_id)` no longer matches what the request captured, the repository SHALL return `ErrShiftChangeAssignmentMiss` and the service SHALL transition the request to `invalidated`. The client SHALL observe HTTP 409 with error code `SHIFT_CHANGE_INVALIDATED`.

This mechanism is how admin edits to assignments "cascade-invalidate" pending shift-change requests without a foreign key or trigger.

#### Scenario: Approved stale request is invalidated

- **GIVEN** a pending swap whose captured `requester_assignment_id` no longer exists because the admin deleted that assignment after the request was created
- **WHEN** the counterpart approves
- **THEN** the request's state transitions to `invalidated` and the client receives HTTP 409 with error code `SHIFT_CHANGE_INVALIDATED`

### Requirement: Lazy expiry on read

Read paths SHALL transition a `pending` request whose `NOW() > expires_at` to `expired` so the counterpart's list remains tidy even before the activate transaction runs.

#### Scenario: Reading a stale pending request expires it

- **GIVEN** a pending request where `NOW() > expires_at` and the publication has not been activated
- **WHEN** any caller reads a shift-change list or detail that surfaces this request
- **THEN** the request's state is set to `expired` lazily

### Requirement: Terminal-state guards

Approve, reject, and cancel SHALL each be rejected on a request that is not `pending`. Approve/reject/cancel on a terminal request SHALL yield HTTP 409 with error code `SHIFT_CHANGE_NOT_PENDING`. Approve/reject/cancel on a request past `expires_at` SHALL yield HTTP 409 with error code `SHIFT_CHANGE_EXPIRED`.

#### Scenario: Approve on approved request is rejected

- **GIVEN** a shift-change request whose state is `approved`, `rejected`, `cancelled`, `expired`, or `invalidated`
- **WHEN** any caller calls approve, reject, or cancel
- **THEN** the response is HTTP 409 with error code `SHIFT_CHANGE_NOT_PENDING`

#### Scenario: Approve on expired-by-clock request is rejected

- **WHEN** a caller invokes approve, reject, or cancel on a pending request past `expires_at`
- **THEN** the response is HTTP 409 with error code `SHIFT_CHANGE_EXPIRED`

### Requirement: Shift-change authorization and visibility

The following rules SHALL govern shift-change authorization:

- Create: requester must own the `requester_assignment_id`; publication must be `PUBLISHED`.
- Cancel: only the requester may cancel their own request.
- Reject: only the counterpart, and only for `swap`/`give_direct`.
- Approve: the counterpart for `swap`/`give_direct`; any qualified non-self user for `give_pool`.
- List / view: employees see rows where they are requester, counterpart, or (for `give_pool`) any pending pool request. Admins see everything.

Unauthorized access SHALL yield HTTP 403 with error code `SHIFT_CHANGE_NOT_OWNER`, or HTTP 404 with error code `SHIFT_CHANGE_NOT_FOUND` for rows that are hidden from the viewer.

#### Scenario: Non-counterpart cannot reject a swap

- **WHEN** a user other than the swap's counterpart calls reject
- **THEN** the response is HTTP 403 with error code `SHIFT_CHANGE_NOT_OWNER`

#### Scenario: Non-requester cannot cancel

- **WHEN** a user other than the request's requester calls cancel
- **THEN** the response is HTTP 403 with error code `SHIFT_CHANGE_NOT_OWNER`

#### Scenario: Hidden row yields 404

- **WHEN** an employee fetches a shift-change detail for a row in which they are neither requester nor counterpart, and which is not a pending pool request
- **THEN** the response is HTTP 404 with error code `SHIFT_CHANGE_NOT_FOUND`

### Requirement: Pool request notifications

`give_pool` requests SHALL NOT send an email notification at creation, because they target no specific recipient. `swap` and `give_direct` requests SHALL send an email to the counterpart at creation. Once a pool request is claimed, the requester SHALL receive a resolution email.

#### Scenario: No email at pool creation

- **WHEN** an employee creates a `give_pool` request
- **THEN** no email is sent at creation

#### Scenario: Email at direct creation

- **WHEN** an employee creates a `swap` or `give_direct`
- **THEN** an email is sent to the counterpart

#### Scenario: Resolution email on pool claim

- **WHEN** a qualified non-self user claims a `give_pool`
- **THEN** the requester receives a resolution email

### Requirement: Pending-count badge excludes pool

`GET /users/me/notifications/unread-count` (`CountPendingForViewer`) SHALL exclude `give_pool` requests, because those have no specific recipient and should not be counted in a personal "you have N requests waiting" badge.

#### Scenario: Pool request is not counted

- **GIVEN** a pending `give_pool` request visible to the viewer
- **WHEN** the viewer calls `GET /users/me/notifications/unread-count`
- **THEN** that request is not included in the count

### Requirement: Current-publication and current-roster reads

`GET /publications/current` SHALL return the currently non-ENDED publication (if any) and require `RequireAuth`. `GET /roster/current` SHALL return the roster of the current publication (or empty) and require `RequireAuth`; access is gated on effective state being `PUBLISHED` or `ACTIVE`. When the current publication transitions to `ENDED`, both endpoints SHALL return null/empty.

#### Scenario: Current endpoints return null after end

- **GIVEN** no publication currently has `state != 'ENDED'`
- **WHEN** an authenticated user calls `GET /publications/current` or `GET /roster/current`
- **THEN** the response indicates no current publication (null/empty)

### Requirement: Publication roster read

`GET /publications/{id}/roster` SHALL return the full weekly roster for a publication when its effective state is `PUBLISHED` or `ACTIVE`. Requests outside those states SHALL be rejected with HTTP 409 and error code `PUBLICATION_NOT_ACTIVE` (the code used when a roster is fetched for a publication that is not viewable).

#### Scenario: Roster outside PUBLISHED/ACTIVE is refused

- **WHEN** any caller calls `GET /publications/{id}/roster` while the effective state is not `PUBLISHED` or `ACTIVE`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_ACTIVE`

### Requirement: Weekly roster is computed on read (D4)

Weekly concrete shifts during `PUBLISHED`/`ACTIVE` SHALL be computed on read from `(publication, assignments)`. They SHALL NOT be materialized per week.

#### Scenario: Roster reflects current assignments without materialization

- **GIVEN** a `PUBLISHED` publication and its assignments table
- **WHEN** a caller fetches the roster
- **THEN** the response is derived from the current assignments at read time, with no per-week materialized rows

### Requirement: Employee roster includes full roster with self highlight (E7)

The employee weekly roster SHALL show the full roster — every shift and every assigned user — with the viewer's own shifts flagged for highlighting by the client.

#### Scenario: Viewer sees full roster with self marked

- **WHEN** an employee fetches the weekly roster
- **THEN** all shifts and all assigned users are returned
- **AND** the assignments belonging to the viewer are distinguishable so the client can highlight them

### Requirement: Publication members listing

`GET /publications/{id}/members` SHALL return the set of users assigned within the publication and require `RequireAuth`.

#### Scenario: Members list contains each assigned user once

- **WHEN** a caller fetches `GET /publications/{id}/members`
- **THEN** the response contains each distinct user assigned in the publication

### Requirement: Admins do not tick availability through the standard flow

Administrators SHALL NOT tick their own availability through the employee submission flow. Nothing in the system SHALL prevent a dual-role admin user from being assigned to shifts.

#### Scenario: Admin is assignable even though they do not tick

- **GIVEN** an admin user who is also qualified for a position
- **WHEN** another admin assigns them to a shift
- **THEN** the assignment is accepted (subject to the normal qualification and state checks)

### Requirement: Mutating scheduling operations are audited

Every mutating scheduling endpoint SHALL write to the audit log as described in the `audit` capability. This includes (but is not limited to) template create/update/delete/clone, template-shift create/update/delete, publication create/delete/publish/activate/end, assignment create/delete, auto-assign, availability submission create/delete, and shift-change request create/approve/reject/cancel.

#### Scenario: Publish writes an audit event

- **WHEN** an admin calls `POST /publications/{id}/publish` and it succeeds
- **THEN** a corresponding audit event is recorded with the admin as actor

### Requirement: Scheduling error code catalog

The scheduling subsystem SHALL emit the following JSON `error.code` values with the HTTP statuses given:

- `INVALID_REQUEST` (400) — malformed body/path/query or generic `ErrInvalidInput`.
- `INVALID_PUBLICATION_WINDOW` (400) — window does not satisfy `start < end <= planned_active_from`.
- `SHIFT_CHANGE_INVALID_TYPE` (400) — unknown request type, or wrong counterpart fields for the type.
- `SHIFT_CHANGE_SELF` (400) — counterpart or claimer is the requester themselves.
- `PUBLICATION_NOT_FOUND` (404) — no row, or effective-state resolution requested for a missing publication.
- `TEMPLATE_NOT_FOUND` (404) — referenced template missing.
- `TEMPLATE_SHIFT_NOT_FOUND` (404) — shift not found for the given template.
- `USER_NOT_FOUND` (404) — referenced user missing.
- `SHIFT_CHANGE_NOT_FOUND` (404) — request missing or hidden from the viewer.
- `NOT_QUALIFIED` (403) — employee attempts a submission or approval for a position they lack.
- `SHIFT_CHANGE_NOT_OWNER` (403) — caller is not the request's requester, counterpart, or eligible claimer.
- `SHIFT_CHANGE_NOT_QUALIFIED` (403) — swap or give counterpart is not mutually qualified.
- `PUBLICATION_ALREADY_EXISTS` (409) — create request violates the single-non-ENDED invariant.
- `PUBLICATION_NOT_DELETABLE` (409) — delete request on a non-`DRAFT` publication.
- `PUBLICATION_NOT_COLLECTING` (409) — submission write outside `COLLECTING`.
- `PUBLICATION_NOT_ASSIGNING` (409) — assignment write or publish outside `ASSIGNING`.
- `PUBLICATION_NOT_PUBLISHED` (409) — activate outside `PUBLISHED`, or shift-change write outside `PUBLISHED`.
- `PUBLICATION_NOT_ACTIVE` (409) — end outside `ACTIVE`, or roster fetched for a publication that is not viewable.
- `USER_DISABLED` (409) — admin tries to assign a disabled user.
- `SHIFT_CHANGE_TIME_CONFLICT` (409) — applying the change would create an overlapping weekly assignment.
- `SHIFT_CHANGE_NOT_PENDING` (409) — approve/reject/cancel on a terminal request.
- `SHIFT_CHANGE_EXPIRED` (409) — approve/reject/cancel on a request past `expires_at`.
- `SHIFT_CHANGE_INVALIDATED` (409) — approve surfaces that the captured assignment row is gone or reassigned.
- `INTERNAL_ERROR` (500) — anything else.

#### Scenario: Malformed body yields INVALID_REQUEST

- **WHEN** any scheduling endpoint receives a malformed body, path, or query
- **THEN** the response is HTTP 400 with error code `INVALID_REQUEST`

#### Scenario: Missing publication yields PUBLICATION_NOT_FOUND

- **WHEN** any scheduling endpoint is called with an `{id}` that does not match any publication row
- **THEN** the response is HTTP 404 with error code `PUBLICATION_NOT_FOUND`
