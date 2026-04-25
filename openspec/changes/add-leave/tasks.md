## 1. Database migration

- [ ] 1.1 Add migration `migrations/0001N_add_leaves_table.sql` (next sequential after Phase 1's migration). Up creates `leaves` table per design D-8, adds `shift_change_requests.leave_id` column with FK and `ON DELETE SET NULL`, creates the `leaves_user_id_idx`, `leaves_publication_id_idx`, and `shift_change_requests_leave_id_idx` indexes. Down drops them in reverse order. Verify: `make migrate-down && make migrate-up && make migrate-down` clean; CI's `migrations-roundtrip` exercises Up/Down.

## 2. Backend model layer

- [ ] 2.1 Add `backend/internal/model/leave.go` with the `Leave` struct (`ID, UserID, PublicationID, ShiftChangeRequestID, Category LeaveCategory, Reason, CreatedAt, UpdatedAt`) and a `LeaveCategory` enum string type with constants `LeaveCategorySick`, `LeaveCategoryPersonal`, `LeaveCategoryBereavement`. Verify: `cd backend && go build ./...`.
- [ ] 2.2 Add `LeaveStateFromSCRT(state ShiftChangeState) LeaveState` helper computing the derived state per the table in *Leave state derivation*. `LeaveState` is its own enum: `pending|completed|failed|cancelled`. Verify: unit test covers all six SCRT-state mappings.
- [ ] 2.3 Update `model.ShiftChangeRequest` to include `LeaveID *int64` (nullable). Adjust JSON tags. Verify: `go build ./...`.

## 3. Backend repository layer

- [ ] 3.1 Add `backend/internal/repository/leave.go` with: `Insert(tx, leave)`, `GetByID(ctx, id) (*Leave, *ShiftChangeRequest, error)` returning the leave plus its joined SCRT, `ListForUser(ctx, userID, page, pageSize)`, `ListForPublication(ctx, pubID, page, pageSize)`. Verify: integration tests for each method with `go test -tags=integration ./internal/repository -run Leave`.
- [ ] 3.2 Update `backend/internal/repository/shift_change.go` to include `leave_id` in INSERT/SELECT/UPDATE. The existing `apply` repository helpers already do the override write; this task just propagates the new column through CRUD. Verify: `go build ./...`; existing repository tests pass.
- [ ] 3.3 Update the activation-expiry SQL to include `AND leave_id IS NULL`. Identify the call site (`repository/shift_change.go` or a service helper) and change the query. Verify: integration test creates a `PUBLISHED` publication with one regular SCRT and one leave-bearing SCRT (manually injected via SQL), calls activate, asserts only the regular SCRT is `expired` and the leave-bearing SCRT remains `pending`.

## 4. Backend service layer

- [ ] 4.1 Refactor `ShiftChangeService.CreateRequest` into:
    - public `CreateRequest(ctx, input) (*ShiftChangeRequest, error)` — opens its own transaction, asserts effective state `PUBLISHED`, asserts `input.LeaveID == nil`, calls `CreateRequestTx`.
    - internal `CreateRequestTx(ctx, tx, input *CreateRequestInput) (*ShiftChangeRequest, error)` — performs all SCRT validation (occurrence date, ownership, qualification, self-target, etc.) and inserts the row; trusts the caller for the publication-state gate.

  Verify: existing shift-change handler tests still pass; new unit tests cover `LeaveID != nil` rejection on the public method.
- [ ] 4.2 Add `backend/internal/service/leave.go` with `LeaveService`:
    - `Create(ctx, input CreateLeaveInput) (*Leave, error)` — opens a tx; asserts there is exactly one publication and its effective state is `ACTIVE`; rejects `swap`; calls `ShiftChangeService.CreateRequestTx` with `LeaveID` reserved (insert leave first, then SCRT with leave_id set, OR insert SCRT then leave then UPDATE SCRT.leave_id — pick whichever fits the FK direction; design D-8 has `ON DELETE SET NULL`, so SCRT can reference leave); audits `leave.create`.
    - `Cancel(ctx, leaveID, userID) error` — fetches leave + SCRT; rejects `LEAVE_NOT_FOUND` / `LEAVE_NOT_OWNER`; if SCRT pending, calls `ShiftChangeService.Cancel`; audits `leave.cancel`; if SCRT terminal, returns no-op success without audit.
    - `GetByID(ctx, leaveID) (*LeaveDetail, error)` — returns leave + joined SCRT.
    - `ListForUser(ctx, userID, page, pageSize) ([]*LeaveDetail, error)`.
    - `ListForPublication(ctx, pubID, page, pageSize) ([]*LeaveDetail, error)` — admin only at handler.
    - `PreviewOccurrences(ctx, userID, from, to) ([]*OccurrencePreview, error)` — finds current ACTIVE publication; returns viewer's future occurrences in `[from, to]`.

  Verify: unit and integration tests for each method (success path + at least one rejection per method).
- [ ] 4.3 Wire `LeaveService` and `ShiftChangeService` together in the dependency-injection wiring (likely `backend/cmd/server/main.go` or wherever services are constructed).
- [ ] 4.4 Add audit action constants `audit.ActionLeaveCreate` and `audit.ActionLeaveCancel`. Update the audit capability's action catalog spec entry if it already enumerates known actions. Verify: `go build ./...`; audit tests adapted.

## 5. Backend handler layer

- [ ] 5.1 Add `backend/internal/handler/leave.go` with `LeaveHandler` and these routes:
    - `POST /leaves` (RequireAuth) — body: `{ assignment_id, occurrence_date, type, counterpart_user_id?, category, reason? }`; returns 201 with `{ id, share_url, ... }`.
    - `GET /leaves/{id}` (RequireAuth).
    - `POST /leaves/{id}/cancel` (RequireAuth).
    - `GET /users/me/leaves` (RequireAuth).
    - `GET /users/me/leaves/preview?from=&to=` (RequireAuth).
    - `GET /publications/{id}/leaves` (RequireAdmin).
  Verify: handler unit tests cover each route's success path and one rejection per documented error code.
- [ ] 5.2 Register the routes in the router (likely `backend/internal/handler/router.go` or wherever the route table lives). Verify: `go build ./...`; the new routes resolve correctly.
- [ ] 5.3 Map new error codes (`LEAVE_NOT_FOUND`, `LEAVE_NOT_OWNER`) in the handler error mapping (likely `backend/internal/handler/error.go`). Verify: handler tests exercise both codes.

## 6. Frontend

- [ ] 6.1 Add a `Leave` page accessible from the employee navigation. Layout:
    - Date range picker (`from`, `to`).
    - On range change, call `GET /users/me/leaves/preview` and render one row per occurrence.
    - Each row has fields for `type` (`give_direct` with optional counterpart picker, or `give_pool`), `category` dropdown, `reason` text input, and a per-row submit button.
    - On submit, call `POST /leaves` with the row's payload; on success, store the returned `share_url`.
    - Show the list of `share_url`s after successful submissions, with a "copy" button per URL.
  Verify: `pnpm lint && pnpm test && pnpm build` clean; manual smoke test confirms the flow.
- [ ] 6.2 Add a `LeaveDetail` page at `/leaves/:id`. Render leave metadata (category, reason, requester, occurrence date, slot/position) plus the underlying SCRT (current state, counterpart, expires_at). Surface action buttons based on the SCRT layer's authorization (existing logic): approve/reject for counterpart, cancel for requester, claim for any qualified user on `give_pool`.
  Verify: same.
- [ ] 6.3 Add a `MyLeaves` page reading `GET /users/me/leaves` with pagination. Verify: same.
- [ ] 6.4 Add the leave entry-point to the employee menu / navigation. Verify: same.
- [ ] 6.5 Update `frontend/src/lib/api-error.ts` (or wherever) to map `LEAVE_NOT_FOUND` and `LEAVE_NOT_OWNER` to user-friendly messages. Verify: existing API-error tests adapted.

## 7. Seed updates

- [ ] 7.1 Optional: extend `backend/cmd/seed/scenarios/stress.go` to insert a couple of in-flight leaves on the ACTIVE publication so the admin UI shows realistic data. Skip if scope feels excessive; the smoke test in §8 will exercise the create path manually. Verify: `make seed SCENARIO=stress` runs; `psql -c "SELECT count(*) FROM leaves"` shows expected count.

## 8. Documentation

- [ ] 8.1 Update `README.md` with a short "Leave workflow" sub-section under the existing scheduling overview, pointing at the leave page and noting that `/leaves/:id` is the share URL. Verify: render locally.

## 9. Final verification

- [ ] 9.1 Backend clean: `cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./... && govulncheck ./...`.
- [ ] 9.2 Frontend clean: `cd frontend && pnpm lint && pnpm test && pnpm build`.
- [ ] 9.3 Migrations roundtrip clean: `make migrate-down && make migrate-up && make migrate-down && make migrate-up`.
- [ ] 9.4 Smoke test (manual):
    - (a) `make migrate-down && make migrate-up && make seed SCENARIO=full`. Activate the publication. Login as `employee1@example.com`/`pa55word`. Create a `give_pool` leave for a future occurrence with category `personal` and reason `"考试"`. Verify response carries `share_url`.
    - (b) Open the share URL in another browser session as another employee; verify the leave detail page renders with a "claim" button. Click claim; verify the leave's derived state becomes `completed` and the override is visible on the roster only for that week.
    - (c) Create another leave, then cancel it via `POST /leaves/{id}/cancel`. Verify SCRT becomes `cancelled` and leave's derived state is `cancelled`.
    - (d) Try `POST /leaves` with `type = 'swap'`. Verify HTTP 400 `SHIFT_CHANGE_INVALID_TYPE`.
    - (e) Login as admin, hit `GET /publications/{id}/leaves`. Verify the list contains the leaves created in (a)-(c).
- [ ] 9.5 Confirm CI is green on the `change/add-leave` branch: `backend-test`, `frontend-test`, `migrations-roundtrip`, `docker-build`, `govulncheck`.
