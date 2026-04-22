## ADDED Requirements

### Requirement: Admin may edit assignments during PUBLISHED and ACTIVE

The system SHALL allow an authenticated administrator to create or delete an individual assignment when the publication's effective state is `ASSIGNING`, `PUBLISHED`, or `ACTIVE`. Attempts in any other state SHALL be rejected with `PUBLICATION_NOT_MUTABLE` at HTTP 409.

`AutoAssignPublication` is explicitly excluded from this widening and continues to require effective state `ASSIGNING`.

#### Scenario: Admin creates an assignment during PUBLISHED

- **WHEN** an admin calls `POST /publications/{id}/assignments` while the publication's effective state is `PUBLISHED`
- **THEN** the request succeeds with 201 and the assignment is persisted
- **AND** an `assignment.create` audit event is recorded with the admin as actor

#### Scenario: Admin deletes an assignment during ACTIVE

- **WHEN** an admin calls `DELETE /publications/{id}/assignments/{assignment_id}` while the publication's effective state is `ACTIVE`
- **THEN** the request succeeds with 204 and the assignment row is removed
- **AND** an `assignment.delete` audit event is recorded with the admin as actor

#### Scenario: Admin edits are rejected outside the mutable window

- **WHEN** an admin calls `POST /publications/{id}/assignments` or `DELETE /publications/{id}/assignments/{assignment_id}` while the publication's effective state is `DRAFT`, `COLLECTING`, or `ENDED`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_MUTABLE`
- **AND** the persisted assignment set is unchanged

#### Scenario: Auto-assign remains ASSIGNING-only

- **WHEN** an admin calls `POST /publications/{id}/auto-assign` while the publication's effective state is anything other than `ASSIGNING`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_ASSIGNING`

### Requirement: Admin assignment deletion cascades to pending shift-change requests

When an admin deletes an assignment, the system SHALL transition every pending shift-change request that references the deleted assignment — either as `requester_assignment_id` or as `counterpart_assignment_id` — to the `invalidated` state. For each such request, the system SHALL emit one audit event with action `shift_change.invalidate.cascade` and one email to the requester with outcome `invalidated`.

The cascade is best-effort: failure of the cascade SHALL NOT undo the assignment deletion. The request-approval optimistic lock is the correctness floor; the cascade exists to improve the user experience by not surfacing zombie pending rows.

#### Scenario: Deleting the requester's referenced assignment

- **GIVEN** a pending swap request where `requester_assignment_id = A`
- **WHEN** the admin deletes assignment `A`
- **THEN** the request transitions to `invalidated`
- **AND** one `shift_change.invalidate.cascade` audit event is recorded with metadata `{ request_id, reason: "assignment_deleted", assignment_id: A }`
- **AND** one email is sent to the requester with outcome `invalidated`

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
- **AND** the existing approval-time optimistic lock (`ErrShiftChangeAssignmentMiss`) will still reject any later approve attempt for the affected requests

### Requirement: Assignment board surfaces non-candidate qualified employees

`GET /publications/{id}/assignment-board` SHALL include, per shift, a `non_candidate_qualified` list of users who are qualified for the shift's `position_id` but did NOT submit availability for this publication. Users currently in the shift's `candidates` list or `assignments` list SHALL NOT appear in `non_candidate_qualified`.

Each entry has the same shape as a `candidate` entry: `{ user_id, name, email }`.

#### Scenario: Employee qualified but did not submit

- **GIVEN** a shift with `position_id = P` and an employee qualified for position `P` who did not submit availability for this publication
- **WHEN** an admin fetches the assignment board
- **THEN** the response includes that employee under the shift's `non_candidate_qualified`

#### Scenario: Candidate is excluded from non-candidate list

- **GIVEN** a shift whose `candidates` list includes an employee
- **WHEN** an admin fetches the assignment board
- **THEN** that employee does NOT appear under `non_candidate_qualified` for the same shift

#### Scenario: Currently-assigned employee is excluded from non-candidate list

- **GIVEN** a shift whose `assignments` list includes an employee
- **WHEN** an admin fetches the assignment board
- **THEN** that employee does NOT appear under `non_candidate_qualified` for the same shift
