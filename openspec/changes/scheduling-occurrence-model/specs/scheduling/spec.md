## ADDED Requirements

### Requirement: Occurrence concept and computation

The system SHALL define a `(slot, occurrence_date)` pair as the concrete week-instance of a slot inside a publication. The set of valid occurrences for a publication is enumerable from `(publication.planned_active_from, publication.planned_active_until, slot.weekday, slot.start_time)`:

- Let `from := publication.planned_active_from`, `until := publication.planned_active_until`.
- For each slot `S` of the publication's template, the valid `occurrence_date` values are every calendar date `d` such that `d`'s weekday equals `S.weekday` and `from <= (d + S.start_time) AND (d + S.start_time) < until`.
- An occurrence's *actual start time* is `(occurrence_date + slot.start_time)` interpreted as UTC.

The `IsValidOccurrence(publication, slot, occurrence_date)` predicate SHALL be the authoritative gate for any endpoint that accepts an `occurrence_date`. A request whose `occurrence_date` fails this predicate SHALL be rejected with HTTP 400 and error code `INVALID_OCCURRENCE_DATE`.

#### Scenario: Occurrence weekday must match slot weekday

- **GIVEN** a slot with `weekday = 1` (Monday)
- **WHEN** a caller submits a shift-change request with `occurrence_date` falling on a Tuesday
- **THEN** the response is HTTP 400 with error code `INVALID_OCCURRENCE_DATE`

#### Scenario: Occurrence must fall inside the publication active window

- **GIVEN** a publication with `planned_active_from = 2026-04-27` and `planned_active_until = 2026-06-22`
- **WHEN** a caller submits a request with `occurrence_date = 2026-06-29` (Monday after the window)
- **THEN** the response is HTTP 400 with error code `INVALID_OCCURRENCE_DATE`

#### Scenario: Occurrence start time must be in the future at request creation

- **GIVEN** the current time is 2026-05-04 09:30
- **AND** a slot with `start_time = 09:00`
- **WHEN** a caller submits a request with `occurrence_date = 2026-05-04` (today)
- **THEN** the response is HTTP 400 with error code `INVALID_OCCURRENCE_DATE`

### Requirement: Assignment override data model

`assignment_overrides` rows SHALL store `id`, `assignment_id` (FK to `assignments` with `ON DELETE CASCADE`), `occurrence_date DATE`, `user_id` (FK to `users` with `ON DELETE CASCADE`), and `created_at`. The natural key SHALL be `UNIQUE(assignment_id, occurrence_date)`: at most one override per baseline assignment per concrete week. There SHALL be an index on `user_id` to support roster look-ups by viewer.

An override row records "for this single week, the baseline assignment's user is replaced by `user_id`". The override SHALL NOT carry a `position_id`: position is always inherited from the baseline `assignments.position_id`. Approving any shift-change request inserts override rows; no other mechanism inserts them.

#### Scenario: Override is unique per assignment-occurrence pair

- **GIVEN** an existing override row `(assignment=A, occurrence_date=2026-05-04)`
- **WHEN** an insert is attempted with the same `(assignment=A, occurrence_date=2026-05-04)`
- **THEN** the database's `UNIQUE(assignment_id, occurrence_date)` constraint rejects the row

#### Scenario: Deleting the baseline assignment cascades to its overrides

- **GIVEN** an `assignment` row `A` with two `assignment_overrides` rows referencing `A`
- **WHEN** an admin deletes `A`
- **THEN** both override rows are removed by `ON DELETE CASCADE`

### Requirement: PATCH publication endpoint

The system SHALL expose `PATCH /publications/{id}`, requiring `RequireAdmin`, accepting any subset of `{ name, description, planned_active_until }`. The endpoint SHALL update only the fields supplied; absent fields SHALL be unchanged.

A `planned_active_until` change SHALL be rejected with HTTP 400 and error code `INVALID_PUBLICATION_WINDOW` if it would violate `planned_active_from < planned_active_until`. A `planned_active_until` change SHALL be permitted regardless of effective state, including for publications that are effectively `ENDED` by clock — moving `until` further into the future therefore re-activates the publication.

`POST /publications/{id}/end` SHALL remain available as syntactic sugar for `PATCH /publications/{id} { planned_active_until: NOW() }`. It SHALL be rejected with HTTP 409 and error code `PUBLICATION_NOT_ACTIVE` when the publication's effective state is not `ACTIVE`.

