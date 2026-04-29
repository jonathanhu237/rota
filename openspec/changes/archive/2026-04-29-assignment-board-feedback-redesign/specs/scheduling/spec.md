## MODIFIED Requirements

### Requirement: Assignment board surfaces non-candidate qualified employees

`GET /publications/{id}/assignment-board` SHALL return:

- A `slots` array. Each slot carries its position composition. Per `(slot, position)` pair the response SHALL include `assignments` — the list of users currently assigned to that pair. Each assignment entry has shape `{ assignment_id, user_id, name, email }`. Per-pair `candidates` and `non_candidate_qualified` arrays SHALL NOT be returned.
- A top-level `employees` array listing every employee the admin may consider for assignment in this publication. Each entry has shape `{ user_id, name, email, position_ids: int[], submitted_slots: { slot_id: int, weekday: int }[] }`. The array SHALL be sorted ascending by `user_id`.

Filter rules for `employees`:

- The bootstrap admin user SHALL be excluded.
- Users with `status != 'active'` SHALL be excluded.
- `position_ids` for each user SHALL be the intersection of the user's `user_positions` with the set of `position_id`s appearing in any `template_slot_positions` row of the publication's template. Users whose intersection is empty SHALL be excluded from the array.
- `submitted_slots` for each user SHALL list every `(slot_id, weekday)` pair the user has a row in `availability_submissions` for under this publication. Order within the array is not normative; the frontend treats it as an unordered set. The list MAY be empty (employee never submitted) without affecting whether the user appears in `employees` — qualification alone gates membership.

The response shape MAY include other top-level fields (e.g., `publication`, summary metadata) without violating this requirement; only the per-pair `candidates` / `non_candidate_qualified` removal, the top-level `employees` array, and the per-employee `submitted_slots` field are normative.

The auto-assigner does NOT consume this HTTP response; it queries the underlying tables directly. The shape of this endpoint is therefore decoupled from the auto-assigner's correctness.

#### Scenario: Response carries top-level employees array

- **GIVEN** a publication whose template references positions `P1` and `P2`
- **AND** active users `Alice` qualified for `{P1}`, `Bob` qualified for `{P1, P2}`, and `Carol` qualified for `{P2, P3}` where `P3` does not appear in the template
- **WHEN** an admin fetches `GET /publications/{id}/assignment-board`
- **THEN** the response carries a top-level `employees` array
- **AND** the array contains `Alice` with `position_ids = [P1.id]`, `Bob` with `position_ids = [P1.id, P2.id]`, `Carol` with `position_ids = [P2.id]`
- **AND** the array is sorted ascending by `user_id`

#### Scenario: Bootstrap admin and disabled users are excluded

- **GIVEN** a publication whose `employees` array would otherwise include the bootstrap admin and a disabled user
- **WHEN** an admin fetches `GET /publications/{id}/assignment-board`
- **THEN** the bootstrap admin user does NOT appear in `employees`
- **AND** users with `status != 'active'` do NOT appear in `employees`

#### Scenario: Users with no qualifying intersection are excluded

- **GIVEN** an active user qualified only for positions that do not appear in this publication's template
- **WHEN** an admin fetches `GET /publications/{id}/assignment-board`
- **THEN** that user does NOT appear in `employees`

#### Scenario: Per-pair shape no longer carries candidates or non_candidate_qualified

- **WHEN** an admin fetches `GET /publications/{id}/assignment-board`
- **THEN** each entry under `slots[].positions[]` carries `assignments` and the position composition fields
- **AND** the entry does NOT carry `candidates`
- **AND** the entry does NOT carry `non_candidate_qualified`

#### Scenario: Per-pair assignments shape preserved

- **GIVEN** a `(slot, position)` pair with two currently-applied assignments for `Alice` and `Bob`
- **WHEN** an admin fetches `GET /publications/{id}/assignment-board`
- **THEN** the pair's `assignments` array contains exactly `Alice` and `Bob` with `{ assignment_id, user_id, name, email }` shape

#### Scenario: Each employee carries the user's submitted (slot, weekday) pairs

- **GIVEN** a user `Alice` qualified for at least one position in the template
- **AND** Alice has rows in `availability_submissions` for `(slot S1, weekday 1)` and `(slot S2, weekday 3)` under this publication
- **WHEN** an admin fetches `GET /publications/{id}/assignment-board`
- **THEN** Alice's entry in `employees` has `submitted_slots` containing exactly `{ slot_id: S1, weekday: 1 }` and `{ slot_id: S2, weekday: 3 }`

#### Scenario: An employee who never submitted still appears with an empty submitted_slots

- **GIVEN** a user `Bob` qualified for at least one position in the template
- **AND** Bob has zero rows in `availability_submissions` under this publication
- **WHEN** an admin fetches `GET /publications/{id}/assignment-board`
- **THEN** Bob's entry in `employees` carries `submitted_slots: []`
- **AND** Bob's entry is otherwise present and well-formed (qualification alone gates membership)
