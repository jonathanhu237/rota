## MODIFIED Requirements

### Requirement: Template and shift data model

`templates` rows SHALL store `id`, `name`, `description`, `is_locked`, `created_at`, `updated_at`.

`template_slots` rows SHALL store `id`, `template_id`, `start_time`, `end_time`, `created_at`, `updated_at`. Database CHECK constraints SHALL enforce `end_time > start_time`. `template_slots` SHALL be indexed on `(template_id, start_time)`. The `template_slots` table SHALL NOT carry a `weekday` column. Multiple slots in the same template MAY share the same time range when their weekday sets are disjoint, allowing different compositions for weekday groups without per-weekday composition on a single slot.

`template_slot_weekdays` rows SHALL store `slot_id`, `weekday`. The PRIMARY KEY SHALL be `(slot_id, weekday)`. A database CHECK SHALL enforce `weekday BETWEEN 1 AND 7` (Monday=1 through Sunday=7). Deleting a `template_slot` SHALL cascade to its `template_slot_weekdays`. A slot's set of applicable weekdays SHALL be the set of `template_slot_weekdays` rows referencing it; the empty set SHALL NOT be permitted by the application layer (slot CRUD enforces non-empty).

A trigger function on `template_slot_weekdays` insert and update SHALL forbid two slots in the same template from claiming the same weekday with overlapping `tsrange(start_time, end_time, '[)')`. The trigger SHALL raise SQLSTATE `23P01` (`exclusion_violation`) so the existing pq-error translation produces `ErrTemplateSlotOverlap`.