#### Scenario: Admin extends a publication

- **GIVEN** a publication with `planned_active_until = 2026-06-22`
- **WHEN** an admin calls `PATCH /publications/{id}` with `{ planned_active_until: 2026-07-13 }`
- **THEN** the row is updated and the response reflects the new value

#### Scenario: Admin shortens a publication into the past

- **GIVEN** a publication whose effective state is `ACTIVE` and current `planned_active_until` is in the future
- **WHEN** an admin calls `PATCH /publications/{id}` with `planned_active_until` set to a past timestamp
- **THEN** the row is updated
- **AND** the publication's effective state immediately resolves to `ENDED` on next read

#### Scenario: Patch with from >= until is rejected

- **GIVEN** a publication with `planned_active_from = 2026-04-27`
- **WHEN** an admin calls `PATCH /publications/{id}` with `{ planned_active_until: 2026-04-20 }`
- **THEN** the response is HTTP 400 with error code `INVALID_PUBLICATION_WINDOW`

#### Scenario: Re-activation by extending past until

- **GIVEN** a publication whose `planned_active_until` is in the past, so its effective state is `ENDED`
- **AND** whose stored state has not yet been swept to `ENDED`
- **WHEN** an admin calls `PATCH /publications/{id}` with a future `planned_active_until`
- **THEN** the row is updated
- **AND** the publication's effective state resolves to `ACTIVE` on next read

## MODIFIED Requirements

### Requirement: Publication data model and window invariant

`publications` rows SHALL store `id`, `template_id`, `name`, `state`, `submission_start_at`, `submission_end_at`, `planned_active_from`, `planned_active_until`, `activated_at` (nullable), `created_at`, `updated_at`. A database CHECK SHALL enforce `state ∈ { DRAFT, COLLECTING, ASSIGNING, PUBLISHED, ACTIVE, ENDED }`. A database CHECK SHALL enforce `submission_start_at < submission_end_at <= planned_active_from < planned_active_until`. `template_id` SHALL use `ON DELETE RESTRICT`.

The `ended_at` column SHALL NOT exist; the moment a publication ends is derived from `planned_active_until` (effective ENDED happens when `NOW() > planned_active_until`). Audit records remain the source of truth for "when did the admin act".

#### Scenario: Invalid window rejected by CHECK

- **WHEN** a publication row is written with `submission_start_at >= submission_end_at`, with `submission_end_at > planned_active_from`, or with `planned_active_from >= planned_active_until`
- **THEN** the database CHECK rejects the row
- **AND** the handler maps the failure to HTTP 400 with error code `INVALID_PUBLICATION_WINDOW`

#### Scenario: Template with publications cannot be deleted

- **GIVEN** a template referenced by at least one publication (in any state)
- **WHEN** an admin attempts to delete the template
- **THEN** the delete is blocked by the `ON DELETE RESTRICT` foreign key

### Requirement: Single non-ENDED publication invariant (D2)

At most one publication row SHALL have `state != 'ENDED'` at any time. This SHALL be enforced both in the service layer and by a partial unique index `WHERE state != 'ENDED'`. A create request that would violate this invariant SHALL be rejected with HTTP 409 and error code `PUBLICATION_ALREADY_EXISTS`.

To bridge the gap between effective state (clock-driven) and stored state (write-through), `POST /publications` SHALL, in the same transaction as the new row's `INSERT`, first execute a sweep: `UPDATE publications SET state='ENDED' WHERE state='ACTIVE' AND planned_active_until <= NOW()`. If the sweep transitions the existing publication to `ENDED`, the partial unique index thereafter admits the new row.

#### Scenario: Second non-ENDED publication is rejected

- **GIVEN** an existing publication whose state is not `ENDED` and whose `planned_active_until` is still in the future
- **WHEN** an admin calls `POST /publications` to create another
- **THEN** the response is HTTP 409 with error code `PUBLICATION_ALREADY_EXISTS`

#### Scenario: New publication permitted after ending the previous one

- **GIVEN** the only existing publication has just transitioned to `ENDED`
- **WHEN** an admin calls `POST /publications`
- **THEN** the creation succeeds

#### Scenario: New publication permitted after the previous publication's clock-driven end

- **GIVEN** the only existing publication has stored state `ACTIVE` but `planned_active_until <= NOW()` (effective state has resolved to `ENDED`)
- **WHEN** an admin calls `POST /publications`
- **THEN** the on-create sweep transitions the existing row to stored state `ENDED`
- **AND** the new publication insert succeeds in the same transaction

