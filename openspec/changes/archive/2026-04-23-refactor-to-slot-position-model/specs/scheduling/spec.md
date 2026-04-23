## MODIFIED Requirements

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

### Requirement: Time-conflict check before applying

Before applying a swap or a give, the service SHALL recompute the receiver's full weekly assignment set as it would be after applying and SHALL reject with `SHIFT_CHANGE_TIME_CONFLICT` (HTTP 409) if any two assignments would share a weekday and overlap in time (using the overlap predicate `a.start < b.end && b.start < a.end` on the referenced slots' `start_time` and `end_time`).

Understaffing SHALL NOT cause rejection at this step — it is acceptable for the receiver to take an assignment that leaves the original slot-position short-handed, because `required_headcount` is advisory.

#### Scenario: Overlap with existing weekly assignment rejects the apply

- **GIVEN** a pending `give_direct` whose acceptance would place the receiver in two slots that overlap on the same weekday
- **WHEN** the receiver accepts
- **THEN** the response is HTTP 409 with error code `SHIFT_CHANGE_TIME_CONFLICT`

#### Scenario: Leaving the origin slot-position understaffed does not block apply

- **GIVEN** the origin `(slot, position)` would fall below `required_headcount` after the give is applied
- **WHEN** the receiver accepts and no other rule is violated
- **THEN** the apply succeeds

### Requirement: Scheduling error code catalog

The scheduling subsystem SHALL emit the following JSON `error.code` values with the HTTP statuses given:

- `INVALID_REQUEST` (400) — malformed body/path/query or generic `ErrInvalidInput`.
- `INVALID_PUBLICATION_WINDOW` (400) — window does not satisfy `start < end <= planned_active_from`.
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
- `USER_DISABLED` (409) — admin tries to assign a disabled user.
- `ASSIGNMENT_TIME_CONFLICT` (409) — admin `CreateAssignment` would leave the target user with two overlapping same-weekday slot assignments.
- `ASSIGNMENT_USER_ALREADY_IN_SLOT` (409) — admin `CreateAssignment` for a `(publication, user, slot)` triple that already has an assignment row (regardless of the requested `position_id`). The natural key `UNIQUE(publication_id, user_id, slot_id)` is already occupied.
- `TEMPLATE_SLOT_OVERLAP` (409) — admin `CreateSlot` / `UpdateSlot` that would violate the GIST exclusion constraint (two slots of the same `(template_id, weekday)` with overlapping time ranges).
- `SHIFT_CHANGE_TIME_CONFLICT` (409) — applying a shift change would create an overlapping weekly assignment.
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

### Requirement: Admin may edit assignments during PUBLISHED and ACTIVE

The system SHALL allow an authenticated administrator to create or delete an individual assignment when the publication's effective state is `ASSIGNING`, `PUBLISHED`, or `ACTIVE`. Attempts in any other state SHALL be rejected with `PUBLICATION_NOT_MUTABLE` at HTTP 409.

A create attempt that would leave the target user with two assignments whose slots overlap on the same weekday SHALL be rejected with `ASSIGNMENT_TIME_CONFLICT` at HTTP 409 (see "Admin CreateAssignment rejects same-weekday slot overlap").

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

## ADDED Requirements

### Requirement: Admin CreateAssignment rejects same-weekday slot overlap

`POST /publications/{id}/assignments` SHALL, after the state, qualification, and disabled-user gates, recompute the target user's existing assignments in the same publication and SHALL reject with `ASSIGNMENT_TIME_CONFLICT` (HTTP 409) if the new assignment's slot would overlap in time with any existing same-weekday slot the user already holds (overlap predicate: `a.start < b.end && b.start < a.end`). The check SHALL use the referenced slots' `start_time` and `end_time`.

Understaffing SHALL NOT cause rejection at this step.

#### Scenario: Overlap with existing weekly assignment rejects the create

- **GIVEN** user `U` already assigned to `Mon 09:00-11:00 / position P1`
- **WHEN** an admin calls `POST /publications/{id}/assignments` with `{ user_id: U, slot_id: S', position_id: P2 }` where slot `S'` is `Mon 10:00-12:00`
- **THEN** the response is HTTP 409 with error code `ASSIGNMENT_TIME_CONFLICT`
- **AND** no assignment row is written
- **AND** no `assignment.create` audit event is recorded

#### Scenario: Touching boundaries do not count as overlap

- **GIVEN** user `U` already assigned to `Mon 09:00-10:00 / position P1`
- **WHEN** an admin calls `POST /publications/{id}/assignments` with `{ user_id: U, slot_id: S', position_id: P2 }` where slot `S'` is `Mon 10:00-12:00`
- **THEN** the request succeeds (boundaries touch but do not overlap)

#### Scenario: Overlap across different weekdays is not flagged

- **GIVEN** user `U` already assigned to `Mon 09:00-11:00`
- **WHEN** an admin creates a `Tue 09:00-11:00` assignment for the same user
- **THEN** the request succeeds (different weekday)
