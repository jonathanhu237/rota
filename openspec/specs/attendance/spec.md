# attendance Specification

## Purpose
TBD - created by archiving change add-attendance-tracking. Update Purpose after archive.
## Requirements
### Requirement: Attendance records are concrete occurrence arrivals

The system SHALL track arrivals for concrete roster occurrence users. An attendance arrival SHALL be identified by `(assignment_id, occurrence_date, user_id)` and SHALL store `arrived_at`, `recorded_by_user_id`, `recorded_at`, `updated_by_user_id`, and `updated_at`. The system SHALL NOT store an explicit attendance status; status SHALL be derived from the current actual roster, the occurrence's scheduled start/end, the current clock, and the arrival row.

For an occurrence assignment, the actual user SHALL be resolved from `assignment_overrides` for `(assignment_id, occurrence_date)` when present, otherwise from `assignments.user_id`. Arrival writes SHALL re-check this actual roster user at write time.

#### Scenario: Arrival follows occurrence override

- **GIVEN** assignment `A` has baseline user Alice
- **AND** `assignment_overrides` assigns `A` on `2026-05-10` to Bob
- **WHEN** a caller reads attendance for assignment `A` on `2026-05-10`
- **THEN** the attendance row is shown for Bob, not Alice

#### Scenario: Stale roster write is rejected

- **GIVEN** a leader loads an attendance page where Alice is assigned to assignment `A`
- **AND** before the leader submits, an occurrence override changes assignment `A` to Bob
- **WHEN** the leader attempts to record Alice's arrival for `A`
- **THEN** the request is rejected with HTTP 409 and error code `ATTENDANCE_ROSTER_STALE`

### Requirement: Attendance status is derived from arrival and scheduled time

The system SHALL derive attendance status as follows:

- `pending` when no arrival exists and `NOW() < scheduled_end`.
- `absent` when no arrival exists and `NOW() >= scheduled_end`.
- `present` when `arrived_at <= scheduled_start`.
- `late` when `arrived_at > scheduled_start`.

The system SHALL NOT apply a grace period when deriving attendance status.

#### Scenario: Arrival before or at start is present

- **GIVEN** a scheduled occurrence starts at `2026-05-10T09:00:00Z`
- **WHEN** the user's arrival is recorded as `2026-05-10T09:00:00Z`
- **THEN** the derived attendance status is `present`

#### Scenario: Arrival after start is late

- **GIVEN** a scheduled occurrence starts at `2026-05-10T09:00:00Z`
- **WHEN** the user's arrival is recorded as `2026-05-10T09:15:00Z`
- **THEN** the derived attendance status is `late`

#### Scenario: No arrival before end is pending

- **GIVEN** a scheduled occurrence ends at `2026-05-10T12:00:00Z`
- **AND** no arrival exists for the user
- **WHEN** attendance is read at `2026-05-10T11:00:00Z`
- **THEN** the derived attendance status is `pending`

#### Scenario: No arrival after end is absent

- **GIVEN** a scheduled occurrence ends at `2026-05-10T12:00:00Z`
- **AND** no arrival exists for the user
- **WHEN** attendance is read at `2026-05-10T12:00:00Z` or later
- **THEN** the derived attendance status is `absent`

### Requirement: Leaders can record current shift arrivals only once

An authenticated non-admin or admin user acting through the leader attendance endpoint SHALL be allowed to record arrivals only when all of the following are true:

- The publication effective state is `ACTIVE`.
- The concrete occurrence is valid for the publication and slot.
- The caller is the current actual user assigned to the slot's attendance-responsible position for that same `(slot_id, weekday, occurrence_date)`.
- `scheduled_start <= NOW() < scheduled_end`.
- The target user is a current actual roster user for the same concrete shift occurrence.
- No arrival row already exists for `(assignment_id, occurrence_date, target_user_id)`.

The arrival payload SHALL accept an optional `arrived_at`. When omitted, the service SHALL default `arrived_at` to `scheduled_start`. When supplied, `arrived_at` SHALL satisfy `scheduled_start <= arrived_at <= NOW()`. A successful leader arrival write SHALL be locked against further leader edits.

#### Scenario: Leader records default scheduled-start arrival

- **GIVEN** Alice is the actual user assigned to the responsible position for the current shift
- **AND** Bob is an actual roster user in the same shift
- **AND** the current time is within the shift window
- **WHEN** Alice records Bob's arrival without an `arrived_at` value
- **THEN** one attendance row is created with `arrived_at` equal to the shift's scheduled start
- **AND** the response shows Bob's derived status as `present`

#### Scenario: Leader records late arrival

- **GIVEN** Alice is the actual responsible user for the current shift
- **AND** Bob is an actual roster user in the same shift
- **AND** the current time is `2026-05-10T09:20:00Z`
- **WHEN** Alice records Bob's arrival with `arrived_at = 2026-05-10T09:15:00Z`
- **THEN** one attendance row is created
- **AND** the response shows Bob's derived status as `late`

