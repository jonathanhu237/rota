## 1. Backend error taxonomy

- [x] 1.1 Add `ErrPublicationNotMutable` sentinel to `backend/internal/model/publication.go`. Verify: `cd backend && go build ./...`.
- [x] 1.2 Alias the new sentinel in `backend/internal/service/publication.go` and map it to `PUBLICATION_NOT_MUTABLE` (HTTP 409) in `backend/internal/handler/publication.go:writePublicationServiceError`. Verify: `cd backend && go vet ./... && go build ./...`.
- [x] 1.3 Add `PUBLICATION_NOT_MUTABLE` to the frontend `ApiErrorCode` union in `frontend/src/lib/api-error.ts`. Verify: `cd frontend && pnpm tsc --noEmit`.

## 2. Backend state-guard widening

- [x] 2.1 In `backend/internal/service/publication_pr4.go`, widen the effective-state guard of `CreateAssignment` from `== ASSIGNING` to `∈ {ASSIGNING, PUBLISHED, ACTIVE}`; return `ErrPublicationNotMutable` outside that set.
- [x] 2.2 Apply the same widening to `DeleteAssignment`.
- [x] 2.3 Leave `AutoAssignPublication` unchanged (still `== ASSIGNING`, still `ErrPublicationNotAssigning`).
- [x] 2.4 Update existing service tests in `publication_pr4_test.go`: the table that asserts rejection for non-`ASSIGNING` states must now assert success for `PUBLISHED` and `ACTIVE`, and `ErrPublicationNotMutable` for `DRAFT`/`COLLECTING`/`ENDED`. Verify: `cd backend && go test ./internal/service -run Assignment -count=1`.

## 3. Cascade invalidation on admin delete

- [x] 3.1 Add `InvalidateRequestsForAssignment(ctx, assignmentID, now) ([]int64, error)` to `backend/internal/repository/shift_change.go`: single `UPDATE ... RETURNING id` that transitions pending rows where `requester_assignment_id = $1 OR counterpart_assignment_id = $1` to `invalidated`.
- [x] 3.2 Add a new audit action `ActionShiftChangeInvalidateCascade = "shift_change.invalidate.cascade"` to `backend/internal/audit/audit.go`.
- [x] 3.3 Add `ShiftChangeOutcomeInvalidated` to `backend/internal/email/shift_change.go` and extend `BuildShiftChangeResolvedMessage` to render the invalidation body copy.
- [x] 3.4 In `publication_pr4.go:DeleteAssignment`, after the repo delete succeeds, call the new shift-change repo helper; for each returned request id, emit one `audit.Record` and one `emailer.Send`. Cascade failures are logged at WARN and do not surface to the caller.
- [x] 3.5 Add service tests in `publication_pr4_test.go` covering: requester-side reference, counterpart-side reference, non-pending reference (not touched), missing reference (no events), and cascade-UPDATE error (delete still succeeds). Verify: `cd backend && go test ./internal/service -run DeleteAssignment -count=1`.
- [x] 3.6 Add a repository integration test in `backend/internal/repository/shift_change_db_test.go` for `InvalidateRequestsForAssignment` covering both FK sides and state-filter correctness. Verify: `POSTGRES_HOST=localhost POSTGRES_PORT=${POSTGRES_PORT:-5432} POSTGRES_USER=rota POSTGRES_PASSWORD=pa55word POSTGRES_DB=rota go test -tags=integration ./internal/repository -run InvalidateRequestsForAssignment -count=1`.

## 4. Assignment board non-candidate qualified

- [x] 4.1 Add `ListQualifiedUsersForPositions(ctx, positionIDs []int64) (map[int64][]*model.AssignmentCandidate, error)` to the publication or user-position repository (pick the more natural home).
- [x] 4.2 In `GetAssignmentBoard` (publication_pr4.go), after loading candidates and assignments, populate each shift's `NonCandidateQualified` = qualified-for-position minus candidates minus assigned. Type: `[]*model.AssignmentCandidate`.
- [x] 4.3 Extend `AssignmentBoardShiftResult` (service) and `assignmentBoardShiftResponse` (handler) to carry `NonCandidateQualified` / `non_candidate_qualified`.
- [x] 4.4 Add a service test that seeds one candidate, one assigned, and one qualified-but-silent user for a shift; asserts each user appears in the correct list only. Verify: `cd backend && go test ./internal/service -run AssignmentBoard -count=1`.
- [x] 4.5 Add a handler response test asserting the JSON shape includes `non_candidate_qualified`. Verify: `cd backend && go test ./internal/handler -run AssignmentBoard -count=1`.

