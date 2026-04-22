## ADDED Requirements

### Requirement: shift_change.invalidate.cascade action

The audit taxonomy SHALL include an action `shift_change.invalidate.cascade`, emitted once per shift-change request that transitions from `pending` to `invalidated` as a result of an admin assignment deletion.

The event SHALL be recorded with `target_type = shift_change_request` and `target_id = <invalidated request id>`. Its metadata SHALL include `reason = "assignment_deleted"` and the `assignment_id` of the deleted assignment that triggered the cascade.

#### Scenario: Cascade invalidation emits per-request audit events

- **GIVEN** two pending shift-change requests, R1 and R2, each referencing assignment `A`
- **WHEN** the admin deletes assignment `A`
- **THEN** exactly two `shift_change.invalidate.cascade` audit events are recorded
- **AND** each event's `target_id` matches its request's id
- **AND** each event's metadata includes `reason: "assignment_deleted"` and `assignment_id: A`

#### Scenario: No cascade event for non-admin-triggered invalidation

- **GIVEN** a shift-change request that transitions to `invalidated` because the approval-time optimistic lock detected a user_id mismatch (not an admin delete)
- **WHEN** the approval handler completes
- **THEN** no `shift_change.invalidate.cascade` audit event is recorded (the existing `shift_change.approve` rejection path continues to govern that code path)
