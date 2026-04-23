> **Nuke your local DB before starting.** This refactor drops `template_shifts` and rewrites `assignments`. Run `make migrate-down` first, then the new migration will build the schema from scratch.

## 1. Database migration

- [x] 1.1 Create goose migration file `backend/migrations/NNNN_slot_position_model.sql` containing `Up`: `CREATE EXTENSION IF NOT EXISTS btree_gist;`, `CREATE TABLE template_slots` with CHECK / UNIQUE / GIST-exclude, `CREATE INDEX template_slots_template_weekday_idx`, `CREATE TABLE template_slot_positions` with UNIQUE and CHECK, `DROP TABLE assignments`, `CREATE TABLE assignments` (new shape) with trigger + trigger function, availability_submissions column swap (drop `template_shift_id`, add `slot_id` + `position_id` + UNIQUE), finally `DROP TABLE template_shifts`. `Down` reverses. Verify: `make migrate-up && make migrate-down && make migrate-up`.
- [x] 1.2 Add an integration test in `backend/internal/repository/template_slot_db_test.go` that confirms the GIST exclusion constraint rejects overlapping slots in the same `(template, weekday)`. Verify: `POSTGRES_HOST=localhost POSTGRES_PORT=${POSTGRES_PORT:-5433} POSTGRES_USER=rota POSTGRES_PASSWORD=pa55word POSTGRES_DB=rota go test -tags=integration ./internal/repository -run TemplateSlot -count=1`.
- [x] 1.3 Add an integration test proving the `assignments_position_belongs_to_slot` trigger rejects an insert where `(slot_id, position_id)` is not in `template_slot_positions`. Same verify command as 1.2 with `-run AssignmentPositionTrigger`.
- [x] 1.4 Add an integration test proving `UNIQUE(publication_id, user_id, slot_id)` rejects a second assignment of the same user to the same slot (different position). Same verify command with `-run AssignmentSlotUnique`.

## 2. Backend model layer

- [x] 2.1 In `backend/internal/model/template.go`: remove `TemplateShift`, add `TemplateSlot { id, template_id, weekday, start_time, end_time, created_at, updated_at }` and `TemplateSlotPosition { id, slot_id, position_id, required_headcount, created_at, updated_at }`. Verify: `cd backend && go build ./...`.
- [x] 2.2 In `backend/internal/model/publication.go` (or wherever `Assignment` lives): change `Assignment` from `{ id, publication_id, user_id, template_shift_id, created_at }` to `{ id, publication_id, user_id, slot_id, position_id, created_at }`. Update `AssignmentCandidate` / `AssignmentParticipant` to carry `slot_id` + `position_id` where previously `template_shift_id`. Verify: `cd backend && go build ./...`.
- [x] 2.3 Add sentinel errors `ErrTemplateSlotNotFound`, `ErrTemplateSlotPositionNotFound`, `ErrAssignmentTimeConflict` in the appropriate model files. Verify: `cd backend && go build ./...`.

## 3. Backend repository layer — template slot + slot-position

- [x] 3.1 In `backend/internal/repository/template.go`, replace shift CRUD with slot CRUD (`CreateSlot`, `UpdateSlot`, `DeleteSlot`, `GetSlot`, `ListSlotsByTemplate`). Remove `template_shifts`-related code. Verify: `cd backend && go build ./... && go vet ./...`.
- [x] 3.2 Add slot-position CRUD in the same repo: `CreateSlotPosition`, `UpdateSlotPosition`, `DeleteSlotPosition`, `GetSlotPosition`, `ListSlotPositions(slot_id)`. Verify: same as 3.1.
- [x] 3.3 Replace `GetTemplateShift` callers with `GetSlot` and/or `GetSlotPosition` — do a codebase-wide pass. Verify: `cd backend && go build ./...`.
- [x] 3.4 Add integration tests for slot CRUD and slot-position CRUD in `backend/internal/repository/template_slot_db_test.go` (golden-path + uniqueness + cascade). Verify: `POSTGRES_HOST=localhost POSTGRES_PORT=${POSTGRES_PORT:-5433} POSTGRES_USER=rota POSTGRES_PASSWORD=pa55word POSTGRES_DB=rota go test -tags=integration ./internal/repository -run TemplateSlot -count=1`.

## 4. Backend repository layer — assignment

