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

`templates` rows SHALL store `id`, `name`, `description`, `is_locked`, `created_at`, `updated_at`.

`template_slots` rows SHALL store `id`, `template_id`, `weekday`, `start_time`, `end_time`, `created_at`, `updated_at`. Database CHECK constraints SHALL enforce `weekday BETWEEN 1 AND 7` (Monday=1 through Sunday=7) and `end_time > start_time`. `template_slots` SHALL be indexed on `(template_id, weekday, start_time)` to back the canonical week-grid sort order. A PostgreSQL GIST `EXCLUDE` constraint SHALL forbid two rows with the same `(template_id, weekday)` whose `tsrange(start_time, end_time, '[)')` overlap.

`template_slot_positions` rows SHALL store `id`, `slot_id`, `position_id`, `required_headcount`, `created_at`, `updated_at`. `template_slot_positions` SHALL be unique on `(slot_id, position_id)`. `required_headcount > 0` SHALL be enforced by a database CHECK. Deleting a `position` that is referenced by any `template_slot_position` SHALL be blocked by `ON DELETE RESTRICT`. Deleting a `template_slot` SHALL cascade to its `template_slot_positions`.

Template `name` SHALL be trimmed and limited to 100 code points; `description` to 500. Slot times SHALL be stored as `TIME` and serialized over the wire as `HH:MM`. Slots SHALL NOT cross midnight; a span over midnight is represented by two slot rows.

The legacy `template_shifts` table SHALL NOT exist.

#### Scenario: Invalid weekday is rejected at the database

- **WHEN** an insert of a `template_slot` sets `weekday = 0` or `weekday = 8`
- **THEN** the database CHECK rejects the row

#### Scenario: Overlong name is trimmed to the limit

- **WHEN** an admin creates a template with a name longer than 100 code points
- **THEN** the request is rejected with `INVALID_REQUEST`

#### Scenario: Position referenced by a slot cannot be deleted

- **GIVEN** a `template_slot_position` that references `position_id = P`
- **WHEN** an admin attempts to delete position `P`
- **THEN** the delete is blocked by the `ON DELETE RESTRICT` foreign key

#### Scenario: Overlapping slots in the same template and weekday are rejected at the database

- **GIVEN** a `template_slot` at `Mon 09:00-11:00` in template `T`
- **WHEN** an insert attempts `Mon 10:00-12:00` in template `T`
- **THEN** the PostgreSQL GIST exclusion constraint rejects the row
- **AND** the repository translates the `pq` exclusion-violation into `ErrTemplateSlotOverlap`
- **AND** the handler returns HTTP 409 with error code `TEMPLATE_SLOT_OVERLAP` (not `INTERNAL_ERROR`)

### Requirement: Template CRUD and shift CRUD

Administrators SHALL be able to list, create, get, update, and delete templates (`GET /templates`, `POST /templates`, `GET /templates/{id}`, `PUT /templates/{id}`, `DELETE /templates/{id}`) and to manage a template's slots and slot-positions. The slot endpoints SHALL be `POST /templates/{id}/slots`, `PATCH /templates/{id}/slots/{slot_id}`, `DELETE /templates/{id}/slots/{slot_id}`. The per-slot position-composition endpoints SHALL be `POST /templates/{id}/slots/{slot_id}/positions`, `PATCH /templates/{id}/slots/{slot_id}/positions/{position_entry_id}`, `DELETE /templates/{id}/slots/{slot_id}/positions/{position_entry_id}`. All of these endpoints SHALL require `RequireAdmin`. `GET /templates` SHALL be paginated. `GET /templates/{id}` SHALL include the template's slots, and each slot SHALL include its position composition.

#### Scenario: Admin lists templates

- **WHEN** an admin calls `GET /templates`
- **THEN** a paginated list of templates is returned

#### Scenario: Non-admin cannot access template endpoints

- **WHEN** an employee calls any `/templates*` endpoint
- **THEN** the request is rejected by `RequireAdmin`

#### Scenario: Template detail includes slots and their positions

- **WHEN** an admin calls `GET /templates/{id}`
- **THEN** the response includes the template, its slots ordered by `(weekday, start_time)`, and each slot's `positions[]` ordered by `position_id`

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

`publications` rows SHALL store `id`, `template_id`, `name`, `description` (TEXT, default empty string), `state`, `submission_start_at`, `submission_end_at`, `planned_active_from`, `planned_active_until`, `activated_at` (nullable), `created_at`, `updated_at`. A database CHECK SHALL enforce `state ∈ { DRAFT, COLLECTING, ASSIGNING, PUBLISHED, ACTIVE, ENDED }`. A database CHECK SHALL enforce `submission_start_at < submission_end_at <= planned_active_from < planned_active_until`. `template_id` SHALL use `ON DELETE RESTRICT`.

