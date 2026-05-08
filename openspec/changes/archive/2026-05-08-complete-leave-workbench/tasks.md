## 1. Backend Leave Pool Read Model

- [x] 1.1 Confirm implementation starts on branch `codex/complete-leave-workbench` and that no unrelated working-tree changes are mixed in. Verify: `git status --short --branch`
- [x] 1.2 Add backend model/service response types for leave pool rows, pagination metadata, display names, shift context, urgency metadata, and row action affordances without changing the database schema. Verify: `cd backend && go test ./internal/model ./internal/service`
- [x] 1.3 Implement repository queries for the leave pool read model, including visibility filters, state filters, total count, and required joins for requester/counterpart/substitute names plus slot/position context. Verify: `cd backend && go test -tags=integration ./internal/repository -run Leave`
- [x] 1.4 Add repository tests for public visibility, direct visibility, admin visibility, state filtering, total count, and sorting. Verify: `cd backend && go test -tags=integration ./internal/repository -run LeavePool`
- [x] 1.5 Implement `LeaveService.ListPool` or equivalent service method that normalizes pool pagination, computes urgency/action affordances, and keeps write endpoints authoritative for final validation. Verify: `cd backend && go test ./internal/service -run Leave`
- [x] 1.6 Add service tests for row action behavior: qualified claim, unqualified visible-disabled public leave, requester cancel, counterpart approve/reject, admin view-only, completed substitute display, and invalid state/pagination rejection. Verify: `cd backend && go test ./internal/service -run LeavePool`

## 2. Backend API and Preview Changes

- [x] 2.1 Add `GET /leaves/pool` handler/route, response serialization, query parsing, and error mapping. Verify: `cd backend && go test ./internal/handler -run Leave`
- [x] 2.2 Add handler tests for pool success, invalid state, invalid pagination, employee visibility, admin view-only serialization, and `total_count` metadata. Verify: `cd backend && go test ./internal/handler -run LeavePool`
- [x] 2.3 Extend `GET /users/me/leaves/preview` responses with direct coverage candidates for each occurrence. Verify: `cd backend && go test ./internal/service ./internal/handler -run Leave`
- [x] 2.4 Add service/handler tests proving direct candidates include active qualified non-requesters and exclude the requester plus unqualified users. Verify: `cd backend && go test ./internal/service ./internal/handler -run LeavePreview`
- [x] 2.5 Extend leave detail/list response serialization with display names, substitute name, and scheduling context while preserving existing fields. Verify: `cd backend && go test ./internal/handler -run Leave`
- [x] 2.6 Add tests for detail/list name rendering and completed substitute labeling. Verify: `cd backend && go test ./internal/service ./internal/handler -run Leave`

## 3. Leave Email Behavior

- [x] 3.1 Update shift-change email rendering so leave-bearing requests use leave-specific copy and `/leaves/{leave_id}` links while regular shift changes still link to `/requests`. Verify: `cd backend && go test ./internal/email`
- [x] 3.2 Ensure direct leave creation enqueues a counterpart email after the leave id is available; public leave creation still sends no broadcast email. Verify: `cd backend && go test ./internal/service -run Leave`
- [x] 3.3 Add service/email tests for direct leave creation email, public leave no-email, direct leave approve/reject resolution email, public leave claim resolution email, and regular shift-change email regression. Verify: `cd backend && go test ./internal/service ./internal/email -run 'Leave|ShiftChange'`

## 4. Frontend API Types and Queries

- [x] 4.1 Add frontend TypeScript types for leave pool responses, row actions, direct candidates, enriched leave details, and pagination metadata. Verify: `cd frontend && pnpm lint`
- [x] 4.2 Add query/mutation helpers for `GET /leaves/pool` and update existing leave preview/detail query helpers for added fields. Verify: `cd frontend && pnpm test -- queries`
- [x] 4.3 Add query tests for pool query parameters, total-count response shape, and preview candidate parsing. Verify: `cd frontend && pnpm test -- queries`

## 5. Frontend Leave Workbench

- [x] 5.1 Rebuild `/leaves` as the leave workbench: request CTA, status filters, leave pool table, urgency display, counterpart/substitute display, detail links, and previous/next pagination. Verify: `cd frontend && pnpm test -- leaves`
- [x] 5.2 Wire row actions from backend affordances to existing approve/reject/cancel/claim mutations and invalidate relevant leave pool, detail, personal leaves, and unread-count queries after success. Verify: `cd frontend && pnpm test -- leaves`
- [x] 5.3 Add frontend tests for default pending filter, filter reset to page 1, pagination, urgent-row rendering, public claim action, direct approve/reject actions, own cancel action, disabled not-qualified row, admin view-only row, and detail link. Verify: `cd frontend && pnpm test -- leaves`
- [x] 5.4 Update `/leaves/new` to use occurrence-level `direct_candidates` for the specified-colleague picker while keeping each occurrence independently submitted. Verify: `cd frontend && pnpm test -- leaves`
- [x] 5.5 Add tests proving the direct picker includes only preview candidates and one row submit sends exactly one `POST /leaves`. Verify: `cd frontend && pnpm test -- leaves`
- [x] 5.6 Update `/leaves/:leaveId` to display requester/counterpart/substitute names and leave-specific action/outcome language. Verify: `cd frontend && pnpm test -- leaves`

## 6. Documentation and OpenSpec

- [x] 6.1 Update README leave workflow documentation to describe the leave workbench, public pool, direct coverage, and pending responsibility rule. Verify: `rg -n "Leave Workflow|leave workbench|public" README.md`
- [x] 6.2 Validate this change's OpenSpec artifacts. Verify: `openspec validate complete-leave-workbench --strict`
- [x] 6.3 Validate the full OpenSpec tree. Verify: `openspec validate --all --strict`

## 7. Final Verification

- [x] 7.1 Run backend unit/build/vet checks. Verify: `cd backend && go build ./... && go vet ./... && go test ./...`
- [x] 7.2 Run backend integration tests for SQL-touching leave pool changes. Verify: `cd backend && go test -tags=integration ./...`
- [x] 7.3 Run frontend checks. Verify: `cd frontend && pnpm lint && pnpm test && pnpm build`
- [x] 7.4 Confirm all task checkboxes are completed before reporting apply done. Verify: `! rg -n "^- \\[ \\]" openspec/changes/complete-leave-workbench/tasks.md`