- [x] 4.1 In `backend/internal/repository/assignment.go`, rewrite `CreateAssignment`, `DeleteAssignment`, `GetAssignment`, `ListPublicationAssignments`, `ListAssignmentCandidates`, `ReplaceAssignments` to use `slot_id` + `position_id`. Verify: `cd backend && go build ./... && go vet ./...`.
- [x] 4.2 Rewrite `ListQualifiedUsersForPositions` to still key by `position_id`; no shape change there, only confirm it still compiles against the new `AssignmentCandidate`. Verify: same.
- [x] 4.3 Add a new repo method `ListUserAssignmentsOnWeekdayInPublication(ctx, publicationID, userID, weekday) ([]*AssignmentSlotView, error)` that joins `assignments → template_slots` and returns the user's weekly slots (`slot_id, start_time, end_time, position_id`). Needed by the overlap check on `CreateAssignment`. Verify: `cd backend && go build ./...`.
- [x] 4.4 Update `backend/internal/repository/publication.go` — any query that joined `template_shifts` now joins `template_slots` + `template_slot_positions`. Verify: `cd backend && go build ./...`.
- [x] 4.5 Rewrite the assignment-board query in repository to produce a slot-grouped result: one row per `(slot, position)` with its candidate list, non-candidate-qualified list, and assignment list. Intermediate method shape can be a `map[slotID] SlotBoardView{positions: map[positionID] PositionBoardView{...}}`. Verify: `cd backend && go build ./...`.
- [x] 4.6 Update the shift-change repository (`repository/shift_change.go`) so any JOIN that used `template_shift_id` now uses `slot_id` (assignments still the FK carrier). `InvalidateRequestsForAssignment` is unchanged because it keys on `assignment_id`. Verify: `cd backend && go build ./...`.
- [x] 4.7 Extend `backend/internal/repository/assignment_db_test.go` (or add new) — golden-path create/delete, overlap rejection via trigger, uniqueness rejection, cascade on publication/user/slot delete. Verify: `POSTGRES_HOST=localhost POSTGRES_PORT=${POSTGRES_PORT:-5433} POSTGRES_USER=rota POSTGRES_PASSWORD=pa55word POSTGRES_DB=rota go test -tags=integration ./internal/repository -run Assignment -count=1`.

## 5. Backend service layer — publication & assignment-board

- [x] 5.1 In `backend/internal/service/publication.go` / `publication_pr4.go`: update `publicationRepository` interface — `CreateAssignment` params now carry `slot_id` + `position_id`; `GetAssignment` returns new shape. Update all implementations (stateful mocks included). Verify: `cd backend && go vet ./... && go build ./...`.
- [x] 5.2 In `CreateAssignment` service method, after the existing state / disabled / qualification gates, call the new overlap check: load the target user's other assignments for the same weekday; reject with `ErrAssignmentTimeConflict` on overlap. Emit no audit on the rejection path. Verify: `cd backend && go test ./internal/service -run CreateAssignment -count=1`.
- [x] 5.3 Rewrite `GetAssignmentBoard` to produce the slot-grouped result: `AssignmentBoardResult { Slots []*AssignmentBoardSlotResult { Slot, Positions []*AssignmentBoardPositionResult { Position, RequiredHeadcount, Candidates, NonCandidateQualified, Assignments } } }`. Understaffed computed per `(slot, position)`. Verify: `cd backend && go test ./internal/service -run AssignmentBoard -count=1`.
- [x] 5.4 Service tests for `CreateAssignment` overlap rejection: success when no conflict, 409 when same-weekday slot overlaps, success across different weekdays, success on touching boundary. Verify: same as 5.2.
- [x] 5.5 Update service tests for `DeleteAssignment` to use slot+position shape; cascade tests (from admin-shift-adjustments) continue to pass. Verify: `cd backend && go test ./internal/service -run DeleteAssignment -count=1`.
- [x] 5.6 Update the state-widening assignment tests (table-driven "allows all mutable effective states") to use slot+position shape. Verify: `cd backend && go test ./internal/service -run Assignment -count=1`.

## 6. Backend service layer — auto-assign MCMF