The `ended_at` column SHALL NOT exist; the moment a publication ends is derived from `planned_active_until` (effective ENDED happens when `NOW() >= planned_active_until`). Audit records remain the source of truth for "when did the admin act".

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

The state machine SHALL be `DRAFT → COLLECTING → ASSIGNING → PUBLISHED → ACTIVE → ENDED`. Transitions from `DRAFT → COLLECTING` and `COLLECTING → ASSIGNING` SHALL be time-driven (effective-state resolution). Transitions from `ASSIGNING → PUBLISHED` and `PUBLISHED → ACTIVE` SHALL be manual admin actions via `POST /publications/{id}/publish` and `POST /publications/{id}/activate` respectively. The transition `ACTIVE → ENDED` SHALL be time-driven by `NOW() >= planned_active_until`; admin SHALL be able to short-circuit it via `PATCH /publications/{id} { planned_active_until: ... }` with a current or past timestamp, and `POST /publications/{id}/end` SHALL remain available as a convenience alias that sets `planned_active_until = NOW()` atomically.

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
2. Else if `pub.state = 'ACTIVE'` and `NOW() >= pub.planned_active_until`, the effective state is `ENDED`.
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

#### Scenario: ACTIVE is observed as ENDED at or after planned_active_until

- **GIVEN** `NOW() >= planned_active_until` and a stored state of `ACTIVE`
- **WHEN** any reader resolves effective state
- **THEN** the effective state is `ENDED` even though the stored state remains `ACTIVE` until the next publication-create sweep

#### Scenario: ENDED stored state is terminal

- **GIVEN** a stored state of `ENDED`
- **WHEN** a reader resolves effective state
- **THEN** the effective state is `ENDED` regardless of `planned_active_until`

### Requirement: Lazy stored-state write-through on submission

`UpsertSubmission` and `DeleteSubmission` SHALL accept a caller-supplied `PublicationState` override and SHALL set the publication's stored `state = 'COLLECTING'` in the same transaction as the submission write when the stored state was still `DRAFT`.

#### Scenario: First submission during the window advances stored state

- **GIVEN** a publication whose stored state is `DRAFT` but whose effective state is `COLLECTING`
- **WHEN** an employee submits availability
- **THEN** the submission is persisted and the publication's stored state becomes `COLLECTING` in the same transaction

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

### Requirement: Availability submission data model

`availability_submissions` rows SHALL store `id`, `publication_id`, `user_id`, `slot_id`, `position_id`, `created_at`. There SHALL be a unique constraint on `(publication_id, user_id, slot_id, position_id)`. Rows SHALL be `ON DELETE CASCADE` from publication, user, and slot. A submission's `(slot_id, position_id)` pair SHALL correspond to an existing `template_slot_positions` row.

#### Scenario: Duplicate tick is idempotent at the database

- **GIVEN** an existing `availability_submissions` row for `(pub, user, slot, position)`
- **WHEN** another insert is attempted for the same tuple
- **THEN** the database's unique constraint rejects it

#### Scenario: Submitted position must belong to the slot

- **GIVEN** a slot `S` whose composition does not include position `P`
- **WHEN** a submission is attempted for `(pub, user, S, P)`
- **THEN** the request is rejected with `NOT_QUALIFIED` (the client-facing code is unchanged; the server-side enforcement uses the `template_slot_positions` link)

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

`assignments` rows SHALL store `id`, `publication_id`, `user_id`, `slot_id`, `position_id`, `created_at`. The natural key SHALL be `UNIQUE(publication_id, user_id, slot_id)`: one user can hold at most one position in any given slot. The pair `(slot_id, position_id)` SHALL reference an existing `template_slot_positions` row; this SHALL be enforced by a row-level trigger (Postgres does not support subqueries in `CHECK`). Rows SHALL be `ON DELETE CASCADE` from publication, user, and slot; `position_id` SHALL use `ON DELETE RESTRICT`.

The number of assignments for a given `(publication_id, slot_id, position_id)` SHOULD equal the slot-position's `required_headcount` but SHALL NOT be hard-enforced: understaffed slot-positions are permitted.

#### Scenario: Understaffed slot-positions are permitted

- **GIVEN** a slot-position with `required_headcount = 3`
- **WHEN** only two assignments exist for that slot-position in a publication
- **THEN** the publication may still transition to `PUBLISHED` and `ACTIVE` without server-side rejection

#### Scenario: One user cannot hold two positions in the same slot

