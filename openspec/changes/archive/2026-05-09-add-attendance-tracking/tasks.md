## 1. Schema and Models

- [x] 1.1 Add goose migration `00021_add_attendance_tracking.sql` with `attendance_responsible`, `overtime_entry_window_hours`, `attendance_records`, `attendance_overtime_records`, indexes, constraints, and Down SQL. Verify: `make migrate-up && make migrate-down && make migrate-up`.
- [x] 1.2 Add backend model types and error sentinels for attendance records, overtime records, derived status, attendance windows, and attendance errors. Verify: `cd backend && go test ./internal/model`.
- [x] 1.3 Add repository integration tests for migration constraints: one responsible position per slot, responsible headcount equals one, attendance arrival uniqueness, overtime hour/note constraints, and cascade behavior. Verify: `cd backend && go test -tags=integration ./internal/repository -run Attendance`.
- [x] 1.4 Implement repository methods for attendance shift reads, arrival insert/admin upsert/delete, overtime create/update/delete, orphan arrival reads, and publication attendance settings. Verify: `cd backend && go test -tags=integration ./internal/repository -run Attendance`.

## 2. Scheduling Integration

- [x] 2.1 Extend template slot-position request/response models, handlers, services, repositories, and frontend schemas to carry `attendance_responsible`. Verify: `cd backend && go test ./internal/handler ./internal/service -run Template && cd frontend && pnpm test -- template`.
- [x] 2.2 Add service validation tests for responsible marker writes: reject headcount > 1, reject second responsible entry, expose marker in template detail, and allow non-responsible entries. Verify: `cd backend && go test ./internal/service -run Template`.
- [x] 2.3 Implement publication `overtime_entry_window_hours` read/update support in backend models, repository, service, handler, response serialization, and frontend types. Verify: `cd backend && go test ./internal/handler ./internal/service -run Publication`.
- [x] 2.4 Add tests for overtime window defaulting, invalid values, and attendance settings update behavior. Verify: `cd backend && go test ./internal/service ./internal/handler -run AttendanceSettings`.
- [x] 2.5 Update seed scenarios so demo slot positions named with `负责人` are marked responsible, without production migration guessing. Verify: `cd backend && go test ./cmd/seed ./cmd/seed/scenarios`.

## 3. Attendance Backend

- [x] 3.1 Implement `AttendanceService` read model for leader current shifts and admin shift details, deriving actual users from `assignments + assignment_overrides` and deriving `pending/present/late/absent` status. Verify: `cd backend && go test ./internal/service -run AttendanceRead`.
- [x] 3.2 Add service tests for derived statuses, occurrence override resolution, no early display before scheduled start, current shift visibility, overtime-window visibility, and orphan arrival detection. Verify: `cd backend && go test ./internal/service -run AttendanceRead`.
- [x] 3.3 Implement leader arrival recording with responsible-user authorization, arrival window enforcement, default scheduled-start arrival, stale roster rejection, active publication gating, and write-once behavior. Verify: `cd backend && go test ./internal/service -run AttendanceArrival`.
- [x] 3.4 Add service tests for leader arrival success, late arrival, duplicate arrival rejection, non-leader rejection, after-end rejection, future arrival rejection, and stale roster rejection. Verify: `cd backend && go test ./internal/service -run AttendanceArrival`.
- [x] 3.5 Implement leader overtime recording with responsible-user authorization, active target-user validation, publication overtime window enforcement, decimal hour validation, required note, multiple rows, and save-as-final behavior. Verify: `cd backend && go test ./internal/service -run AttendanceOvertime`.
- [x] 3.6 Add service tests for leader overtime during shift, leader overtime after shift within window, expired-window rejection, active non-roster user acceptance, blank note rejection, and hours > 24 rejection. Verify: `cd backend && go test ./internal/service -run AttendanceOvertime`.
- [x] 3.7 Implement admin arrival correction and overtime management service methods, unconstrained by leader windows but still validating publication/occurrence/current roster or active target user as specified. Verify: `cd backend && go test ./internal/service -run AttendanceAdmin`.
- [x] 3.8 Add service tests for admin arrival upsert, admin clear, admin create/update/delete overtime, admin outside leader window, and current-roster-only arrival correction. Verify: `cd backend && go test ./internal/service -run AttendanceAdmin`.

