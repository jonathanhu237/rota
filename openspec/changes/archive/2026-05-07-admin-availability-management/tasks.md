## 1. Backend API And Service

- [x] 1.1 Add repository query models and SQL methods for the admin availability board, employee detail payload, and per-user availability diff/replacement helpers; verify with `cd backend && go test ./internal/repository`.
- [x] 1.2 Add publication service DTOs and read methods for paginated/searchable admin availability board data and single-employee availability detail; verify with `cd backend && go test ./internal/service -run TestPublicationServiceAdminAvailabilityRead`.
- [x] 1.3 Implement the atomic admin availability replacement service method, including state gate, target user filtering, template cell validation, qualification validation, duplicate normalization, empty-set clearing, and transaction rollback on failure; verify with `cd backend && go test ./internal/service -run TestPublicationServiceAdminAvailabilityReplace`.
- [x] 1.4 Add HTTP handlers and routes for `GET /publications/{id}/availability-board`, `GET /publications/{id}/availability-submissions/{user_id}`, and `PUT /publications/{id}/availability-submissions/{user_id}` with strict request decoding and mapped error codes; verify with `cd backend && go test ./internal/handler -run TestPublicationHandlerAdminAvailability`.

## 2. Backend Tests And Audit

- [x] 2.1 Add success-path service tests for board inclusion of zero-submission employees, search/pagination, detail eligibility payloads, replacement in `COLLECTING`, and replacement in `ASSIGNING`; verify with `cd backend && go test ./internal/service -run TestPublicationServiceAdminAvailability`.
- [x] 2.2 Add rejection-path service tests for disabled users, irrelevant users, ineligible final cells, invalid template cells, non-mutable publication states, and atomic rollback; verify with `cd backend && go test ./internal/service -run TestPublicationServiceAdminAvailability`.
- [x] 2.3 Add audit action constants and emit one `availability.admin.create` or `availability.admin.delete` event per changed cell only after successful replacement; verify with `cd backend && go test ./internal/audit ./internal/service -run 'Test.*Availability.*Audit'`.
- [x] 2.4 Add handler tests for admin-only access, invalid body/query handling, empty replacement, and response shape; verify with `cd backend && go test ./internal/handler -run TestPublicationHandlerAdminAvailability`.

## 3. Frontend Implementation

- [x] 3.1 Refactor publication file routes so the parent route is a thin layout and publication detail, assignments, availability, availability editor, and shift changes render as standalone subpages; verify with `cd frontend && pnpm test -- --run publications`.
- [x] 3.2 Add frontend API types, query keys, board query, detail query, and replacement mutation for the admin availability endpoints; verify with `cd frontend && pnpm test -- --run queries`.
- [x] 3.3 Build the admin availability table with search, pagination, qualification chips, submitted count, zero-submission employees, and navigation to the editor; verify with `cd frontend && pnpm test -- --run availability`.
- [x] 3.4 Build the single-employee availability editor with the template grid, eligible editable cells, ineligible disabled cells, removable ineligible-submitted exceptions, local draft state, dirty navigation prompt, discard, save, success toast, and read-only states; verify with `cd frontend && pnpm test -- --run availability`.
- [x] 3.5 Add publication detail and assignment-board entry actions that navigate to availability management; verify with `cd frontend && pnpm test -- --run assignments`.
- [x] 3.6 Add all new zh/en locale strings and route labels without hardcoded user-facing text; verify with `cd frontend && pnpm lint`.

## 4. Frontend Tests

- [x] 4.1 Add route/layout tests proving assignments, availability, editor, and shift-change pages do not render beneath the publication detail card; verify with `cd frontend && pnpm test -- --run publications`.
- [x] 4.2 Add availability table tests for search resetting to page 1, pagination requests, zero-submission rows, and editor navigation; verify with `cd frontend && pnpm test -- --run availability`.
- [x] 4.3 Add editor tests for read-only states, dirty prompt, successful save refresh, discard, ineligible cell blocking, and clearing ineligible submitted exceptions; verify with `cd frontend && pnpm test -- --run availability`.

## 5. Final Verification

- [x] 5.1 Run backend unit/build checks with `cd backend && go build ./... && go vet ./... && go test ./...`.
- [x] 5.2 Because this change touches SQL queries, run backend integration checks with `make test-integration`.
- [x] 5.3 Run frontend checks with `cd frontend && pnpm lint && pnpm test && pnpm build`.
- [x] 5.4 Validate the OpenSpec change with `openspec validate admin-availability-management --strict`.