`template_slot_positions` rows SHALL store `id`, `slot_id`, `position_id`, `required_headcount`, `created_at`, `updated_at`. `template_slot_positions` SHALL be unique on `(slot_id, position_id)`. `required_headcount > 0` SHALL be enforced by a database CHECK. Deleting a `position` that is referenced by any `template_slot_position` SHALL be blocked by `ON DELETE RESTRICT`. Deleting a `template_slot` SHALL cascade to its `template_slot_positions`. Composition is per-slot (uniform across all weekdays in the slot's set), not per-`(slot, weekday)`.

Template `name` SHALL be trimmed and limited to 100 code points; `description` to 500. Slot times SHALL be stored as `TIME` and serialized over the wire as `HH:MM`. Slots SHALL NOT cross midnight; a span over midnight is represented by two slot rows.

The legacy `template_shifts` table SHALL NOT exist.

#### Scenario: Invalid weekday is rejected at the database

- **WHEN** an insert of a `template_slot_weekdays` row sets `weekday = 0` or `weekday = 8`
- **THEN** the database CHECK rejects the row

#### Scenario: Overlong name is trimmed to the limit

- **WHEN** an admin creates a template with a name longer than 100 code points
- **THEN** the request is rejected with `INVALID_REQUEST`

#### Scenario: Position referenced by a slot cannot be deleted

- **GIVEN** a `template_slot_position` that references `position_id = P`
- **WHEN** an admin attempts to delete position `P`
- **THEN** the delete is blocked by the `ON DELETE RESTRICT` foreign key

#### Scenario: Overlapping slots claiming the same weekday are rejected at the database

- **GIVEN** a `template_slot` at `09:00-11:00` in template `T` with weekday `1` (Monday) in its weekday set
- **WHEN** an admin attempts to add weekday `1` to a different `10:00-12:00` slot in the same template `T`
- **THEN** the trigger on `template_slot_weekdays` rejects the row with SQLSTATE `23P01`
- **AND** the repository translates the exclusion-violation into `ErrTemplateSlotOverlap`
- **AND** the handler returns HTTP 409 with error code `TEMPLATE_SLOT_OVERLAP` (not `INTERNAL_ERROR`)

#### Scenario: Two slots with the same time range but disjoint weekdays coexist

- **GIVEN** a `template_slot` at `09:00-10:00` in template `T` whose weekday set is `{1, 2, 3, 4, 5}` (Mon-Fri)
- **WHEN** an admin creates another `09:00-10:00` slot in the same template with weekday set `{6, 7}` (Sat-Sun)
- **THEN** both slots coexist (the trigger does not fire — no shared weekday)

### Requirement: Template CRUD and shift CRUD

Administrators SHALL be able to list, create, get, update, and delete templates (`GET /templates`, `POST /templates`, `GET /templates/{id}`, `PUT /templates/{id}`, `DELETE /templates/{id}`) and to manage a template's slots and slot-positions. The slot endpoints SHALL be `POST /templates/{id}/slots`, `PATCH /templates/{id}/slots/{slot_id}`, `DELETE /templates/{id}/slots/{slot_id}`. The per-slot position-composition endpoints SHALL be `POST /templates/{id}/slots/{slot_id}/positions`, `PATCH /templates/{id}/slots/{slot_id}/positions/{position_entry_id}`, `DELETE /templates/{id}/slots/{slot_id}/positions/{position_entry_id}`. All of these endpoints SHALL require `RequireAdmin`. `GET /templates` SHALL be paginated. `GET /templates/{id}` SHALL include the template's slots, and each slot SHALL include its position composition and its applicable weekday set.

`POST /templates/{id}/slots` SHALL accept body `{ start_time, end_time, weekdays: int[] }`. The `weekdays` array MUST be non-empty, contain only integers in `[1,7]`, and be deduplicated server-side; an empty or out-of-range `weekdays` array SHALL be rejected with HTTP 400 / `INVALID_REQUEST`. `PATCH /templates/{id}/slots/{slot_id}` SHALL accept any subset of `{ start_time, end_time, weekdays }`; if `weekdays` is present, it replaces the slot's weekday set atomically — removing weekdays cascades and drops referencing `availability_submissions` and `assignments` rows via the composite FK. The legacy `weekday: int` body field SHALL be rejected with HTTP 400 / `INVALID_REQUEST`.

#### Scenario: Admin lists templates

- **WHEN** an admin calls `GET /templates`
- **THEN** a paginated list of templates is returned

#### Scenario: Non-admin cannot access template endpoints

- **WHEN** an employee calls any `/templates*` endpoint
- **THEN** the request is rejected by `RequireAdmin`

#### Scenario: Template detail includes slots, weekdays, and positions

- **WHEN** an admin calls `GET /templates/{id}`
- **THEN** the response includes the template, its slots ordered by `(start_time, end_time, id)`
- **AND** each slot carries `weekdays: int[]` (sorted ascending)
- **AND** each slot carries `positions[]` ordered by `position_id`

#### Scenario: Empty weekdays array is rejected

- **WHEN** an admin calls `POST /templates/{id}/slots` or `PATCH /templates/{id}/slots/{slot_id}` with `weekdays: []`
- **THEN** the response is HTTP 400 with error code `INVALID_REQUEST`

#### Scenario: Legacy weekday body field is rejected

- **WHEN** any caller posts `POST /templates/{id}/slots` or `PATCH /templates/{id}/slots/{slot_id}` with a body containing `weekday: int` and missing the required `weekdays: int[]`
- **THEN** the response is HTTP 400 with error code `INVALID_REQUEST`
- **AND** no row is persisted

#### Scenario: Removing a weekday from a slot cascades to referencing submissions and assignments

- **GIVEN** a slot whose weekday set is `{1, 2, 3}` and which has `availability_submissions` and `assignments` rows referencing weekday `3`
- **WHEN** an admin patches the slot to weekday set `{1, 2}`
- **THEN** the `template_slot_weekdays` row for `weekday = 3` is removed
- **AND** every `availability_submissions` and `assignments` row whose `(slot_id, weekday)` was `(this slot, 3)` is removed by the composite FK's `ON DELETE CASCADE`

### Requirement: Availability submission data model

`availability_submissions` rows SHALL store `id`, `publication_id`, `user_id`, `slot_id`, `weekday`, `created_at`. There SHALL be a UNIQUE constraint on `(publication_id, user_id, slot_id, weekday)`. A database CHECK SHALL enforce `weekday BETWEEN 1 AND 7`. The composite `(slot_id, weekday)` SHALL FK reference `template_slot_weekdays(slot_id, weekday)` with `ON DELETE CASCADE`, guaranteeing a submission can only exist for `(slot, weekday)` cells the slot actually claims. Rows SHALL also `ON DELETE CASCADE` from publication and user. There SHALL be an index on `(publication_id, slot_id, weekday)` to support the auto-assigner and assignment-board reads.

A submission row carries no position information: it expresses "this user is available for this `(slot, weekday)` cell in this publication". The position the user fills is decided downstream by auto-assign (and may be hand-edited by an admin), bounded by `user_positions ∩ template_slot_positions(slot_id)`.

#### Scenario: Duplicate tick is idempotent at the database

- **GIVEN** an existing `availability_submissions` row for `(pub, user, slot, weekday)`
- **WHEN** another insert is attempted for the same tuple
- **THEN** the database's unique constraint rejects it

#### Scenario: Submission for a slot whose composition has no overlap with user_positions is rejected

- **GIVEN** a slot `S` whose composition does not include any position in the user's `user_positions`
- **WHEN** a submission is attempted for `(pub, user, S, weekday)`
- **THEN** the request is rejected with HTTP 403 and error code `NOT_QUALIFIED`

#### Scenario: Submission for a weekday not in the slot's set is rejected

- **GIVEN** a slot `S` whose weekday set is `{1, 2, 3, 4, 5}`
- **WHEN** a submission is attempted for `(pub, user, S, weekday=6)`
- **THEN** the composite FK rejects the row (no `template_slot_weekdays(S, 6)`)
- **AND** the handler returns HTTP 400 with error code `INVALID_REQUEST`

### Requirement: Employee availability endpoints

The system SHALL expose the following employee-facing endpoints, each requiring `RequireAuth`:

- `GET /publications/{id}/shifts/me` — returns the `(slot, weekday)` cells the viewer is qualified to fill (gated on effective state `COLLECTING`).
- `GET /publications/{id}/submissions/me` — returns the viewer's ticked `(slot_id, weekday)` pairs in this publication.
- `POST /publications/{id}/submissions` — body `{ slot_id, weekday }` (gated on `COLLECTING`).
- `DELETE /publications/{id}/submissions/{slot_id}/{weekday}` — un-tick (gated on `COLLECTING`).

`GET /publications/{id}/shifts/me` SHALL return one row per `(slot, weekday)` cell whose slot composition has at least one position in the viewer's `user_positions`. Each row SHALL carry `slot_id`, `weekday`, `start_time`, `end_time`, and a `composition` array — the array enumerates the slot's `(position_id, position_name, required_headcount)` triples for display purposes only. The viewer ticks the cell, not individual positions in the composition.

`GET /publications/{id}/submissions/me` SHALL return an array of `{ slot_id, weekday }` pairs.

#### Scenario: shifts/me filters by qualification overlap

- **GIVEN** a template with slot `S1` whose composition is `{P1}` and slot `S2` whose composition is `{P2}`, and a viewer whose `user_positions` is `{P1}`
- **WHEN** the viewer calls `GET /publications/{id}/shifts/me` during `COLLECTING`
- **THEN** the response contains rows for every `(S1, weekday)` cell of `S1`'s weekday set and does NOT contain any cell of `S2`

#### Scenario: shifts/me response shape carries slot_id, weekday, and composition

- **WHEN** an authenticated employee calls `GET /publications/{id}/shifts/me`
- **THEN** each returned row has fields `slot_id`, `weekday`, `start_time`, `end_time`, `composition`
- **AND** no top-level `position_id` field is present at the row level

#### Scenario: Submission body carries slot_id and weekday

- **WHEN** an authenticated employee calls `POST /publications/{id}/submissions`
- **THEN** the request body is `{ slot_id: <int>, weekday: <int> }`
- **AND** any `position_id` field in the body is ignored
- **AND** a body missing `weekday` is rejected with HTTP 400 / `INVALID_REQUEST`

#### Scenario: Delete URL carries slot_id and weekday

- **WHEN** an authenticated employee calls `DELETE /publications/{id}/submissions/{slot_id}/{weekday}`
- **THEN** the row matching `(publication_id, viewer_user_id, slot_id, weekday)` is removed

### Requirement: Assignment data model

`assignments` rows SHALL store `id`, `publication_id`, `user_id`, `slot_id`, `weekday`, `position_id`, `created_at`. The natural key SHALL be `UNIQUE(publication_id, user_id, slot_id, weekday)`: one user can hold at most one position in any given `(slot, weekday)` cell. A database CHECK SHALL enforce `weekday BETWEEN 1 AND 7`. The composite `(slot_id, weekday)` SHALL FK reference `template_slot_weekdays(slot_id, weekday)` with `ON DELETE CASCADE`. The pair `(slot_id, position_id)` SHALL reference an existing `template_slot_positions` row; this SHALL be enforced by the row-level `assignments_position_belongs_to_slot` trigger (Postgres does not support subqueries in `CHECK`). Rows SHALL be `ON DELETE CASCADE` from publication and user; `position_id` SHALL use `ON DELETE RESTRICT`.

The number of assignments for a given `(publication_id, slot_id, weekday, position_id)` SHOULD equal the slot-position's `required_headcount` but SHALL NOT be hard-enforced: understaffed cells are permitted.

#### Scenario: Understaffed cells are permitted

- **GIVEN** a `(slot, weekday)` cell with a slot-position whose `required_headcount = 3`
- **WHEN** only two assignments exist for that cell-position triple in a publication
- **THEN** the publication may still transition to `PUBLISHED` and `ACTIVE` without server-side rejection

#### Scenario: One user cannot hold two positions in the same (slot, weekday) cell

- **GIVEN** an existing assignment `(publication P, user U, slot S, weekday W, position P1)`
- **WHEN** an insert is attempted for `(publication P, user U, slot S, weekday W, position P2)`
- **THEN** the database's `UNIQUE(publication_id, user_id, slot_id, weekday)` constraint rejects the row
- **AND** the repository translates the `pq` unique-violation into `ErrAssignmentUserAlreadyInSlot`
- **AND** the handler returns HTTP 409 with error code `ASSIGNMENT_USER_ALREADY_IN_SLOT`
- **AND** no `assignment.create` audit event is emitted for the rejected call (the existing row is untouched; the client's intent to add a new position in the same cell is not silently upserted onto the existing row)

#### Scenario: Position must belong to the slot composition

- **GIVEN** a slot `S` whose composition does not include position `P`
- **WHEN** an insert is attempted for `(publication, user, S, weekday, P)`
- **THEN** the `assignments_position_belongs_to_slot` trigger rejects the row

#### Scenario: Assignment for a weekday not in the slot's set is rejected

- **GIVEN** a slot `S` whose weekday set is `{1, 2, 3, 4, 5}`
- **WHEN** an insert is attempted for `(publication, user, S, weekday=6, position)`
- **THEN** the composite FK rejects the row (no `template_slot_weekdays(S, 6)`)
- **AND** the handler returns HTTP 400 with error code `INVALID_REQUEST`

### Requirement: Admin assignment endpoints

The system SHALL expose `GET /publications/{id}/assignment-board`, `POST /publications/{id}/auto-assign`, `POST /publications/{id}/assignments`, and `DELETE /publications/{id}/assignments/{assignment_id}`, all requiring `RequireAdmin` and the state gates described in "Assignment window" and "Admin may edit assignments during PUBLISHED and ACTIVE". The request body of `POST /publications/{id}/assignments` SHALL carry `{ user_id, slot_id, weekday, position_id }`. Any unknown field, including the legacy `template_shift_id`, SHALL be rejected with HTTP 400 and error code `INVALID_REQUEST`. A body missing `weekday` SHALL be rejected with HTTP 400 and error code `INVALID_REQUEST`.

#### Scenario: Non-admin cannot access assignment endpoints

- **WHEN** an employee calls any of the admin assignment endpoints
- **THEN** the request is rejected by `RequireAdmin`

#### Scenario: Create assignment body uses slot_id, weekday, and position_id

- **WHEN** an admin calls `POST /publications/{id}/assignments` with `{ user_id, slot_id, weekday, position_id }` and all other gates pass
- **THEN** the assignment is persisted and the response reflects the new row

#### Scenario: Legacy template_shift_id field is rejected

- **WHEN** any caller posts `POST /publications/{id}/assignments` or `POST /publications/{id}/submissions` with a body containing `template_shift_id` and missing the required `slot_id`/`weekday`/`position_id` fields
- **THEN** the response is HTTP 400 with error code `INVALID_REQUEST`
- **AND** no row is persisted

### Requirement: Auto-assign replaces the full assignment set via MCMF

`POST /publications/{id}/auto-assign` SHALL run a min-cost max-flow solver over the candidate pool and SHALL replace the entire assignment set for the publication inside one transaction, so a partial result is never observed.

The candidate pool SHALL be derived by joining `availability_submissions` (carrying `slot_id` and `weekday`) with each submission's slot composition (`template_slot_positions`), the user's *current* `user_positions`, and the user's *current* `users.status`. A `(user, slot, weekday)` submission contributes candidacy iff `user_positions(user) ∩ composition(slot) ≠ ∅` AND `users.status = 'active'`. A submission whose user has lost all qualifying positions for the slot's composition, or whose user is no longer `active`, SHALL NOT contribute, even though the submission row remains in the database (admin can re-add a position to restore candidacy).

The graph SHALL be constructed as follows: a source `s`; for each user with at least one candidacy, per-weekday maximal overlap groups of `(slot, weekday)` cells the user submitted availability for, where the weekday is the submission's `weekday` column (a user may take at most one cell per overlap group); up to `min(#groups, total_demand)` per-user "seat" nodes between `s` and a central "employee" node; one node per `(slot, weekday, position)` cell (i.e., per `(template_slot_positions row, weekday in slot's set)` that has at least one candidate); an intermediate `(user, slot, weekday)` node of capacity 1 between the user and the `(slot, weekday, position)` cells of that `(slot, weekday)` — edges from `(user, slot, weekday)` go ONLY to those cells whose position is in `user_positions(user)` (so a user is only routed to roles they can actually fill); `(slot, weekday, position)` nodes connected to sink `t` with capacity `required_headcount` and a negative coverage bonus; all user-side edges of capacity 1; seat edges with costs that grow linearly with the seat index so work is spread across employees. The coverage bonus SHALL be large and negative (`-2 * total_demand`) so demand fill dominates spreading.

The solver SHALL NOT optimise for fairness over time, seniority, or preference weighting; those are out of scope. Admins MAY hand-edit any assignment afterward.

#### Scenario: Auto-assign is atomic

- **GIVEN** a publication with an existing assignment set
- **WHEN** an admin calls `POST /publications/{id}/auto-assign`
- **THEN** the response reflects the new assignment set with the previous set fully replaced, or an error with the previous set untouched — no partial replacement is observed

#### Scenario: Auto-assign does not double-book within an overlap group

- **GIVEN** a user who submitted availability for two `(slot, weekday)` cells whose times overlap on the same weekday
- **WHEN** auto-assign runs
- **THEN** the user is assigned to at most one of those cells

#### Scenario: Auto-assign does not put a user in two positions of the same cell

- **GIVEN** a user who submitted availability for `(slot S, weekday W)` whose composition is `{P1, P2}` and the user is qualified for both
- **WHEN** auto-assign runs
- **THEN** the user is assigned to at most one of `(S, W, P1)` or `(S, W, P2)`, consistent with the per-cell unique key

#### Scenario: Auto-assign routes a multi-qualified user to whichever cell helps coverage

- **GIVEN** a user qualified for both `P1` and `P2` who submitted availability for `(slot S, weekday W)` whose composition is `{P1, P2}`
- **AND** another candidate exists for `(S, W, P1)` but not `(S, W, P2)`
- **WHEN** auto-assign runs
- **THEN** the multi-qualified user is preferentially assigned to `(S, W, P2)` so coverage is maximised, subject to the rest of the graph

#### Scenario: Auto-assign skips submissions whose qualification was revoked

- **GIVEN** a user `U` who submitted availability for `(slot S, weekday W)` whose composition is `{P}` while qualified for `P`
- **AND** an admin removed `P` from `U`'s `user_positions` before auto-assign runs
- **WHEN** auto-assign runs
- **THEN** `U` does not appear in the candidate pool for any cell of `S` (no qualifying overlap remains)
- **AND** auto-assign does not assign `U` to any cell of `S`
- **AND** the `availability_submissions` row for `(U, S, W)` is unchanged in the database (it stays for potential future re-qualification)

#### Scenario: Auto-assign skips submissions from disabled users

- **GIVEN** a user `U` who submitted availability and was later disabled
- **WHEN** auto-assign runs
- **THEN** the candidate pool does not include any `(U, slot, weekday)` rows

### Requirement: Shift-change request data model

`shift_change_requests` rows SHALL carry: `id BIGSERIAL`, `publication_id BIGINT` (FK with `ON DELETE CASCADE`), `type TEXT` with CHECK `IN ('swap', 'give_direct', 'give_pool')`, `requester_user_id BIGINT` (FK to `users.id`), `requester_assignment_id BIGINT` (the offered baseline assignment; no FK), `occurrence_date DATE` (the concrete week the request operates on), `counterpart_user_id BIGINT NULL` (required for `swap` and `give_direct`, null for `give_pool`), `counterpart_assignment_id BIGINT NULL` (required for `swap` only; no FK), `counterpart_occurrence_date DATE NULL` (required for `swap` only — the swap counterpart's concrete week, which may differ from the requester's), `state TEXT` with CHECK `IN ('pending', 'approved', 'rejected', 'cancelled', 'expired', 'invalidated')`, `leave_id BIGINT NULL` (FK to `leaves(id)` with `ON DELETE SET NULL`), `decided_by_user_id BIGINT NULL`, `created_at`, `decided_at TIMESTAMPTZ NULL` (null until terminal), and `expires_at TIMESTAMPTZ` derived at creation as `publication.planned_active_from + (assignment.weekday - 1) * INTERVAL '1 day' + slot.start_time` for the requester's chosen `(assignment, occurrence_date)` — i.e., the actual start time of the requested occurrence, where `assignment.weekday` is read from the requester's referenced `assignments` row.

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

### Requirement: Occurrence concept and computation

The system SHALL define a `(slot, weekday, occurrence_date)` triple as the concrete week-instance of a `(slot, weekday)` cell inside a publication. The set of valid occurrences for a publication is enumerable from `(publication.planned_active_from, publication.planned_active_until, slot.start_time)` and the slot's weekday set:

- Let `from := publication.planned_active_from`, `until := publication.planned_active_until`.
- For each slot `S` of the publication's template, and each weekday `W` in `S`'s weekday set, the valid `occurrence_date` values are every calendar date `d` such that `d`'s weekday equals `W` and `from <= (d + S.start_time) AND (d + S.start_time) < until`.
- An occurrence's *actual start time* is `(occurrence_date + slot.start_time)` interpreted as UTC.

The `IsValidOccurrence(publication, slot, weekday, occurrence_date)` predicate SHALL be the authoritative gate for any endpoint that accepts an `occurrence_date`. A request whose `(weekday, occurrence_date)` fails this predicate SHALL be rejected with HTTP 400 and error code `INVALID_OCCURRENCE_DATE`. For shift-change endpoints that operate on an existing `assignment`, the comparand SHALL be `assignment.weekday` (read from the `assignments` row), not the slot's weekday set — i.e., the date's weekday must equal the assignment's weekday column.

#### Scenario: Occurrence weekday must match the assignment's weekday

- **GIVEN** an assignment whose `weekday = 1` (Monday)
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