- [x] 6.1 Rewrite the MCMF graph construction in `publication_pr4.go` (or wherever auto-assign lives): source → user-seat → (user, slot) intermediate cap-1 → (slot, position) → sink. Capacities from `template_slot_positions.required_headcount`. Verify: `cd backend && go build ./...`.
- [x] 6.2 Keep existing auto-assign golden-case tests but adapt the fixtures to the new schema. Add one new test for "same user won't get two positions of the same slot." Verify: `cd backend && go test ./internal/service -run AutoAssign -count=1`.
- [x] 6.3 Integration test end-to-end: seed template+slots+slot_positions, run auto-assign, verify the output honors `UNIQUE(publication_id, user_id, slot_id)` and doesn't violate the per-weekday overlap rule. Verify: `POSTGRES_HOST=localhost POSTGRES_PORT=${POSTGRES_PORT:-5433} POSTGRES_USER=rota POSTGRES_PASSWORD=pa55word POSTGRES_DB=rota go test -tags=integration ./internal/service -run AutoAssign -count=1`.

## 7. Backend service layer — shift-change

- [x] 7.1 Update `shift_change.go` service methods so any reference to `template_shift_id` on an assignment becomes `(slot_id, position_id)`. The `(requester, counterpart)` qualification check still matches by `position_id`. The existing time-conflict check on apply uses slot time — adapt the join path. Verify: `cd backend && go test ./internal/service -run ShiftChange -count=1`.
- [x] 7.2 Update `BuildShiftChangeResolvedMessage` and its friends to accept slot-based data; the email bodies already reference an abstract "shift" phrasing and do not need to change textually. Verify: `cd backend && go test ./internal/email -count=1`.
- [x] 7.3 Cascade invalidation tests (from admin-shift-adjustments) continue to pass with slot-based assignments. Verify: `cd backend && go test ./internal/service -run DeleteAssignment -count=1`.

## 8. Backend handler layer

- [x] 8.1 In `backend/internal/handler/publication.go`: update the assignment-board response shape to `{ slots: [{ slot, positions: [{ position, required_headcount, candidates, non_candidate_qualified, assignments }] }] }`. Verify: `cd backend && go test ./internal/handler -run AssignmentBoard -count=1`.
- [x] 8.2 Update `POST /publications/{id}/assignments` body to accept `{ user_id, slot_id, position_id }`; remove the old `template_shift_id` field. Reject requests carrying the old field with `INVALID_REQUEST`. Verify: `cd backend && go test ./internal/handler -count=1`.
- [x] 8.3 Map `ErrAssignmentTimeConflict` → `ASSIGNMENT_TIME_CONFLICT` (HTTP 409) in `writePublicationServiceError`. Verify: `cd backend && go vet ./... && go build ./...`.
- [x] 8.4 Handler tests: roundtrip the new assignment-board response shape; assert the shape includes `slots[].positions[]`; verify the new error code mapping. Verify: `cd backend && go test ./internal/handler -count=1`.
- [x] 8.5 Roster handler: the publication roster response is now grouped by slot, with each slot listing its positions and assignees. Adapt `publicationRosterResponse`. Verify: `cd backend && go test ./internal/handler -run Roster -count=1`.
- [x] 8.6 Template / slot / slot-position handler endpoints: replace `*/shifts/*` with `*/slots/*` and add `*/slots/{slot_id}/positions/*`. Verify: `cd backend && go test ./internal/handler -run Template -count=1`.

## 9. Backend audit + error taxonomy

- [x] 9.1 In `backend/internal/audit/audit.go`: confirm no audit action names change. Update any doc comments that mentioned `template_shift_id` in metadata to `slot_id + position_id`. No code change required here — the metadata map is built at the emit site. Verify: `cd backend && go vet ./...`.
- [x] 9.2 Update the emit sites (`assignment.create`, `assignment.delete`, `shift_change.*`) in service code so metadata carries `slot_id` and `position_id` instead of `template_shift_id`. Verify: `cd backend && go test ./internal/service -count=1`.
- [x] 9.3 Add `ASSIGNMENT_TIME_CONFLICT` to any backend error-code constant / switch used for response mapping. Verify: `cd backend && go build ./...`.

## 10. Frontend — types and API client

- [x] 10.1 In `frontend/src/lib/types.ts`: remove `AssignmentBoardShift`; add `AssignmentBoardSlot { slot: PublicationSlot; positions: AssignmentBoardPosition[] }`, `AssignmentBoardPosition { position: PublicationPosition; required_headcount: number; candidates; non_candidate_qualified; assignments }`. Update `PublicationShift` → `PublicationSlot`. Verify: `cd frontend && pnpm tsc --noEmit`.
- [x] 10.2 In `frontend/src/lib/api-error.ts`: add `ASSIGNMENT_TIME_CONFLICT` to the `ApiErrorCode` union. Verify: `cd frontend && pnpm tsc --noEmit`.
- [x] 10.3 Update `frontend/src/lib/api.ts` client methods — `createAssignment({user_id, slot_id, position_id})`; roster / assignment-board response parsers. Verify: `cd frontend && pnpm tsc --noEmit`.