### Requirement: Publication state transitions

The state machine SHALL be `DRAFT → COLLECTING → ASSIGNING → PUBLISHED → ACTIVE → ENDED`. Transitions from `DRAFT → COLLECTING` and `COLLECTING → ASSIGNING` SHALL be time-driven (effective-state resolution). Transitions from `ASSIGNING → PUBLISHED` and `PUBLISHED → ACTIVE` SHALL be manual admin actions via `POST /publications/{id}/publish` and `POST /publications/{id}/activate` respectively. The transition `ACTIVE → ENDED` SHALL be time-driven by `NOW() > planned_active_until`; admin SHALL be able to short-circuit it via `PATCH /publications/{id} { planned_active_until: ... }` with a current or past timestamp, and `POST /publications/{id}/end` SHALL remain available as a convenience alias that sets `planned_active_until = NOW()` atomically.

The manual transitions (publish, activate) SHALL be implemented as single-row conditional `UPDATE`s; `sql.ErrNoRows` SHALL be folded into a domain "not in expected state" error so concurrent clicks never double-transition.

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

#### Scenario: Time-driven end via clock

- **GIVEN** a publication with stored state `ACTIVE` and `planned_active_until = 2026-06-22 00:00`
- **WHEN** any reader resolves effective state at 2026-06-22 00:01
- **THEN** the effective state is `ENDED`

### Requirement: Effective state resolution on read

Effective state SHALL be computed on every publication read according to the following ordered cascade:

1. If `pub.state = 'ENDED'`, the effective state is `ENDED`.
2. Else if `pub.state = 'ACTIVE'` and `NOW() > pub.planned_active_until`, the effective state is `ENDED`.
3. Else if `pub.state ∈ { 'PUBLISHED', 'ACTIVE' }`, the effective state equals the stored state.
4. Else if `NOW() >= pub.submission_end_at`, the effective state is `ASSIGNING`.
5. Else if `NOW() >= pub.submission_start_at`, the effective state is `COLLECTING`.
6. Else the effective state is `DRAFT`.

No background job SHALL advance the stored state. Stored state SHALL be advanced only when a state-gated write arrives that carries a lazy write-through (e.g., the first submission writing through `DRAFT → COLLECTING`, or a new `POST /publications` sweeping `ACTIVE → ENDED` per requirement *Single non-ENDED publication invariant (D2)*).

#### Scenario: DRAFT is observed as COLLECTING after submission_start_at

- **GIVEN** a stored state of `DRAFT` and `NOW() >= submission_start_at < submission_end_at`
- **WHEN** any reader resolves effective state
- **THEN** the effective state is `COLLECTING` while the stored state remains `DRAFT` until a submission write-through occurs

#### Scenario: COLLECTING is observed as ASSIGNING after submission_end_at

- **GIVEN** `NOW() >= submission_end_at` and a stored state of `DRAFT` or `COLLECTING`
- **WHEN** any reader resolves effective state
- **THEN** the effective state is `ASSIGNING`

#### Scenario: ACTIVE is observed as ENDED after planned_active_until

- **GIVEN** `NOW() > planned_active_until` and a stored state of `ACTIVE`
- **WHEN** any reader resolves effective state
- **THEN** the effective state is `ENDED` even though the stored state remains `ACTIVE` until the next publication-create sweep

#### Scenario: ENDED stored state is terminal

- **GIVEN** a stored state of `ENDED`
- **WHEN** a reader resolves effective state
- **THEN** the effective state is `ENDED` regardless of `planned_active_until`

### Requirement: Shift-change request data model

`shift_change_requests` rows SHALL carry: `id BIGSERIAL`, `publication_id BIGINT` (FK with `ON DELETE CASCADE`), `type TEXT` with CHECK `IN ('swap', 'give_direct', 'give_pool')`, `requester_user_id BIGINT` (FK to `users.id`), `requester_assignment_id BIGINT` (the offered baseline assignment; no FK), `occurrence_date DATE` (the concrete week the request operates on), `counterpart_user_id BIGINT NULL` (required for `swap` and `give_direct`, null for `give_pool`), `counterpart_assignment_id BIGINT NULL` (required for `swap` only; no FK), `counterpart_occurrence_date DATE NULL` (required for `swap` only — the swap counterpart's concrete week, which may differ from the requester's), `state TEXT` with CHECK `IN ('pending', 'approved', 'rejected', 'cancelled', 'expired', 'invalidated')`, `decided_by_user_id BIGINT NULL`, `created_at`, `decided_at TIMESTAMPTZ NULL` (null until terminal), and `expires_at TIMESTAMPTZ` derived at creation as `publication.planned_active_from + (slot.weekday - 1) * INTERVAL '1 day' + slot.start_time` for the requester's chosen `(slot, occurrence_date)` — i.e., the actual start time of the requested occurrence.

