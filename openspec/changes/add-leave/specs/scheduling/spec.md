## ADDED Requirements

### Requirement: Leave data model

`leaves` rows SHALL store `id`, `user_id` (FK to `users` with `ON DELETE CASCADE`), `publication_id` (FK to `publications` with `ON DELETE CASCADE`), `shift_change_request_id` (FK to `shift_change_requests` with `ON DELETE CASCADE`, `UNIQUE`), `category TEXT` with CHECK `IN ('sick','personal','bereavement')`, `reason TEXT NOT NULL DEFAULT ''`, `created_at`, and `updated_at`. Indexes SHALL cover `user_id` and `publication_id`. The `category` column SHALL be a database CHECK enum; adding categories requires a migration.

A leave row SHALL be 1:1 with its underlying shift-change request: every `leaves` row references exactly one `shift_change_requests` row, and a given SCRT row is referenced by at most one leaves row (enforced by the `UNIQUE` constraint). The `user_id` on a leave SHALL equal the `requester_user_id` of its underlying SCRT.

The `leaves` table SHALL NOT carry a stored `state` column; leave state is derived from the underlying SCRT (see *Leave state derivation*).

#### Scenario: Unknown category is rejected at the database

- **WHEN** an insert is attempted with `category = 'vacation'`
- **THEN** the database CHECK rejects the row

#### Scenario: Cascading from SCRT removes the leave

- **GIVEN** a `leaves` row referencing SCRT row `R`
- **WHEN** `R` is removed via the existing `ON DELETE CASCADE` from publication or by admin operation
- **THEN** the `leaves` row is removed in the same cascade

#### Scenario: Two leaves cannot share an SCRT

- **GIVEN** a leaves row referencing SCRT row `R`
- **WHEN** another leaves row insert is attempted with `shift_change_request_id = R`
- **THEN** the database `UNIQUE` constraint rejects it

### Requirement: Leave state derivation

The system SHALL compute `leave.state` on read from the underlying SCRT's `state` according to:

| `shift_change_requests.state` | derived `leave.state` |
|---|---|
| `pending` | `pending` |
| `approved` | `completed` |
| `expired` | `failed` |
| `rejected` | `failed` |
| `cancelled` | `cancelled` |
| `invalidated` | `cancelled` |

No background job, trigger, or write-through SHALL maintain a stored leave state.

#### Scenario: Approved SCRT renders as completed leave

- **GIVEN** a leave whose SCRT state is `approved`
- **WHEN** any reader fetches the leave
- **THEN** the response carries `state = "completed"`

#### Scenario: Expired SCRT renders as failed leave

- **GIVEN** a leave whose SCRT state is `expired` (no one took the offered shift before its `expires_at`)
- **WHEN** any reader fetches the leave
- **THEN** the response carries `state = "failed"`

#### Scenario: Invalidated SCRT renders as cancelled leave

- **GIVEN** a leave whose SCRT state is `invalidated` (admin deleted the underlying assignment)
- **WHEN** any reader fetches the leave
- **THEN** the response carries `state = "cancelled"`

### Requirement: Leave creation

`POST /leaves` SHALL require `RequireAuth`. The body SHALL carry `{ assignment_id, occurrence_date, type, counterpart_user_id?, category, reason? }` where `type ∈ {'give_direct', 'give_pool'}` and `category ∈ {'sick', 'personal', 'bereavement'}`. The endpoint creates one leave row and one underlying SCRT row in a single transaction.

The endpoint SHALL be gated on the current ACTIVE publication's effective state being `ACTIVE`. If the system has no ACTIVE publication, or if the publication's effective state has resolved to `ENDED` by clock, the request SHALL be rejected with HTTP 409 and error code `PUBLICATION_NOT_ACTIVE`.

The endpoint SHALL reject `type = 'swap'` with HTTP 400 and error code `SHIFT_CHANGE_INVALID_TYPE`. Swap is not a leave action.

The endpoint SHALL apply every existing SCRT validation: requester ownership of the assignment, occurrence-date validity (`IsValidOccurrence` per *Occurrence concept and computation*), counterpart qualification (for `give_direct`), self-target rejection (for `give_direct`). Failures SHALL surface with the same HTTP status and error code as the SCRT layer would emit.

On success, the endpoint SHALL emit one `leave.create` audit event with metadata `{ leave_id, user_id, publication_id, shift_change_request_id, category }`. The metadata SHALL NOT include `reason`. The underlying SCRT layer SHALL emit its own `shift_change.create` event as today; both events fire in the same transaction.

#### Scenario: Successful give_pool leave creation

