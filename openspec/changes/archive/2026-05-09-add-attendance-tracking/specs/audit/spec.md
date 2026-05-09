## MODIFIED Requirements

### Requirement: TargetType values are restricted to the defined set

When a `target_type` is set on an audit event, it SHALL be one of: `user`, `position`, `template`, `template_shift`, `publication`, `availability_submission`, `assignment`, `shift_change_request`, `leave`, `attendance_record`, `attendance_overtime`.

#### Scenario: Event outside the enumeration is rejected at review

- **WHEN** a new call site sets `target_type` to a value outside the enumerated set
- **THEN** reviewers reject the change until the enumeration is extended

## ADDED Requirements

### Requirement: Attendance actions are emitted by attendance services

The system SHALL emit attendance audit events from the attendance service on successful attendance mutations. Failed mutations SHALL NOT emit attendance audit events.

The attendance audit action taxonomy SHALL include:

- `attendance.arrival.record` with `target_type = attendance_record`.
- `attendance.arrival.admin_adjust` with `target_type = attendance_record`.
- `attendance.arrival.admin_clear` with `target_type = attendance_record`.
- `attendance.overtime.record` with `target_type = attendance_overtime`.
- `attendance.overtime.admin_create` with `target_type = attendance_overtime`.
- `attendance.overtime.admin_adjust` with `target_type = attendance_overtime`.
- `attendance.overtime.admin_delete` with `target_type = attendance_overtime`.
- `attendance.settings.update` with `target_type = publication`.

Attendance audit metadata SHALL include small typed identifiers such as `publication_id`, `slot_id`, `weekday`, `occurrence_date`, `assignment_id` when applicable, `user_id`, and old/new values for structured fields such as `arrived_at`, `hours`, or `overtime_entry_window_hours`. Metadata SHALL NOT include overtime notes, passwords, tokens, session IDs, or other long free-form text.

#### Scenario: Leader arrival records audit event

- **WHEN** a responsible leader successfully records a user's arrival
- **THEN** one `attendance.arrival.record` audit event is recorded with `target_type = attendance_record`
- **AND** the metadata includes `publication_id`, `assignment_id`, `occurrence_date`, `user_id`, and `arrived_at`

#### Scenario: Admin arrival adjustment records old and new values

- **GIVEN** a user has an existing arrival value
- **WHEN** an admin changes the arrival time
- **THEN** one `attendance.arrival.admin_adjust` audit event is recorded
- **AND** the metadata includes the previous and new `arrived_at` values

#### Scenario: Admin clearing arrival records audit event

- **WHEN** an admin clears an arrival row
- **THEN** one `attendance.arrival.admin_clear` audit event is recorded
- **AND** the metadata identifies the cleared attendance record and user

#### Scenario: Leader overtime records audit event without note text

- **WHEN** a responsible leader successfully records overtime
- **THEN** one `attendance.overtime.record` audit event is recorded with `target_type = attendance_overtime`
- **AND** the metadata includes `publication_id`, `slot_id`, `weekday`, `occurrence_date`, `user_id`, and `hours`
- **AND** the metadata does not include the overtime note body

#### Scenario: Attendance settings update records audit event

- **WHEN** an admin updates a publication's overtime entry window
- **THEN** one `attendance.settings.update` audit event is recorded with `target_type = publication`
- **AND** the metadata includes old and new `overtime_entry_window_hours` values

#### Scenario: Failed attendance mutation emits no audit event

- **WHEN** a leader arrival write is rejected because the leader window is closed
- **THEN** no attendance audit event is recorded for that failed write