Indexes SHALL cover `(publication_id, state, created_at DESC)`, `(requester_user_id, state, created_at DESC)`, and `(counterpart_user_id, state, created_at DESC)`.

Assignment ID columns SHALL NOT be FK-enforced so an admin edit that deletes a referenced assignment does not cascade-delete pending rows; staleness is detected lazily at approval time.

#### Scenario: Unknown type is rejected at the database

- **WHEN** an insert is attempted with `type = 'borrow'`
- **THEN** the database CHECK rejects the row

#### Scenario: Invalid state is rejected at the database

- **WHEN** an UPDATE or INSERT sets `state` to a value outside the allowed enum
- **THEN** the database CHECK rejects the change

#### Scenario: Occurrence date is required

- **WHEN** an insert is attempted with `occurrence_date` NULL
- **THEN** the database `NOT NULL` constraint rejects the row

#### Scenario: Swap requires counterpart occurrence date

- **WHEN** an insert is attempted with `type = 'swap'` and `counterpart_occurrence_date` NULL
- **THEN** the service-layer validation rejects the request with HTTP 400 and error code `INVALID_REQUEST`

### Requirement: Shift-change endpoints

All shift-change endpoints SHALL require `RequireAuth`. The endpoints SHALL be:
`POST /publications/{id}/shift-changes` (create; gated on `PUBLISHED`),
`GET /publications/{id}/shift-changes` (list, filtered by audience),
`GET /publications/{id}/shift-changes/{request_id}` (detail),
`POST /publications/{id}/shift-changes/{request_id}/approve` (counterpart approve or pool claim; gated on `PUBLISHED`),
`POST /publications/{id}/shift-changes/{request_id}/reject` (counterpart reject; `swap` / `give_direct` only),
`POST /publications/{id}/shift-changes/{request_id}/cancel` (requester cancel),
`GET /users/me/notifications/unread-count` (pending count for viewer as counterpart).

The `POST` create body SHALL carry `{ type, requester_assignment_id, occurrence_date, counterpart_user_id?, counterpart_assignment_id?, counterpart_occurrence_date? }`. The `occurrence_date` SHALL be validated by `IsValidOccurrence(publication, slot_of(requester_assignment), occurrence_date)`. For `type = 'swap'`, the same predicate SHALL be applied to the counterpart's `(slot, counterpart_occurrence_date)`.

#### Scenario: Create outside PUBLISHED is rejected

- **WHEN** an employee calls `POST /publications/{id}/shift-changes` while the publication's effective state is not `PUBLISHED`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_PUBLISHED`

#### Scenario: Requester must own the offered assignment

- **WHEN** an employee calls `POST /publications/{id}/shift-changes` with a `requester_assignment_id` that does not belong to them
- **THEN** the request is rejected

#### Scenario: Invalid occurrence date is rejected

- **WHEN** an employee calls `POST /publications/{id}/shift-changes` with an `occurrence_date` failing `IsValidOccurrence`
- **THEN** the response is HTTP 400 with error code `INVALID_OCCURRENCE_DATE`

### Requirement: Optimistic lock on apply (cascade-invalidate)

`ApplySwap` and `ApplyGive` SHALL run inside a single transaction that re-reads both the request row and the referenced baseline assignment row(s). If either referenced assignment's `(id, publication_id, user_id)` no longer matches what the request captured, the repository SHALL return `ErrShiftChangeAssignmentMiss` and the service SHALL transition the request to `invalidated`. The client SHALL observe HTTP 409 with error code `SHIFT_CHANGE_INVALIDATED`.

When the apply succeeds, the service SHALL write override rows in the same transaction:

- For `give_direct` and `give_pool`: insert one row in `assignment_overrides` with `(assignment_id = request.requester_assignment_id, occurrence_date = request.occurrence_date, user_id = approving user)`.
- For `swap`: insert two rows: `(assignment_id = request.requester_assignment_id, occurrence_date = request.occurrence_date, user_id = request.counterpart_user_id)` and `(assignment_id = request.counterpart_assignment_id, occurrence_date = request.counterpart_occurrence_date, user_id = request.requester_user_id)`.

The service SHALL NOT mutate `assignments.user_id` on apply. The baseline `assignments` table is the weekly schedule; per-week deviations live exclusively in `assignment_overrides`.

This mechanism is how admin edits to assignments "cascade-invalidate" pending shift-change requests without a foreign key or trigger.

#### Scenario: Approved stale request is invalidated

- **GIVEN** a pending swap whose captured `requester_assignment_id` no longer exists because the admin deleted that assignment after the request was created
- **WHEN** the counterpart approves
- **THEN** the request's state transitions to `invalidated` and the client receives HTTP 409 with error code `SHIFT_CHANGE_INVALIDATED`

#### Scenario: Approving a give writes one override

- **GIVEN** a pending `give_direct` from Alice to Bob for `(assignment A, occurrence_date 2026-05-04)`
- **WHEN** Bob approves
- **THEN** an `assignment_overrides` row is inserted with `(assignment_id=A, occurrence_date=2026-05-04, user_id=Bob)`
- **AND** the `assignments` row for A is unchanged

#### Scenario: Approving a swap writes two overrides

- **GIVEN** a pending `swap` between Alice's assignment A on 2026-05-04 and Bob's assignment B on 2026-05-05
- **WHEN** Bob approves
- **THEN** two override rows are inserted: `(A, 2026-05-04, Bob)` and `(B, 2026-05-05, Alice)`
- **AND** neither `assignments` row is modified

### Requirement: Admin assignment deletion cascades to pending shift-change requests

When an admin deletes an assignment, the system SHALL transition every pending shift-change request that references the deleted assignment — either as `requester_assignment_id` or as `counterpart_assignment_id`, regardless of `occurrence_date` — to the `invalidated` state. For each such request, the system SHALL emit one audit event with action `shift_change.invalidate.cascade` and one email to the requester with outcome `invalidated`.

The cascade is best-effort: failure of the cascade SHALL NOT undo the assignment deletion. The request-approval optimistic lock is the correctness floor; the cascade exists to improve the user experience by not surfacing zombie pending rows.

The `ON DELETE CASCADE` foreign key on `assignment_overrides.assignment_id` SHALL also remove any override rows for the deleted assignment as part of the same delete.

#### Scenario: Deleting the requester's referenced assignment

- **GIVEN** two pending shift-change requests for assignment `A` on different `occurrence_date`s
- **WHEN** the admin deletes assignment `A`
- **THEN** both requests transition to `invalidated`
- **AND** two `shift_change.invalidate.cascade` audit events are recorded
- **AND** two emails are sent (one per requester)

#### Scenario: Deleting the counterpart's referenced assignment

- **GIVEN** a pending swap request where `counterpart_assignment_id = B`
- **WHEN** the admin deletes assignment `B`
- **THEN** the request transitions to `invalidated`
- **AND** cascade side effects occur as above

#### Scenario: Non-pending requests are not touched

- **GIVEN** an already-`approved`, `rejected`, `cancelled`, `expired`, or `invalidated` shift-change request that references an assignment
- **WHEN** the admin deletes that assignment
- **THEN** the request row is not modified
- **AND** no cascade audit event or email is emitted

#### Scenario: Cascade failure does not block the delete

- **WHEN** the admin's `DeleteAssignment` repository call succeeds, but the cascade `InvalidateRequestsForAssignment` UPDATE errors
- **THEN** the admin's HTTP response is still 204
- **AND** the error is logged at `WARN`
- **AND** the existing approval-time optimistic lock will still reject any later approve attempt for the affected requests

#### Scenario: Existing override rows are removed when assignment is deleted

- **GIVEN** an assignment `A` with two `assignment_overrides` rows
- **WHEN** the admin deletes `A`
- **THEN** the override rows are removed by `ON DELETE CASCADE`

### Requirement: Reject assignment of disabled users

The system SHALL reject an admin attempt to assign a disabled user with HTTP 409 and error code `USER_DISABLED`. The check SHALL be performed twice: once before the apply transaction (fast-fail for UX latency) and once again inside the transaction by re-reading `users.status` with `FOR UPDATE`. The in-tx check is the correctness floor and ensures a user disabled between the pre-tx read and the apply commit is still rejected.