#### Scenario: Leader cannot modify locked arrival

- **GIVEN** Alice already recorded Bob's arrival for an occurrence
- **WHEN** Alice attempts to record Bob's arrival again through the leader endpoint
- **THEN** the request is rejected with HTTP 409 and error code `ATTENDANCE_ALREADY_RECORDED`

#### Scenario: Leader cannot record after shift end

- **GIVEN** Alice is the actual responsible user for a shift that ended at `2026-05-10T12:00:00Z`
- **WHEN** Alice attempts to record an arrival at or after `2026-05-10T12:00:00Z`
- **THEN** the request is rejected with HTTP 409 and error code `ATTENDANCE_WINDOW_CLOSED`

#### Scenario: Non-leader cannot record attendance

- **GIVEN** Bob is not the actual user assigned to the responsible position for the current shift
- **WHEN** Bob attempts to record an arrival for that shift through the leader endpoint
- **THEN** the request is rejected with HTTP 403 and error code `ATTENDANCE_NOT_LEADER`

### Requirement: Leader attendance entry lists current and overtime-window shifts

`GET /attendance/current` SHALL require `RequireAuth`. It SHALL return the caller's eligible attendance shifts from the current ACTIVE publication. A shift SHALL be included when the caller is the actual responsible user and either the arrival window is currently open or the overtime entry window has not yet expired.

The endpoint SHALL NOT show shifts before `scheduled_start`. There is no early display window.

#### Scenario: Current shift appears at scheduled start

- **GIVEN** Alice is the responsible user for a shift starting at `2026-05-10T09:00:00Z`
- **WHEN** Alice calls `GET /attendance/current` at `2026-05-10T09:00:00Z`
- **THEN** the response includes that shift with arrival actions enabled

#### Scenario: Future shift does not appear early

- **GIVEN** Alice is the responsible user for a shift starting at `2026-05-10T09:00:00Z`
- **WHEN** Alice calls `GET /attendance/current` at `2026-05-10T08:59:59Z`
- **THEN** the response does not include that shift

#### Scenario: Shift remains visible for overtime window

- **GIVEN** Alice is the responsible user for a shift that ended at `2026-05-10T12:00:00Z`
- **AND** the publication's overtime window is `24` hours
- **WHEN** Alice calls `GET /attendance/current` at `2026-05-11T11:00:00Z`
- **THEN** the response includes that shift with arrival actions disabled and overtime actions enabled

### Requirement: Overtime records are independent locked facts

The system SHALL record overtime independently of roster assignments and occurrence overrides. An overtime record SHALL store `publication_id`, `slot_id`, `weekday`, `occurrence_date`, `user_id`, `hours`, `note`, `recorded_by_user_id`, `recorded_at`, `updated_by_user_id`, and `updated_at`.

`hours` SHALL be decimal, greater than `0`, and no greater than `24`. `note` SHALL be required, trimmed, and no longer than 500 characters. Multiple overtime records for the same user and shift SHALL be allowed. Overtime writes SHALL NOT create, update, or delete `assignments` or `assignment_overrides`.

#### Scenario: Leader records overtime for active user

- **GIVEN** Alice is the responsible user for a shift still inside its overtime entry window
- **AND** Bob is an active user
- **WHEN** Alice records overtime for Bob with `hours = 1.5` and a non-empty note
- **THEN** one overtime record is created
- **AND** no assignment or assignment override row is changed

#### Scenario: Multiple overtime rows are allowed

- **GIVEN** Alice already recorded one overtime row for Bob in a shift
- **WHEN** Alice records another overtime row for Bob in the same shift
- **THEN** the second row is accepted as a separate overtime record

#### Scenario: Blank overtime note is rejected

- **WHEN** a leader or admin submits overtime with a blank note
- **THEN** the request is rejected with HTTP 400 and error code `INVALID_REQUEST`

#### Scenario: Excessive overtime hours are rejected

- **WHEN** a leader or admin submits overtime with `hours = 24.01`
- **THEN** the request is rejected with HTTP 400 and error code `INVALID_REQUEST`

### Requirement: Leaders can record overtime during the publication window

An authenticated user acting through the leader overtime endpoint SHALL be allowed to record overtime when the caller is the actual responsible user for the shift, the target user is active, and `scheduled_start <= NOW() <= scheduled_end + overtime_entry_window_hours`.

Leader overtime writes SHALL be final for leaders. Leaders SHALL NOT update or delete overtime rows after creation.

#### Scenario: Leader records overtime after shift end within window

- **GIVEN** Alice is the responsible user for a shift ending at `2026-05-10T12:00:00Z`
- **AND** the publication overtime window is `24` hours
- **WHEN** Alice records overtime at `2026-05-11T11:00:00Z`
- **THEN** the overtime record is accepted

#### Scenario: Leader cannot record overtime after window