## 11. Frontend — components

- [x] 11.1 Rewrite `frontend/src/components/assignments/assignment-board.tsx` to render slots top-level, with each slot's position composition as a sub-grid. The existing "Show all qualified employees" toggle stays; the banners, understaffed styling stay. Verify: `cd frontend && pnpm test -- assignment-board && pnpm build`.
- [x] 11.2 Update `frontend/src/components/assignments/assignment-board-state.ts` helpers — `isAssignmentBoardShiftUnderstaffed` → `isAssignmentBoardPositionUnderstaffed` (takes `{required_headcount, assignments}`); `getVisibleNonCandidateQualified` signature updated to take a `(slot, position)` pair. Verify: `cd frontend && pnpm test -- assignment-board-state`.
- [x] 11.3 Update `frontend/src/components/requests/requests-list.tsx` to render shift-change rows referencing slot+position. Copy "shift" terminology if present in the UI strings — leave unchanged for now. Verify: `cd frontend && pnpm test -- requests-list && pnpm build`.
- [x] 11.4 Update the assignments route `frontend/src/routes/_authenticated/publications/$publicationId/assignments.tsx` — body of `POST /assignments` now sends `{user_id, slot_id, position_id}`. Verify: `cd frontend && pnpm build`.
- [x] 11.5 Any admin template-editing UI (if present) is updated from "shifts" to "slots" + "positions"; if the UI does not exist yet, skip with a note. Verify: `cd frontend && pnpm build`.

## 12. Frontend — i18n

- [x] 12.1 Add English + Chinese copy for `ASSIGNMENT_TIME_CONFLICT` under `publications.errors.*` (en: "This assignment overlaps with another slot the user is already assigned to."; zh: "该分配与用户已有的班次时间冲突。"). Verify: `cd frontend && pnpm build`.
- [x] 12.2 Verify en/zh key parity: `python3 -c "import json;a,b=[set(__import__('json').load(open(f'frontend/src/i18n/locales/{l}.json'))) for l in ('en','zh')];print(a^b)"` — empty set expected.

## 13. Main specs sync

- [x] 13.1 Note: the main spec merge happens at archive time; this task just records that the delta spec (`openspec/changes/refactor-to-slot-position-model/specs/scheduling/spec.md`) correctly declares every MODIFIED requirement with the FULL updated content and every ADDED requirement. Verify: `openspec validate refactor-to-slot-position-model --strict`.

## 14. Final verification

- [x] 14.1 `cd backend && go build ./... && go vet ./... && go test ./... && govulncheck ./...` — all clean.
- [x] 14.2 `POSTGRES_HOST=localhost POSTGRES_PORT=${POSTGRES_PORT:-5433} POSTGRES_USER=rota POSTGRES_PASSWORD=pa55word POSTGRES_DB=rota go test -tags=integration ./... -count=1` — all clean.
- [x] 14.3 `cd frontend && pnpm lint && pnpm test && pnpm build` — all clean.
- [x] 14.4 Smoke test against `docker compose -f docker-compose.prod.yml up` (after local `make migrate-up` on a fresh DB): create a template with 2 slots having different position compositions; create a publication; open COLLECTING → ASSIGNING; admin creates assignments; try to create an overlapping assignment and verify it returns 409 `ASSIGNMENT_TIME_CONFLICT`; advance to PUBLISHED → ACTIVE; admin edits an assignment; verify audit events use `slot_id + position_id`. (First attempt surfaced the two bugs fixed in Section 15; re-run after those fixes land.)

## 15. Post-smoke-test fixes

First smoke test of 14.4 surfaced two real bugs. Both are constraint-violation → error-mapping gaps: the database layer correctly rejects invalid writes, but the repository/handler layer does not translate those rejections into meaningful `409` responses. One of them is a spec-scenario violation (C1); the other is a usability regression (W1). Both must be fixed before archiving.

