## Why

Shift leaders need a lightweight way to record who actually arrived for each concrete shift occurrence, including last-minute overtime work, without changing the published roster after the fact. Administrators also need a controlled correction surface so missed leader actions, mistaken arrivals, and overtime entries can be fixed with audit history.

## What Changes

- Add attendance tracking for concrete roster occurrences, keyed to the current publication roster after assignment overrides.
- Add an attendance-responsible marker to template slot position entries so each slot has one unambiguous leader position.
- Require responsible slot positions to have `required_headcount = 1`; publications whose templates lack exactly one responsible position per slot cannot expose leader attendance entry.
- Add publication-level overtime entry window configuration, defaulting to 24 hours, editable by administrators from the publication attendance management area.
- Add leader attendance entry at `/attendance`: leaders can record arrivals only for their currently running responsible shift, and can record overtime for that shift until its configured overtime window expires.
- Model arrivals as locked records: once a leader records a user's arrival for an occurrence, the leader cannot change it; administrators can adjust or clear it.
- Derive attendance status from roster, shift time, and arrival data: pending before shift end, present or late when arrival exists, absent after shift end when no arrival exists.
- Add independent overtime records that store user, hours, required note, and shift context without modifying roster assignments or occurrence overrides.
- Add administrator attendance management at `/publications/:publicationId/attendance` for viewing shifts by date, correcting arrivals, managing overtime records, and seeing records no longer matching the current roster.
- Emit audit events for leader arrival/overtime recording, administrator corrections, overtime changes, and attendance settings updates.

## Non-goals

- No attendance export, payroll export, wage calculation, or work-hour settlement logic.
- No lateness grace-period configuration; any tolerance remains a later export/reporting concern.
- No employee self check-in, geofencing, device binding, QR code, WebSocket, push, SMS, or real-time notification workflow.
- No backup leader or multi-leader model; a shift has exactly one responsible position entry.
- No automatic excused/leave-status attendance handling; leave and coverage continue to affect the actual roster via existing occurrence overrides.
- No automatic migration that guesses production responsible positions from position names.

## Capabilities

### New Capabilities

- `attendance`: Arrival and overtime recording for concrete shift occurrences, leader/admin authorization, derived attendance status, and administrator correction behavior.

### Modified Capabilities

- `scheduling`: Template slot position entries gain an attendance-responsible marker, and publications gain an overtime-entry window setting.
- `audit`: The audit taxonomy expands to cover attendance arrival, overtime, and attendance settings mutations.
- `frontend-shell`: Authenticated navigation and publication pages expose attendance entry and management routes.

## Impact

- Backend model/repository/service/handler layers for attendance records, overtime records, publication attendance views, and publication attendance settings.
- PostgreSQL goose migrations with Up and Down SQL for new attendance tables, indexes, template slot position marker, and publication overtime window column.
- Existing template, publication, roster, and assignment read paths remain authoritative for occurrence validity and actual roster membership.
- Frontend routes, i18n, query helpers, leader attendance UI, administrator attendance management UI, and tests.
- No new external runtime dependency is expected.