- **GIVEN** Alice is the responsible user for a shift ending at `2026-05-10T12:00:00Z`
- **AND** the publication overtime window is `24` hours
- **WHEN** Alice records overtime at `2026-05-11T12:00:01Z`
- **THEN** the request is rejected with HTTP 409 and error code `ATTENDANCE_WINDOW_CLOSED`

#### Scenario: Leader can record overtime for non-roster active user

- **GIVEN** Alice is the responsible user for a shift
- **AND** Carol is an active user who is not in the shift's roster
- **WHEN** Alice records overtime for Carol inside the overtime window
- **THEN** the overtime record is accepted

### Requirement: Admin attendance management can correct arrivals

Administrators SHALL be able to view attendance for a publication by occurrence date and shift, including current actual roster users, derived statuses, matching arrival rows, and orphan arrival rows that no longer match the current actual roster after schedule changes.

Administrators SHALL be able to create, modify, or clear arrival rows for current actual roster users at any time while the referenced publication and occurrence remain valid. Admin arrival corrections SHALL NOT require the leader window to be open. Clearing an arrival SHALL remove the arrival row, causing status to derive as `pending` or `absent`.

#### Scenario: Admin adjusts arrival after shift end

- **GIVEN** Bob is a current actual roster user for an ended occurrence
- **WHEN** an admin sets Bob's `arrived_at` through the admin attendance endpoint
- **THEN** the arrival row is created or updated
- **AND** Bob's derived status is recalculated from the new arrival time

#### Scenario: Admin clears mistaken arrival

- **GIVEN** Bob has an arrival row for an ended occurrence
- **WHEN** an admin clears Bob's arrival
- **THEN** the arrival row is removed
- **AND** Bob's derived status becomes `absent`

#### Scenario: Admin sees orphan arrival

- **GIVEN** Alice has an arrival row for assignment `A` on an occurrence
- **AND** assignment `A` is later reassigned to Bob for that occurrence
- **WHEN** an admin views the shift attendance detail
- **THEN** Bob appears in the current roster section
- **AND** Alice's old arrival appears in an orphan records section

### Requirement: Admin attendance management can manage overtime

Administrators SHALL be able to create overtime for any active user and to update or delete any overtime record in the publication attendance management UI. Admin overtime writes SHALL NOT be constrained by the leader overtime entry window.

#### Scenario: Admin creates overtime outside leader window

- **GIVEN** a shift's leader overtime entry window has expired
- **AND** Bob is an active user
- **WHEN** an admin creates an overtime row for Bob in that shift
- **THEN** the overtime row is accepted

#### Scenario: Admin updates overtime hours and note

- **GIVEN** an overtime row exists with `hours = 1.00`
- **WHEN** an admin changes it to `hours = 1.50` with a new note
- **THEN** the overtime row stores the new hours and note

#### Scenario: Admin deletes overtime

- **GIVEN** an overtime row exists
- **WHEN** an admin deletes it
- **THEN** the overtime row is removed from attendance reads

### Requirement: Attendance error code catalog

The attendance subsystem SHALL emit the following JSON `error.code` values with the HTTP statuses given:

- `INVALID_REQUEST` (400) — malformed body/path/query, invalid decimal hours, blank note, invalid arrival time, or invalid overtime window.
- `INVALID_OCCURRENCE_DATE` (400) — occurrence date outside the publication active window, mismatched weekday, or invalid date for the requested slot.
- `PUBLICATION_NOT_FOUND` (404) — publication is missing.
- `TEMPLATE_SLOT_NOT_FOUND` (404) — slot is missing for the publication template.
- `USER_NOT_FOUND` (404) — target user is missing or not active for an operation requiring an active user.
- `ATTENDANCE_RECORD_NOT_FOUND` (404) — referenced attendance or overtime record is missing.
- `ATTENDANCE_NOT_LEADER` (403) — caller is not the responsible user for the concrete shift occurrence.
- `ATTENDANCE_WINDOW_CLOSED` (409) — leader arrival or overtime write is outside the permitted window.
- `ATTENDANCE_ALREADY_RECORDED` (409) — leader attempts to record an already recorded arrival.
- `ATTENDANCE_ROSTER_STALE` (409) — write target no longer matches the current actual roster.
- `ATTENDANCE_RESPONSIBLE_REQUIRED` (409) — a template or publication lacks exactly one valid responsible position per slot where required.
- `INTERNAL_ERROR` (500) — unexpected failures.

#### Scenario: Non-leader error maps to 403

- **WHEN** a caller who is not the responsible user submits a leader attendance write
- **THEN** the response is HTTP 403 with error code `ATTENDANCE_NOT_LEADER`

#### Scenario: Existing arrival error maps to 409

- **WHEN** a leader records an arrival that already exists
- **THEN** the response is HTTP 409 with error code `ATTENDANCE_ALREADY_RECORDED`

