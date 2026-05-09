## ADDED Requirements

### Requirement: Template slot position attendance responsibility

Template slot position entries SHALL carry `attendance_responsible: boolean`, defaulting to false. For each `template_slot`, at most one `template_slot_position` row SHALL have `attendance_responsible = true`, and any responsible row SHALL have `required_headcount = 1`.

Template detail responses SHALL include `attendance_responsible` for each slot position. Template slot-position create and update endpoints SHALL accept this field. The service layer SHALL reject any write that would produce multiple responsible rows for a slot or a responsible row with `required_headcount != 1`.

The system SHALL NOT infer responsible rows for production data from position names during migration.

#### Scenario: Responsible position must have headcount one

- **WHEN** an admin creates or updates a template slot position with `attendance_responsible = true` and `required_headcount = 2`
- **THEN** the request is rejected with HTTP 409 and error code `ATTENDANCE_RESPONSIBLE_REQUIRED`

#### Scenario: Slot cannot have two responsible positions

- **GIVEN** a slot already has a responsible position entry
- **WHEN** an admin marks a second position entry in the same slot as responsible
- **THEN** the request is rejected with HTTP 409 and error code `ATTENDANCE_RESPONSIBLE_REQUIRED`

#### Scenario: Template detail exposes responsible marker

- **WHEN** an admin fetches `GET /templates/{id}`
- **THEN** every slot position entry includes `attendance_responsible`

#### Scenario: Production migration does not guess by name

- **GIVEN** a production template has a position named `前台负责人`
- **WHEN** the attendance migration runs
- **THEN** the corresponding slot position row is not automatically marked responsible by name

### Requirement: Publication overtime entry window

Publication rows SHALL store `overtime_entry_window_hours` as a decimal number of hours, defaulting to `24.00`. The value SHALL be greater than or equal to `0` and less than or equal to `168`.

Administrators SHALL be able to update this field for a publication through the attendance settings endpoint. The setting SHALL control only leader overtime entry. It SHALL NOT extend leader arrival recording after the scheduled shift end.

#### Scenario: New publication defaults overtime window to twenty-four hours

- **WHEN** an admin creates a publication without specifying attendance settings
- **THEN** the publication stores `overtime_entry_window_hours = 24.00`

#### Scenario: Admin updates overtime window

- **WHEN** an admin updates a publication's attendance settings to `overtime_entry_window_hours = 12.5`
- **THEN** subsequent leader overtime permission checks for that publication use `12.5` hours

#### Scenario: Negative overtime window is rejected

- **WHEN** an admin sets `overtime_entry_window_hours = -1`
- **THEN** the request is rejected with HTTP 400 and error code `INVALID_REQUEST`

#### Scenario: Overtime window does not reopen arrival entry

- **GIVEN** a publication has `overtime_entry_window_hours = 24`
- **AND** a shift ended one hour ago
- **WHEN** the responsible leader attempts to record an arrival
- **THEN** the request is rejected with HTTP 409 and error code `ATTENDANCE_WINDOW_CLOSED`

### Requirement: Attendance seed data marks demo leaders

Development seed scenarios SHALL mark exactly one responsible position entry per demo slot when the seeded slot has a position whose name contains `负责人`. This seed behavior exists only for local/demo data and SHALL NOT be part of the migration for existing production data.

#### Scenario: Realistic seed marks lead positions

- **WHEN** the realistic seed scenario creates slots with `前台负责人` or `外勤负责人` position entries
- **THEN** each seeded slot marks exactly one of those entries as `attendance_responsible = true`
- **AND** the responsible entry has `required_headcount = 1`

### Requirement: Scheduling error code catalog includes attendance responsibility errors

The scheduling subsystem SHALL emit `ATTENDANCE_RESPONSIBLE_REQUIRED` with HTTP 409 when a template or publication operation would leave a slot without a valid single responsible position for attendance behavior.

#### Scenario: Missing responsible position yields ATTENDANCE_RESPONSIBLE_REQUIRED

- **WHEN** a scheduling write requires attendance responsibility validation and finds a slot with no responsible position
- **THEN** the response is HTTP 409 with error code `ATTENDANCE_RESPONSIBLE_REQUIRED`