- [x] 15.1 **Fix C1** (same-user-same-slot silent upsert). In `backend/internal/repository/assignment.go:CreateAssignment`, remove the `ON CONFLICT (publication_id, user_id, slot_id) DO NOTHING` clause and the subsequent `sql.ErrNoRows` → `getAssignmentByKey` fall-through. Instead, let the unique-violation error propagate. Detect `*pq.Error` with `Code == "23505"` (unique_violation) on the `assignments_publication_user_slot_key` constraint; translate to a new sentinel `model.ErrAssignmentUserAlreadyInSlot`. Verify: `cd backend && go build ./... && go vet ./...`.
- [x] 15.2 **Fix C1** (service side). Alias the new sentinel in `backend/internal/service/publication.go` (and wherever `Err*` aliases live). In `publication_pr4.go:hasAssignmentTimeConflict`, remove the `if assignment.SlotID == targetSlotID { continue }` branch — a same-slot existing assignment IS a conflict (the repository's unique-violation path will also catch it as a defense-in-depth). Service returns `ErrAssignmentUserAlreadyInSlot` when it detects the same-slot case before hitting the repo (preferred, avoids the duplicate audit event), or lets the repo's unique-violation bubble up and maps it to the same sentinel. Verify: `cd backend && go test ./internal/service -run CreateAssignment -count=1`.
- [x] 15.3 **Fix C1** (handler side). Map `service.ErrAssignmentUserAlreadyInSlot` to HTTP 409 with error code `ASSIGNMENT_USER_ALREADY_IN_SLOT` in `backend/internal/handler/publication.go:writePublicationServiceError`. Add `ASSIGNMENT_USER_ALREADY_IN_SLOT` to `frontend/src/lib/api-error.ts` `ApiErrorCode` union. Verify: `cd backend && go vet ./... && cd ../frontend && pnpm tsc --noEmit`.
- [x] 15.4 **Fix C1** (tests). Add a service test in `publication_pr4_test.go` covering: "same user, same slot, different position → ErrAssignmentUserAlreadyInSlot". Add a repository integration test in `assignment_db_test.go` proving that a direct second insert with the same `(publication, user, slot)` tuple returns the mapped sentinel (not `sql.ErrNoRows`). Assert in both tests that exactly ONE audit event is emitted across the two API calls (the first create), not two. Verify: `cd backend && go test ./internal/service -run CreateAssignment -count=1 && POSTGRES_HOST=localhost POSTGRES_PORT=${POSTGRES_PORT:-5433} POSTGRES_USER=rota POSTGRES_PASSWORD=pa55word POSTGRES_DB=rota go test -tags=integration ./internal/repository -run Assignment -count=1`.
- [x] 15.5 **Fix W1** (template-slot GIST-overlap mapping). In `backend/internal/repository/template.go:CreateSlot`, detect `*pq.Error` with `Code == "23P01"` (exclusion_violation) on `template_slots` and translate to a new sentinel `model.ErrTemplateSlotOverlap`. Alias it in `service`, map to HTTP 409 `TEMPLATE_SLOT_OVERLAP` in the template handler. Add `TEMPLATE_SLOT_OVERLAP` to `frontend/src/lib/api-error.ts`. Verify: `cd backend && go build ./... && cd ../frontend && pnpm tsc --noEmit`.
- [x] 15.6 **Fix W1** (tests). Repo integration test proves the exclusion-violation maps to `ErrTemplateSlotOverlap` (not a raw `pq.Error` or `sql.ErrNoRows`). Handler test asserts the 409 `TEMPLATE_SLOT_OVERLAP` response shape. Verify: `cd backend && go test ./internal/handler -run Template -count=1 && POSTGRES_HOST=localhost POSTGRES_PORT=${POSTGRES_PORT:-5433} POSTGRES_USER=rota POSTGRES_PASSWORD=pa55word POSTGRES_DB=rota go test -tags=integration ./internal/repository -run TemplateSlot -count=1`.
- [x] 15.7 **i18n** for the two new error codes. en + zh under `publications.errors.*`. Suggested copy — en: `ASSIGNMENT_USER_ALREADY_IN_SLOT` → `"This user is already assigned to another position in this slot."`; `TEMPLATE_SLOT_OVERLAP` → `"This slot overlaps with an existing slot on the same day."`; zh equivalents. Verify: `cd frontend && pnpm build && python3 -c "import json;a,b=[set(__import__('json').load(open(f'frontend/src/i18n/locales/{l}.json'))) for l in ('en','zh')];print(a^b)"` — empty set expected.
- [x] 15.8 Re-run full gate: `cd backend && go build ./... && go vet ./... && go test ./... && govulncheck ./...`, integration tests, `cd frontend && pnpm lint && pnpm test && pnpm build` — all clean.
