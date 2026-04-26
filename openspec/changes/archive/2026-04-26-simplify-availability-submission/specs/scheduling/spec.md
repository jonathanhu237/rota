## MODIFIED Requirements

### Requirement: Qualification gates employee actions

An employee SHALL only be permitted to submit availability for a slot whose composition has at least one position in their `user_positions` set. For shift-change requests (create / accept give / approve swap / claim pool), the qualification check is per `(slot, position)` since assignments carry a specific position. Admins bypass these checks when creating assignments directly.

#### Scenario: Employee submits availability for a slot whose composition does not overlap

- **GIVEN** a slot `S` whose composition is `{P1, P2}` and a viewer whose `user_positions` is `{P3}`
- **WHEN** the viewer submits availability for `S`
- **THEN** the response is HTTP 403 with error code `NOT_QUALIFIED`

#### Scenario: Admin assigns regardless of qualification check path

- **WHEN** an admin creates an assignment for a `(user, slot, position)` triple
- **THEN** the qualification check is enforced against the target user's `user_positions`, and the admin's own qualifications are irrelevant

### Requirement: Availability submission data model

`availability_submissions` rows SHALL store `id`, `publication_id`, `user_id`, `slot_id`, `created_at`. There SHALL be a unique constraint on `(publication_id, user_id, slot_id)`. Rows SHALL be `ON DELETE CASCADE` from publication, user, and slot. There SHALL be an index on `(publication_id, slot_id)` to support the auto-assigner and assignment-board reads.

A submission row carries no position information: it expresses "this user is available during this time block in this publication". The position the user fills is decided downstream by auto-assign (and may be hand-edited by an admin), bounded by `user_positions ∩ template_slot_positions(slot_id)`.

#### Scenario: Duplicate tick is idempotent at the database

- **GIVEN** an existing `availability_submissions` row for `(pub, user, slot)`
- **WHEN** another insert is attempted for the same tuple
- **THEN** the database's unique constraint rejects it

#### Scenario: Submission for a slot whose composition has no overlap with user_positions is rejected

- **GIVEN** a slot `S` whose composition does not include any position in the user's `user_positions`
- **WHEN** a submission is attempted for `(pub, user, S)`
- **THEN** the request is rejected with HTTP 403 and error code `NOT_QUALIFIED`

### Requirement: Availability window

The system SHALL permit creation and deletion of `availability_submissions` only when the publication's *effective* state is `COLLECTING`. Writes outside that window SHALL be rejected with HTTP 409 and error code `PUBLICATION_NOT_COLLECTING`.

#### Scenario: Tick during COLLECTING is accepted

- **GIVEN** a publication whose effective state is `COLLECTING`
- **WHEN** a qualified employee calls `POST /publications/{id}/submissions` for a slot whose composition overlaps their `user_positions`
- **THEN** the submission is persisted

#### Scenario: Tick outside COLLECTING is refused

