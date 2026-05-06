## ADDED Requirements

### Requirement: Admin availability replacement audit actions

The audit taxonomy SHALL include `availability.admin.create` and `availability.admin.delete` for availability rows added or removed by an administrator through admin availability replacement.

The system SHALL emit one `availability.admin.create` event for each newly inserted availability cell and one `availability.admin.delete` event for each removed availability cell. Each event SHALL use `target_type = availability_submission`. The metadata SHALL include `publication_id`, `user_id`, `slot_id`, and `weekday`. The metadata SHALL NOT include passwords, tokens, session ids, or free-form reason text.

Admin availability replacement events SHALL be recorded only after the replacement transaction succeeds. A failed replacement SHALL NOT emit any audit event. A successful no-op replacement whose target set equals the existing set SHALL NOT emit admin availability audit events.

#### Scenario: Admin replacement records per-cell create and delete events

- **GIVEN** employee Alice currently has availability cell `(S1, 1)`
- **WHEN** an admin replaces Alice's availability with target set `[(S2, 2)]`
- **THEN** one `availability.admin.delete` event is recorded for `(S1, 1)`
- **AND** one `availability.admin.create` event is recorded for `(S2, 2)`
- **AND** both events include metadata `publication_id`, `user_id`, `slot_id`, and `weekday`

#### Scenario: Failed admin replacement records no events

- **GIVEN** an admin availability replacement request contains an ineligible final cell
- **WHEN** the service rejects the request with `NOT_QUALIFIED`
- **THEN** no `availability.admin.create` or `availability.admin.delete` event is recorded

#### Scenario: No-op admin replacement records no events

- **GIVEN** employee Alice currently has availability cells `[(S1, 1)]`
- **WHEN** an admin replaces Alice's availability with target set `[(S1, 1)]`
- **THEN** no `availability.admin.create` or `availability.admin.delete` event is recorded

## MODIFIED Requirements

### Requirement: Mutating domain operations emit audit events

The service layer SHALL emit audit events at the successful end of every state-changing domain operation covered by the action taxonomy (user management, position/template/publication lifecycle, availability submissions, assignments, shift changes). Operations that return an error SHALL NOT emit an audit event. A taxonomy entry MAY define one event per affected row or cell when a single service method intentionally changes multiple auditable entities.

#### Scenario: Successful mutation is recorded

- **WHEN** a service method in the action taxonomy completes successfully
- **THEN** the audit events defined by that action's taxonomy entry are recorded with the matching action constant or constants

#### Scenario: Failed mutation is not recorded

- **GIVEN** a service method that returns an error before completing the mutation
- **WHEN** the caller observes the error
- **THEN** no audit event is recorded for that call
