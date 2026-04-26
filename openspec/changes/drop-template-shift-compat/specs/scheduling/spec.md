## MODIFIED Requirements

### Requirement: Qualification gates employee actions

An employee SHALL only be permitted to submit availability, create a shift-change request, accept a direct give, approve a swap, or claim a pool request for a `(slot, position)` pair whose `position_id` is in their `user_positions` set. Admins bypass this check when creating assignments directly.

#### Scenario: Employee submits availability for an unqualified position

- **WHEN** an employee submits availability for a `(slot, position)` pair whose `position_id` is not in their `user_positions`
- **THEN** the response is HTTP 403 with error code `NOT_QUALIFIED`

#### Scenario: Admin assigns regardless of qualification check path

- **WHEN** an admin creates an assignment for a `(user, slot, position)` triple
- **THEN** the qualification check is enforced against the target user's `user_positions`, and the admin's own qualifications are irrelevant

### Requirement: Availability window

The system SHALL permit creation and deletion of `availability_submissions` only when the publication's *effective* state is `COLLECTING`. Writes outside that window SHALL be rejected with HTTP 409 and error code `PUBLICATION_NOT_COLLECTING`.

#### Scenario: Tick during COLLECTING is accepted

- **GIVEN** a publication whose effective state is `COLLECTING`
- **WHEN** a qualified employee calls `POST /publications/{id}/submissions` for a `(slot, position)` pair they are qualified for
- **THEN** the submission is persisted

#### Scenario: Tick outside COLLECTING is refused

- **WHEN** an employee calls `POST /publications/{id}/submissions` or `DELETE /publications/{id}/submissions/{slot_id}/{position_id}` while the effective state is not `COLLECTING`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_COLLECTING`

### Requirement: Employee availability endpoints

The system SHALL expose the following employee-facing endpoints, each requiring `RequireAuth`: `GET /publications/{id}/shifts/me` (returns the `(slot, position)` pairs the viewer is qualified for; gated on effective state `COLLECTING`), `GET /publications/{id}/submissions/me` (viewer's own ticked `(slot, position)` pairs), `POST /publications/{id}/submissions` (tick; gated on `COLLECTING`; body `{ slot_id, position_id }`), and `DELETE /publications/{id}/submissions/{slot_id}/{position_id}` (un-tick; gated on `COLLECTING`).

`GET /publications/{id}/shifts/me` SHALL return one row per `(slot, position)` pair whose `position_id` is in the viewer's `user_positions`. Each row SHALL carry `slot_id`, `position_id`, `weekday`, `start_time`, `end_time`, and `required_headcount`. The response SHALL NOT include a single legacy surrogate `id` field; callers identify rows by the `(slot_id, position_id)` natural key.

#### Scenario: shifts/me filters by qualification

- **GIVEN** a template with `(slot, position)` pairs in positions `P1` and `P2`, and a viewer qualified only for `P1`
- **WHEN** the viewer calls `GET /publications/{id}/shifts/me` during `COLLECTING`
- **THEN** the response contains only rows whose `position_id = P1`

#### Scenario: shifts/me response shape carries slot_id and position_id

- **WHEN** an authenticated employee calls `GET /publications/{id}/shifts/me`
- **THEN** each returned row has fields `slot_id`, `position_id`, `weekday`, `start_time`, `end_time`, `required_headcount`
- **AND** no top-level `id` or `template_shift_id` field is present

### Requirement: Admin assignment endpoints

The system SHALL expose `GET /publications/{id}/assignment-board`, `POST /publications/{id}/auto-assign`, `POST /publications/{id}/assignments`, and `DELETE /publications/{id}/assignments/{assignment_id}`, all requiring `RequireAdmin` and the state gates described in "Assignment window" and "Admin may edit assignments during PUBLISHED and ACTIVE". The request body of `POST /publications/{id}/assignments` SHALL carry `{ user_id, slot_id, position_id }`. Any unknown field, including the legacy `template_shift_id`, SHALL be rejected with HTTP 400 and error code `INVALID_REQUEST`.

#### Scenario: Non-admin cannot access assignment endpoints

- **WHEN** an employee calls any of the admin assignment endpoints
- **THEN** the request is rejected by `RequireAdmin`

#### Scenario: Create assignment body uses slot_id and position_id

- **WHEN** an admin calls `POST /publications/{id}/assignments` with `{ user_id, slot_id, position_id }` and all other gates pass
- **THEN** the assignment is persisted and the response reflects the new row

#### Scenario: Legacy template_shift_id field is rejected

- **WHEN** any caller posts `POST /publications/{id}/assignments` or `POST /publications/{id}/submissions` with a body containing `template_shift_id` and missing the required `slot_id`/`position_id` fields
- **THEN** the response is HTTP 400 with error code `INVALID_REQUEST`
- **AND** no row is persisted