- **WHEN** an employee calls `POST /publications/{id}/submissions` or `DELETE /publications/{id}/submissions/{slot_id}` while the effective state is not `COLLECTING`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_COLLECTING`

### Requirement: Employee availability endpoints

The system SHALL expose the following employee-facing endpoints, each requiring `RequireAuth`:

- `GET /publications/{id}/shifts/me` — returns the slots the viewer is qualified to fill (gated on effective state `COLLECTING`).
- `GET /publications/{id}/submissions/me` — returns the viewer's ticked `slot_id`s in this publication.
- `POST /publications/{id}/submissions` — body `{ slot_id }` (gated on `COLLECTING`).
- `DELETE /publications/{id}/submissions/{slot_id}` — un-tick (gated on `COLLECTING`).

`GET /publications/{id}/shifts/me` SHALL return one row per slot whose composition has at least one position in the viewer's `user_positions`. Each row SHALL carry `slot_id`, `weekday`, `start_time`, `end_time`, and a `composition` array — the array enumerates the slot's `(position_id, position_name, required_headcount)` triples for display purposes only. The viewer ticks the slot, not individual positions in the composition.

#### Scenario: shifts/me filters by qualification overlap

- **GIVEN** a template with slot `S1` whose composition is `{P1}` and slot `S2` whose composition is `{P2}`, and a viewer whose `user_positions` is `{P1}`
- **WHEN** the viewer calls `GET /publications/{id}/shifts/me` during `COLLECTING`
- **THEN** the response contains `S1` and does NOT contain `S2`

#### Scenario: shifts/me response shape carries slot_id and composition

- **WHEN** an authenticated employee calls `GET /publications/{id}/shifts/me`
- **THEN** each returned row has fields `slot_id`, `weekday`, `start_time`, `end_time`, `composition`
- **AND** no top-level `position_id` field is present at the row level

#### Scenario: Submission body carries slot_id only

- **WHEN** an authenticated employee calls `POST /publications/{id}/submissions`
- **THEN** the request body is `{ slot_id: <int> }`
- **AND** any `position_id` field in the body is ignored

#### Scenario: Delete URL carries slot_id only

- **WHEN** an authenticated employee calls `DELETE /publications/{id}/submissions/{slot_id}`
- **THEN** the row matching `(publication_id, viewer_user_id, slot_id)` is removed

### Requirement: Auto-assign replaces the full assignment set via MCMF

`POST /publications/{id}/auto-assign` SHALL run a min-cost max-flow solver over the candidate pool and SHALL replace the entire assignment set for the publication inside one transaction, so a partial result is never observed.

The candidate pool SHALL be derived by joining `availability_submissions` with each submission's slot composition (`template_slot_positions`), the user's *current* `user_positions`, and the user's *current* `users.status`. A `(user, slot)` submission contributes candidacy iff `user_positions(user) ∩ composition(slot) ≠ ∅` AND `users.status = 'active'`. A submission whose user has lost all qualifying positions for the slot's composition, or whose user is no longer `active`, SHALL NOT contribute, even though the submission row remains in the database (admin can re-add a position to restore candidacy).

The graph SHALL be constructed as follows: a source `s`; for each user with at least one candidacy, per-weekday maximal overlap groups of slots the user submitted availability for (a user may take at most one slot per overlap group); up to `min(#groups, total_demand)` per-user "seat" nodes between `s` and a central "employee" node; one node per `(slot, position)` cell (i.e., per `template_slot_positions` row that has at least one candidate); an intermediate `(user, slot)` node of capacity 1 between the user and the `(slot, position)` cells of that slot — edges from `(user, slot)` go ONLY to those cells whose position is in `user_positions(user)` (so a user is only routed to roles they can actually fill); `(slot, position)` nodes connected to sink `t` with capacity `required_headcount` and a negative coverage bonus; all user-side edges of capacity 1; seat edges with costs that grow linearly with the seat index so work is spread across employees. The coverage bonus SHALL be large and negative (`-2 * total_demand`) so demand fill dominates spreading.

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

- **GIVEN** a user who submitted availability for slot `S` whose composition is `{P1, P2}` and the user is qualified for both
- **WHEN** auto-assign runs
- **THEN** the user is assigned to at most one of `(S, P1)` or `(S, P2)`, consistent with the per-slot unique key

#### Scenario: Auto-assign routes a multi-qualified user to whichever cell helps coverage

- **GIVEN** a user qualified for both `P1` and `P2` who submitted availability for slot `S` whose composition is `{P1, P2}`
- **AND** another candidate exists for `(S, P1)` but not `(S, P2)`
- **WHEN** auto-assign runs
- **THEN** the multi-qualified user is preferentially assigned to `(S, P2)` so coverage is maximised, subject to the rest of the graph

#### Scenario: Auto-assign skips submissions whose qualification was revoked

- **GIVEN** a user `U` who submitted availability for slot `S` whose composition is `{P}` while qualified for `P`
- **AND** an admin removed `P` from `U`'s `user_positions` before auto-assign runs
- **WHEN** auto-assign runs
- **THEN** `U` does not appear in the candidate pool for `S` (no qualifying overlap remains)
- **AND** auto-assign does not assign `U` to any cell of `S`
- **AND** the `availability_submissions` row for `(U, S)` is unchanged in the database (it stays for potential future re-qualification)

#### Scenario: Auto-assign skips submissions from disabled users

- **GIVEN** a user `U` who submitted availability and was later disabled
- **WHEN** auto-assign runs
- **THEN** the candidate pool does not include any `(U, slot)` rows