- **GIVEN** an authenticated employee Alice with assignment `A` in the current ACTIVE publication
- **AND** an `occurrence_date` that passes `IsValidOccurrence`
- **WHEN** Alice calls `POST /leaves` with `{ assignment_id: A, occurrence_date, type: 'give_pool', category: 'personal' }`
- **THEN** the response is HTTP 201 with `{ id, share_url: '/leaves/{id}', ... }`
- **AND** one `leaves` row and one `shift_change_requests` row exist, linked via `leave_id`
- **AND** the SCRT's `type = 'give_pool'`, `state = 'pending'`, `leave_id` set

#### Scenario: Swap is rejected for leaves

- **WHEN** an employee calls `POST /leaves` with `type = 'swap'`
- **THEN** the response is HTTP 400 with error code `SHIFT_CHANGE_INVALID_TYPE`

#### Scenario: Leave outside ACTIVE is rejected

- **GIVEN** the current publication's effective state is not `ACTIVE` (e.g., `PUBLISHED`, or no publication at all)
- **WHEN** an employee calls `POST /leaves`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_ACTIVE`

#### Scenario: Leave for a past occurrence is rejected

- **GIVEN** an `occurrence_date` whose computed start time is `<= NOW()`
- **WHEN** an employee calls `POST /leaves`
- **THEN** the response is HTTP 400 with error code `INVALID_OCCURRENCE_DATE`

### Requirement: Leave preview endpoint

`GET /users/me/leaves/preview?from=YYYY-MM-DD&to=YYYY-MM-DD` SHALL require `RequireAuth`. It SHALL return the viewer's future occurrences in the current ACTIVE publication that fall within `[from, to]`.

The response SHALL list each occurrence with `{ assignment_id, occurrence_date, slot: {id, weekday, start_time, end_time}, position: {id, name}, occurrence_start, occurrence_end }`, sorted by `occurrence_start` ascending. Occurrences whose `occurrence_start <= NOW()` SHALL be excluded.

If no ACTIVE publication exists, the response SHALL be HTTP 200 with an empty `occurrences` array. If `from > to`, the response SHALL be HTTP 400 with error code `INVALID_REQUEST`.

#### Scenario: Future occurrences in the requested range

- **GIVEN** Alice has assignments in the current ACTIVE publication with multiple future occurrences in `[2026-05-01, 2026-05-31]`
- **WHEN** Alice calls `GET /users/me/leaves/preview?from=2026-05-01&to=2026-05-31`
- **THEN** the response includes those occurrences sorted by `occurrence_start`

#### Scenario: Past occurrences are filtered out

- **GIVEN** an occurrence on `2026-04-26` whose `occurrence_start` is in the past at request time
- **WHEN** Alice calls preview with `from = 2026-04-01`
- **THEN** the response does NOT include the past occurrence

#### Scenario: No active publication returns empty list

- **GIVEN** no publication has stored or effective state `ACTIVE`
- **WHEN** Alice calls the preview endpoint
- **THEN** the response is HTTP 200 with `{ occurrences: [] }`

#### Scenario: Inverted range is rejected

- **WHEN** Alice calls preview with `from = 2026-05-10&to = 2026-05-01`
- **THEN** the response is HTTP 400 with error code `INVALID_REQUEST`

### Requirement: Leave cancel endpoint

`POST /leaves/{id}/cancel` SHALL require `RequireAuth`. Only the leave's `user_id` SHALL be permitted to call it; other callers SHALL be rejected with HTTP 403 and error code `LEAVE_NOT_OWNER`. A missing leave SHALL be rejected with HTTP 404 and error code `LEAVE_NOT_FOUND`.

If the underlying SCRT is `pending`, the cancel call SHALL transition it to `cancelled` (using the existing `ShiftChangeService.Cancel` path, which already audits and emails). If the SCRT is already terminal (`approved`, `rejected`, `expired`, `cancelled`, `invalidated`), the cancel call SHALL be a no-op success — HTTP 204 — because the leave's derived state is already what the caller wanted (or the underlying transfer already happened and reversing it is out of scope).

On a state-transitioning cancel, the endpoint SHALL emit a `leave.cancel` audit event with metadata `{ leave_id }`. The underlying SCRT cancel SHALL emit its own `shift_change.cancel` event.

#### Scenario: Owner cancels a pending leave

- **GIVEN** Alice's pending leave `L` with SCRT `R` in state `pending`
- **WHEN** Alice calls `POST /leaves/{L}/cancel`
- **THEN** the response is HTTP 204
- **AND** SCRT `R.state = 'cancelled'`
- **AND** `leave.cancel` and `shift_change.cancel` audit events are recorded

#### Scenario: Non-owner cancel is rejected

- **WHEN** Bob calls `POST /leaves/{L}/cancel` for a leave whose `user_id` is Alice
- **THEN** the response is HTTP 403 with error code `LEAVE_NOT_OWNER`

#### Scenario: Cancel of an approved leave is a no-op success

- **GIVEN** Alice's leave whose SCRT state is `approved`
- **WHEN** Alice calls `POST /leaves/{id}/cancel`
- **THEN** the response is HTTP 204
- **AND** no audit event is emitted
- **AND** the SCRT row is unchanged

### Requirement: Leave detail and listing endpoints

`GET /leaves/{id}` SHALL require `RequireAuth` only — any logged-in user SHALL be permitted to read leave details. The response SHALL include the leave row and the underlying SCRT in a single payload, plus derived `state`. A missing leave SHALL be rejected with HTTP 404 and error code `LEAVE_NOT_FOUND`. The frontend SHALL use the SCRT layer's existing authorization rules to decide which action buttons (approve, reject, cancel) to render.

`GET /users/me/leaves` SHALL require `RequireAuth` and return the viewer's leaves, sorted by `created_at DESC`, paginated.

`GET /publications/{id}/leaves` SHALL require `RequireAdmin` and return all leaves in the named publication, sorted by `created_at DESC`, paginated.

#### Scenario: Any logged-in user can read a leave

- **GIVEN** Alice's leave `L`
- **WHEN** any authenticated employee or admin calls `GET /leaves/{L}`
- **THEN** the response is HTTP 200 with the leave detail

#### Scenario: Non-admin cannot list publication-wide leaves

- **WHEN** an employee calls `GET /publications/{id}/leaves`
- **THEN** the request is rejected by the `RequireAdmin` middleware

#### Scenario: Self listing returns own leaves only

- **WHEN** Alice calls `GET /users/me/leaves`
- **THEN** the response contains only leaves whose `user_id = Alice.id`

## MODIFIED Requirements

### Requirement: Shift-change request data model

`shift_change_requests` rows SHALL carry: `id BIGSERIAL`, `publication_id BIGINT` (FK with `ON DELETE CASCADE`), `type TEXT` with CHECK `IN ('swap', 'give_direct', 'give_pool')`, `requester_user_id BIGINT` (FK to `users.id`), `requester_assignment_id BIGINT` (the offered baseline assignment; no FK), `occurrence_date DATE` (the concrete week the request operates on), `counterpart_user_id BIGINT NULL` (required for `swap` and `give_direct`, null for `give_pool`), `counterpart_assignment_id BIGINT NULL` (required for `swap` only; no FK), `counterpart_occurrence_date DATE NULL` (required for `swap` only — the swap counterpart's concrete week, which may differ from the requester's), `state TEXT` with CHECK `IN ('pending', 'approved', 'rejected', 'cancelled', 'expired', 'invalidated')`, `leave_id BIGINT NULL` (FK to `leaves(id)` with `ON DELETE SET NULL`), `decided_by_user_id BIGINT NULL`, `created_at`, `decided_at TIMESTAMPTZ NULL` (null until terminal), and `expires_at TIMESTAMPTZ` derived at creation as `publication.planned_active_from + (slot.weekday - 1) * INTERVAL '1 day' + slot.start_time` for the requester's chosen `(slot, occurrence_date)` — i.e., the actual start time of the requested occurrence.

When `leave_id IS NULL`, the row is a regular shift-change request created via `POST /publications/{id}/shift-changes` and gated on effective state `PUBLISHED`. When `leave_id IS NOT NULL`, the row is a leave-bearing request created via `POST /leaves` and gated on effective state `ACTIVE` (see *Leave creation*).

Indexes SHALL cover `(publication_id, state, created_at DESC)`, `(requester_user_id, state, created_at DESC)`, `(counterpart_user_id, state, created_at DESC)`, and `leave_id`.

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

#### Scenario: Regular shift-changes have leave_id NULL

- **GIVEN** a row created via `POST /publications/{id}/shift-changes`
- **WHEN** the row is inspected
- **THEN** `leave_id IS NULL`

#### Scenario: Leave-bearing requests have leave_id set

- **GIVEN** a row created via `POST /leaves`
- **WHEN** the row is inspected
- **THEN** `leave_id` references a leaves row

### Requirement: Activation bulk-expires pending shift-change requests

Activating a publication SHALL, inside the same transaction that transitions the publication from `PUBLISHED` to `ACTIVE`, perform `UPDATE shift_change_requests SET state='expired' WHERE publication_id = $1 AND state='pending' AND leave_id IS NULL`.

The `leave_id IS NULL` clause excludes leave-bearing requests from activation expiry. In practice, leave-bearing rows do not exist at activation time (they are created during ACTIVE), so the clause is defensive — it documents intent and guards against any future flow that might create leave-bearing rows pre-activation.

#### Scenario: Pending regular requests are expired atomically on activate

- **GIVEN** a `PUBLISHED` publication with two `pending` shift-change requests where `leave_id IS NULL`
- **WHEN** an admin calls `POST /publications/{id}/activate`
- **THEN** the publication's state becomes `ACTIVE` and both requests have state `expired` as a result of the same transaction

#### Scenario: Hypothetical leave-bearing rows survive activation

- **GIVEN** a `PUBLISHED` publication with one `pending` shift-change request where `leave_id` is non-NULL
- **WHEN** an admin calls `POST /publications/{id}/activate`
- **THEN** the publication's state becomes `ACTIVE`
- **AND** the leave-bearing request is NOT expired by this transaction

### Requirement: Scheduling error code catalog

The scheduling subsystem SHALL emit the following JSON `error.code` values with the HTTP statuses given:

- `INVALID_REQUEST` (400) — malformed body/path/query or generic `ErrInvalidInput`.
- `INVALID_PUBLICATION_WINDOW` (400) — window does not satisfy `start < end <= planned_active_from < planned_active_until`.
- `INVALID_OCCURRENCE_DATE` (400) — `occurrence_date` outside the publication's active window, weekday mismatch with the slot, occurrence start time `<= NOW()` at request creation, or roster `?week` parameter outside the window or not a Monday.
- `SHIFT_CHANGE_INVALID_TYPE` (400) — unknown request type, or wrong counterpart fields for the type, or `type = 'swap'` on a leave creation.
- `SHIFT_CHANGE_SELF` (400) — counterpart or claimer is the requester themselves.
- `PUBLICATION_NOT_FOUND` (404) — no row, or effective-state resolution requested for a missing publication.
- `TEMPLATE_NOT_FOUND` (404) — referenced template missing.
- `TEMPLATE_SLOT_NOT_FOUND` (404) — slot not found for the given template.
- `TEMPLATE_SLOT_POSITION_NOT_FOUND` (404) — position composition row not found for the given slot.
- `USER_NOT_FOUND` (404) — referenced user missing.
- `SHIFT_CHANGE_NOT_FOUND` (404) — request missing or hidden from the viewer.
- `LEAVE_NOT_FOUND` (404) — leave row missing.
- `NOT_QUALIFIED` (403) — employee attempts a submission or approval for a `(slot, position)` they lack.
- `SHIFT_CHANGE_NOT_OWNER` (403) — caller is not the request's requester, counterpart, or eligible claimer.
- `SHIFT_CHANGE_NOT_QUALIFIED` (403) — swap or give counterpart is not mutually qualified.
- `LEAVE_NOT_OWNER` (403) — caller is not the leave's `user_id` on a cancel attempt.
- `PUBLICATION_ALREADY_EXISTS` (409) — create request violates the single-non-ENDED invariant.
- `PUBLICATION_NOT_DELETABLE` (409) — delete request on a non-`DRAFT` publication.
- `PUBLICATION_NOT_COLLECTING` (409) — submission write outside `COLLECTING`.
- `PUBLICATION_NOT_MUTABLE` (409) — assignment create/delete outside `{ASSIGNING, PUBLISHED, ACTIVE}`.
- `PUBLICATION_NOT_ASSIGNING` (409) — auto-assign or publish outside `ASSIGNING`.
- `PUBLICATION_NOT_PUBLISHED` (409) — activate outside `PUBLISHED`, or shift-change write outside `PUBLISHED`.
- `PUBLICATION_NOT_ACTIVE` (409) — end outside `ACTIVE`, leave create outside `ACTIVE`, or roster fetched for a publication that is not viewable.
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

#### Scenario: Missing leave yields LEAVE_NOT_FOUND

- **WHEN** `GET /leaves/{id}` or `POST /leaves/{id}/cancel` is called with an `{id}` that does not match any leaves row
- **THEN** the response is HTTP 404 with error code `LEAVE_NOT_FOUND`

#### Scenario: Cancel by non-owner yields LEAVE_NOT_OWNER

- **WHEN** a user other than the leave's `user_id` calls `POST /leaves/{id}/cancel`
- **THEN** the response is HTTP 403 with error code `LEAVE_NOT_OWNER`