- **GIVEN** an existing assignment `(publication P, user U, slot S, position P1)`
- **WHEN** an insert is attempted for `(publication P, user U, slot S, position P2)`
- **THEN** the database's `UNIQUE(publication_id, user_id, slot_id)` constraint rejects the row
- **AND** the repository translates the `pq` unique-violation into `ErrAssignmentUserAlreadyInSlot`
- **AND** the handler returns HTTP 409 with error code `ASSIGNMENT_USER_ALREADY_IN_SLOT`
- **AND** no `assignment.create` audit event is emitted for the rejected call (the existing row is untouched; the client's intent to add a new position in the same slot is not silently upserted onto the existing row)

#### Scenario: Position must belong to the slot composition

- **GIVEN** a slot `S` whose composition does not include position `P`
- **WHEN** an insert is attempted for `(publication, user, S, P)`
- **THEN** the `assignments_position_belongs_to_slot` trigger rejects the row

### Requirement: Assignment window

Running auto-assign SHALL require the publication's effective state to be `ASSIGNING`. Creating or deleting an individual assignment SHALL require effective state `∈ {ASSIGNING, PUBLISHED, ACTIVE}` (see "Admin may edit assignments during PUBLISHED and ACTIVE" for the rejection behavior in other states). The assignment-board read SHALL accept effective state `∈ {ASSIGNING, PUBLISHED, ACTIVE}` so admins can see and edit who works what throughout the mutable window.

#### Scenario: Auto-assign outside ASSIGNING is rejected

- **WHEN** an admin calls `POST /publications/{id}/auto-assign` while the effective state is not `ASSIGNING`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_ASSIGNING`

#### Scenario: Assignment board read during the mutable window is allowed

- **WHEN** an admin calls `GET /publications/{id}/assignment-board` while the effective state is `ASSIGNING`, `PUBLISHED`, or `ACTIVE`
- **THEN** the request succeeds

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

### Requirement: Admin assignment endpoints

The system SHALL expose `GET /publications/{id}/assignment-board`, `POST /publications/{id}/auto-assign`, `POST /publications/{id}/assignments`, and `DELETE /publications/{id}/assignments/{assignment_id}`, all requiring `RequireAdmin` and the state gates described in "Assignment window" and "Admin may edit assignments during PUBLISHED and ACTIVE". The request body of `POST /publications/{id}/assignments` SHALL carry `{ user_id, slot_id, position_id }`; `template_shift_id` SHALL NOT be accepted.

#### Scenario: Non-admin cannot access assignment endpoints

- **WHEN** an employee calls any of the admin assignment endpoints
- **THEN** the request is rejected by `RequireAdmin`

#### Scenario: Create assignment body uses slot_id and position_id

- **WHEN** an admin calls `POST /publications/{id}/assignments` with `{ user_id, slot_id, position_id }` and all other gates pass
- **THEN** the assignment is persisted and the response reflects the new row

### Requirement: Auto-assign replaces the full assignment set via MCMF

`POST /publications/{id}/auto-assign` SHALL run a min-cost max-flow solver over the candidate pool and SHALL replace the entire assignment set for the publication inside one transaction, so a partial result is never observed.

The candidate pool SHALL be derived by joining `availability_submissions` with the user's *current* `user_positions` and the user's *current* `users.status`. A submission whose `(user_id, position_id)` is no longer present in `user_positions`, or whose `user_id` is no longer `status = 'active'`, SHALL NOT contribute to the candidate pool, even though the submission row itself remains in the database (admin can re-add the position to restore it).

The graph SHALL be constructed as follows: a source `s`; for each user with at least one candidacy, per-weekday maximal overlap groups of slots the user submitted availability for (a user may take at most one slot per overlap group); up to `min(#groups, total_demand)` per-user "seat" nodes between `s` and a central "employee" node; one node per `(slot, position)` pair (i.e., per `template_slot_positions` row with candidacy); an intermediate `(user, slot)` node of capacity 1 between the user and any `(slot, position)` node for that slot (so a user can hold at most one position in the same slot, consistent with the `UNIQUE(publication_id, user_id, slot_id)` natural key); `(slot, position)` nodes connected to sink `t` with capacity `required_headcount` and a negative coverage bonus; all user-side edges of capacity 1; seat edges with costs that grow linearly with the seat index so work is spread across employees. The coverage bonus SHALL be large and negative (`-2 * total_demand`) so demand fill dominates spreading.

The solver SHALL NOT optimise for fairness over time, seniority, or preference weighting; those are out of scope. Admins MAY hand-edit any assignment afterward.

#### Scenario: Auto-assign is atomic

- **GIVEN** a publication with an existing assignment set
- **WHEN** an admin calls `POST /publications/{id}/auto-assign`
- **THEN** the response reflects the new assignment set with the previous set fully replaced, or an error with the previous set untouched — no partial replacement is observed

#### Scenario: Auto-assign does not double-book within an overlap group

- **GIVEN** a user who submitted availability for two slots that overlap on the same weekday
- **WHEN** auto-assign runs
- **THEN** the user is assigned to at most one of those slots

#### Scenario: Auto-assign does not put a user in two positions of the same slot

- **GIVEN** a user who submitted availability for two positions within the same slot `S`
- **WHEN** auto-assign runs
- **THEN** the user is assigned to at most one of those `(S, position)` pairs, consistent with the per-slot unique key

#### Scenario: Auto-assign skips submissions whose qualification was revoked

- **GIVEN** a user `U` who submitted availability for `(slot S, position P)` while qualified for `P`
- **AND** an admin removed `P` from `U`'s `user_positions` before auto-assign runs
- **WHEN** auto-assign runs
- **THEN** the candidate pool does not include `(U, S, P)`
- **AND** auto-assign does not assign `U` to `(S, P)`
- **AND** the `availability_submissions` row for `(U, S, P)` is unchanged in the database (it stays for potential future re-qualification)

#### Scenario: Auto-assign skips submissions from disabled users

- **GIVEN** a user `U` who submitted availability and was later disabled
- **WHEN** auto-assign runs
- **THEN** the candidate pool does not include any `(U, slot, position)` rows

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

### Requirement: Admin may edit assignments during PUBLISHED and ACTIVE

The system SHALL allow an authenticated administrator to create or delete an individual assignment when the publication's effective state is `ASSIGNING`, `PUBLISHED`, or `ACTIVE`. Attempts in any other state SHALL be rejected with `PUBLICATION_NOT_MUTABLE` at HTTP 409.

`AutoAssignPublication` is explicitly excluded from this widening and continues to require effective state `ASSIGNING`.

#### Scenario: Admin creates an assignment during PUBLISHED

- **WHEN** an admin calls `POST /publications/{id}/assignments` with a non-conflicting body while the publication's effective state is `PUBLISHED`
- **THEN** the request succeeds with 201 and the assignment is persisted
- **AND** an `assignment.create` audit event is recorded with the admin as actor, and metadata `{ publication_id, user_id, slot_id, position_id }`

#### Scenario: Admin deletes an assignment during ACTIVE

- **WHEN** an admin calls `DELETE /publications/{id}/assignments/{assignment_id}` while the publication's effective state is `ACTIVE`
- **THEN** the request succeeds with 204 and the assignment row is removed
- **AND** an `assignment.delete` audit event is recorded with the admin as actor, and metadata `{ publication_id, user_id, slot_id, position_id }`

#### Scenario: Admin edits are rejected outside the mutable window

- **WHEN** an admin calls `POST /publications/{id}/assignments` or `DELETE /publications/{id}/assignments/{assignment_id}` while the publication's effective state is `DRAFT`, `COLLECTING`, or `ENDED`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_MUTABLE`
- **AND** the persisted assignment set is unchanged

#### Scenario: Auto-assign remains ASSIGNING-only

- **WHEN** an admin calls `POST /publications/{id}/auto-assign` while the publication's effective state is anything other than `ASSIGNING`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_ASSIGNING`

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

### Requirement: Assignment board surfaces non-candidate qualified employees

`GET /publications/{id}/assignment-board` SHALL return a list of slots, each carrying its position composition, and per `(slot, position)` pair SHALL include: `candidates` (users who submitted availability for that `(slot, position)`), `assignments` (users currently assigned to that `(slot, position)`), and `non_candidate_qualified` (users qualified for the position but did NOT submit availability for this slot-position in this publication). Users in the `candidates` or `assignments` lists for a given `(slot, position)` SHALL NOT appear in that pair's `non_candidate_qualified`.

Each entry has the shape `{ user_id, name, email }`.

#### Scenario: Employee qualified but did not submit

- **GIVEN** a slot `S` with position `P` in its composition, and an employee qualified for `P` who did not submit availability for `(S, P)` in this publication
- **WHEN** an admin fetches the assignment board
- **THEN** the response includes that employee under the `non_candidate_qualified` of `(S, P)`

#### Scenario: Candidate is excluded from non-candidate list

- **GIVEN** a `(slot, position)` whose `candidates` list includes an employee
- **WHEN** an admin fetches the assignment board
- **THEN** that employee does NOT appear under `non_candidate_qualified` of that same `(slot, position)`

#### Scenario: Currently-assigned employee is excluded from non-candidate list

- **GIVEN** a `(slot, position)` whose `assignments` list includes an employee
- **WHEN** an admin fetches the assignment board
- **THEN** that employee does NOT appear under `non_candidate_qualified` of that same `(slot, position)`

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