The same in-tx check SHALL be applied on the shift-change apply paths (`ApplySwap`, `ApplyGive`) for every user being mutated (receiver of give, both swap participants). Because the apply path now writes `assignment_overrides` rows rather than mutating `assignments.user_id`, the disabled-status check applies to the override write.

#### Scenario: Admin tries to assign a disabled user

- **GIVEN** a user whose account is disabled
- **WHEN** an admin creates an assignment with `user_id` set to that user
- **THEN** the response is HTTP 409 with error code `USER_DISABLED`

#### Scenario: User disabled after the request was created but before apply

- **GIVEN** a pending `give_direct` to user `U`, where `U` was active when the request was created
- **AND** an approve operation has already been authorized for `U`
- **WHEN** an admin disables `U` before the apply transaction's status check
- **THEN** the apply transaction's in-tx status check `SELECT status FROM users WHERE id = $userID FOR UPDATE` observes `U.status = disabled`
- **AND** the apply rolls back without writing an override row
- **AND** the response is HTTP 409 with error code `USER_DISABLED`

#### Scenario: User disabled between admin's pre-tx check and the insert

- **GIVEN** an admin calls `POST /publications/{id}/assignments` for user `U`
- **AND** the pre-tx user-status read shows `U.status = active`
- **AND** another admin disables `U` in the millisecond between the pre-tx read and the insert tx
- **WHEN** the insert transaction's in-tx status check runs
- **THEN** it observes `U.status = disabled`
- **AND** the insert rolls back
- **AND** the response is HTTP 409 with error code `USER_DISABLED`

### Requirement: Weekly roster is computed on read (D4)

Weekly concrete shifts during `PUBLISHED`/`ACTIVE` SHALL be computed on read from `(publication, assignments, assignment_overrides)`. They SHALL NOT be materialized per week.

For a given target week (`occurrence_date` for the Monday of that week), the roster's concrete user assignment for each `(slot, position)` SHALL be derived as: `assignment_overrides.user_id WHERE (assignment_id, occurrence_date) MATCHES`, falling back to `assignments.user_id` when no override exists.

#### Scenario: Roster reflects current assignments without materialization

- **GIVEN** a `PUBLISHED` publication and its assignments table
- **WHEN** a caller fetches the roster for a specific week
- **THEN** the response is derived from the current `assignments` and `assignment_overrides` at read time, with no per-week materialized rows

#### Scenario: Override takes precedence over baseline

- **GIVEN** an `assignment` row `A` (baseline user = Alice) and an override `(A, 2026-05-04, Bob)`
- **WHEN** a caller fetches the roster for the week containing 2026-05-04
- **THEN** the response shows Bob for that `(slot, position)` on 2026-05-04
- **AND** other weeks for the same slot still show Alice

### Requirement: Publication roster read

`GET /publications/{id}/roster` SHALL return the weekly roster for a publication when its effective state is `PUBLISHED` or `ACTIVE`. Requests outside those states SHALL be rejected with HTTP 409 and error code `PUBLICATION_NOT_ACTIVE`.

The endpoint SHALL accept an optional `?week=YYYY-MM-DD` query parameter naming the Monday of the desired calendar week. When present, the response SHALL contain only the `(slot, position, occurrence_date, user)` rows for that week. When absent, the response SHALL default to the week containing `NOW()` if `NOW()` falls inside `[planned_active_from, planned_active_until)`, or otherwise to the first valid week of the publication.

A `?week` value outside `[planned_active_from, planned_active_until)` or that is not a Monday SHALL be rejected with HTTP 400 and error code `INVALID_OCCURRENCE_DATE`.

#### Scenario: Roster outside PUBLISHED/ACTIVE is refused