## 5. Frontend — assignment board

- [x] 5.1 Add `non_candidate_qualified: AssignmentBoardCandidate[]` to `AssignmentBoardShift` in `frontend/src/lib/types.ts`.
- [x] 5.2 In `frontend/src/components/assignments/assignment-board.tsx`, add a "Show all qualified employees" toggle (shadcn `Switch`). When on, render each shift's `non_candidate_qualified` as outlined chips below the candidate list, with a subtle "Didn't submit availability" indicator.
- [x] 5.3 Widen the mutation-enabled predicate on that page from `state === "ASSIGNING"` to `∈ ["ASSIGNING", "PUBLISHED", "ACTIVE"]`.
- [x] 5.4 Add two state-specific warning banners: PUBLISHED ("visible to employees; pending shift-change requests may be invalidated"), ACTIVE ("changes take effect immediately").
- [x] 5.5 Add pure-logic tests for the non-candidate filter and the enabled-state predicate. Verify: `cd frontend && pnpm test -- assignment-board`.

## 6. Frontend — /requests surfacing cascade reason

- [x] 6.1 In `frontend/src/components/requests/requests-list.tsx`, detect `state === "invalidated"` on history rows and render the cascade reason copy: "Cancelled because the referenced shift was edited by an administrator."
- [x] 6.2 Add one test case to `requests-list.test.tsx` covering this branch. Verify: `cd frontend && pnpm test -- requests-list`.

## 7. i18n

- [x] 7.1 Add English + Chinese entries for: `PUBLICATION_NOT_MUTABLE`, the board toggle copy, the two warning banners, the cascade reason on request cards. Keys land under `publications.errors`, `publications.assignmentBoard.*`, `requests.history.*`.
- [x] 7.2 Verify key parity across `en` and `zh`: `python3 -c "import json;a,b=[set(__import__('json').load(open(f'frontend/src/i18n/locales/{l}.json'))) for l in ('en','zh')];print(a^b)"` — empty set expected.

## 8. Final verification

- [x] 8.1 `cd backend && go build ./... && go vet ./... && go test ./... && govulncheck ./...` — all clean.
- [x] 8.2 `POSTGRES_HOST=localhost POSTGRES_PORT=${POSTGRES_PORT:-5432} POSTGRES_USER=rota POSTGRES_PASSWORD=pa55word POSTGRES_DB=rota go test -tags=integration ./... -count=1` — all clean.
- [x] 8.3 `cd frontend && pnpm lint && pnpm test && pnpm build` — all clean.
- [x] 8.4 Smoke test against `docker compose -f docker-compose.prod.yml up`: publish a publication, assign and pending-swap one employee, admin deletes that assignment, verify the swap is `invalidated` in `/requests` history and the affected employee received an email (log line in dev mode).

## 9. Review-time adjustments

The history copy originally written for task 6.1/7.1 (`"Cancelled because the referenced shift was edited by an administrator."`) surfaces for every row with `state === "invalidated"`, including rows invalidated by the pre-existing approval-time optimistic-lock path (`ApplySwap` / `ApplyGive` user_id mismatch). The cascade-specific wording is misleading for that branch. Broaden the copy to cover both paths without changing any code behavior.

- [x] 9.1 Update `requests.history.invalidatedReason` in `frontend/src/i18n/locales/en.json` and `frontend/src/i18n/locales/zh.json` to the values below. Do not change any other i18n keys, do not touch `requests-list.tsx` or its test (the rendering logic and the key lookup are already correct).
  - `en.json` → `"Cancelled because the referenced shift changed before the request was processed."`
  - `zh.json` → `"由于关联班次在处理前已变更，此申请已被取消。"`
  - Verify: `cd frontend && pnpm test -- requests-list && pnpm build`.