## 4. Attendance HTTP API

- [x] 4.1 Add attendance handlers and route registration for `GET /attendance/current`, leader arrival/overtime writes, admin publication attendance reads, admin arrival correction, admin overtime management, and attendance settings update. Verify: `cd backend && go test ./internal/handler -run Attendance`.
- [x] 4.2 Add handler tests for auth/admin gating, JSON request validation, error-code mapping, successful leader arrival/overtime responses, and successful admin correction responses. Verify: `cd backend && go test ./internal/handler -run Attendance`.
- [x] 4.3 Wire attendance services in `cmd/server`, including repositories, audit middleware context usage, and route registration. Verify: `cd backend && go test ./cmd/server && go build ./...`.

## 5. Audit

- [x] 5.1 Add audit constants for attendance target types and actions, and emit audit events from every successful attendance/settings mutation. Verify: `cd backend && go test ./internal/audit ./internal/service -run Attendance`.
- [x] 5.2 Add audit tests covering leader arrival, admin arrival adjust/clear, leader overtime, admin overtime create/adjust/delete, settings update, no audit on failed writes, and no note text in audit metadata. Verify: `cd backend && go test ./internal/service -run AttendanceAudit`.
- [x] 5.3 Update repository audit target-type tests to accept `attendance_record` and `attendance_overtime`. Verify: `cd backend && go test ./internal/audit ./internal/repository -run Audit`.

## 6. Frontend Leader Attendance

- [x] 6.1 Add frontend attendance API helpers, query options, schemas/types, i18n keys, and route generation for `/attendance`. Verify: `cd frontend && pnpm test -- attendance`.
- [x] 6.2 Implement `/attendance` leader page with empty state, eligible shift cards, derived status display, arrival action with scheduled-start default, locked arrival display, overtime entry, and backend error handling. Verify: `cd frontend && pnpm test -- attendance`.
- [x] 6.3 Add component/route tests for current shift display, no early shift display, non-leader empty state, arrival default time, recorded arrival lock, overtime note requirement, and expired-window UI. Verify: `cd frontend && pnpm test -- attendance`.
- [x] 6.4 Add the `Attendance` sidebar item under My schedule and update sidebar tests and localization. Verify: `cd frontend && pnpm test -- app-sidebar`.

## 7. Frontend Admin Attendance

- [x] 7.1 Add `/publications/:publicationId/attendance` route, publication detail navigation affordance, route generation, and admin gating. Verify: `cd frontend && pnpm test -- publications`.
- [x] 7.2 Implement admin attendance management page with date selection, shift list, shift detail, arrival correction controls, overtime create/edit/delete controls, orphan record area, and overtime window settings form. Verify: `cd frontend && pnpm test -- attendance`.
- [x] 7.3 Add frontend tests for admin date shift listing, arrival set/change/clear controls, overtime create/edit/delete, settings update, orphan record display, and non-admin denial. Verify: `cd frontend && pnpm test -- attendance publications`.

## 8. Final Verification

- [x] 8.1 Run OpenSpec status/validation for the completed change. Verify: `openspec status --change add-attendance-tracking`.
- [x] 8.2 Run backend unit checks. Verify: `cd backend && go test ./... && go vet ./... && go build ./...`.
- [x] 8.3 Run backend integration checks for SQL changes. Verify: `cd backend && go test -tags=integration ./...`.
- [x] 8.4 Run frontend checks. Verify: `cd frontend && pnpm lint && pnpm test && pnpm build`.
- [x] 8.5 Confirm every task is ticked before archive. Verify: `rg \"- \\[ \\]\" openspec/changes/add-attendance-tracking/tasks.md`.