- **WHEN** any caller calls `GET /publications/{id}/roster` while the effective state is not `PUBLISHED` or `ACTIVE`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_ACTIVE`

#### Scenario: Roster fetches a specific week

- **WHEN** an authenticated user calls `GET /publications/{id}/roster?week=2026-05-04` for an active publication whose window includes that week
- **THEN** the response contains the (slot, position, user) rows for that week, with overrides applied

#### Scenario: Invalid week parameter is rejected

- **WHEN** an authenticated user calls `GET /publications/{id}/roster?week=2026-05-05` (a Tuesday)
- **THEN** the response is HTTP 400 with error code `INVALID_OCCURRENCE_DATE`

### Requirement: Scheduling error code catalog

The scheduling subsystem SHALL emit the following JSON `error.code` values with the HTTP statuses given:

- `INVALID_REQUEST` (400) — malformed body/path/query or generic `ErrInvalidInput`.
- `INVALID_PUBLICATION_WINDOW` (400) — window does not satisfy `start < end <= planned_active_from < planned_active_until`.
- `INVALID_OCCURRENCE_DATE` (400) — `occurrence_date` outside the publication's active window, weekday mismatch with the slot, occurrence start time `<= NOW()` at request creation, or roster `?week` parameter outside the window or not a Monday.
- `SHIFT_CHANGE_INVALID_TYPE` (400) — unknown request type, or wrong counterpart fields for the type.
- `SHIFT_CHANGE_SELF` (400) — counterpart or claimer is the requester themselves.
- `PUBLICATION_NOT_FOUND` (404) — no row, or effective-state resolution requested for a missing publication.
- `TEMPLATE_NOT_FOUND` (404) — referenced template missing.
- `TEMPLATE_SLOT_NOT_FOUND` (404) — slot not found for the given template.
- `TEMPLATE_SLOT_POSITION_NOT_FOUND` (404) — position composition row not found for the given slot.
- `USER_NOT_FOUND` (404) — referenced user missing.
- `SHIFT_CHANGE_NOT_FOUND` (404) — request missing or hidden from the viewer.
- `NOT_QUALIFIED` (403) — employee attempts a submission or approval for a `(slot, position)` they lack.
- `SHIFT_CHANGE_NOT_OWNER` (403) — caller is not the request's requester, counterpart, or eligible claimer.
- `SHIFT_CHANGE_NOT_QUALIFIED` (403) — swap or give counterpart is not mutually qualified.
- `PUBLICATION_ALREADY_EXISTS` (409) — create request violates the single-non-ENDED invariant.
- `PUBLICATION_NOT_DELETABLE` (409) — delete request on a non-`DRAFT` publication.
- `PUBLICATION_NOT_COLLECTING` (409) — submission write outside `COLLECTING`.
- `PUBLICATION_NOT_MUTABLE` (409) — assignment create/delete outside `{ASSIGNING, PUBLISHED, ACTIVE}`.
- `PUBLICATION_NOT_ASSIGNING` (409) — auto-assign or publish outside `ASSIGNING`.
- `PUBLICATION_NOT_PUBLISHED` (409) — activate outside `PUBLISHED`, or shift-change write outside `PUBLISHED`.
- `PUBLICATION_NOT_ACTIVE` (409) — end outside `ACTIVE`, or roster fetched for a publication that is not viewable.
- `USER_DISABLED` (409) — admin tries to assign a disabled user, or shift-change apply observes a disabled user under `FOR UPDATE`.
- `ASSIGNMENT_USER_ALREADY_IN_SLOT` (409) — admin `CreateAssignment` for a `(publication, user, slot)` triple that already has an assignment row.
- `TEMPLATE_SLOT_OVERLAP` (409) — admin `CreateSlot` / `UpdateSlot` that would violate the GIST exclusion constraint.
- `SHIFT_CHANGE_NOT_PENDING` (409) — approve/reject/cancel on a terminal request.
- `SHIFT_CHANGE_EXPIRED` (409) — approve/reject/cancel on a request past `expires_at`.
- `SHIFT_CHANGE_INVALIDATED` (409) — approve surfaces that the captured baseline assignment row is gone or no longer belongs to the captured user.
- `INTERNAL_ERROR` (500) — anything else.

#### Scenario: Malformed body yields INVALID_REQUEST

- **WHEN** any scheduling endpoint receives a malformed body, path, or query
- **THEN** the response is HTTP 400 with error code `INVALID_REQUEST`

#### Scenario: Missing publication yields PUBLICATION_NOT_FOUND

- **WHEN** any scheduling endpoint is called with an `{id}` that does not match any publication row
- **THEN** the response is HTTP 404 with error code `PUBLICATION_NOT_FOUND`

#### Scenario: Bad occurrence date yields INVALID_OCCURRENCE_DATE

- **WHEN** any endpoint accepting `occurrence_date` receives a value that fails `IsValidOccurrence`
- **THEN** the response is HTTP 400 with error code `INVALID_OCCURRENCE_DATE`
