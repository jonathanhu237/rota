## 1. Simplify the helper

- [x] 1.1 In `backend/internal/repository/assignment.go`, rename `LockAndCheckUserSchedule` to `LockAndCheckUserStatus`. Drop the `additions []SlotTimeWindow` and `excludeAssignmentIDs []int64` parameters. Drop the `SELECT ... FOR UPDATE OF a` over assignments and the entire conflict-predicate loop. Keep the per-(publication, user) advisory lock and the `SELECT status FROM users WHERE id = $userID FOR UPDATE` + `ErrUserDisabled` return. Verify: `cd backend && go build ./...`.
- [x] 1.2 Remove the `ErrTimeConflict` repository sentinel (no caller will return or consume it after §3 / §4 / §5 are done). Verify: `cd backend && go build ./...`.
- [x] 1.3 Remove the `SlotTimeWindow` struct if it has no other users in the repository package. If something else uses it, leave it. Verify: `cd backend && go build ./...`.
- [x] 1.4 Update or replace the helper's repository integration test (`LockAndCheckUserSchedule` → `LockAndCheckUserStatus`): drop the conflict assertions, keep the disabled-user assertion. Rename test to match. Verify: `POSTGRES_HOST=localhost POSTGRES_PORT=${POSTGRES_PORT:-5433} POSTGRES_USER=rota POSTGRES_PASSWORD=pa55word POSTGRES_DB=rota go test -tags=integration ./internal/repository -run LockAndCheckUserStatus -count=1`.

## 2. Remove pre-tx time-conflict checks (service layer)

- [x] 2.1 In `backend/internal/service/shift_change.go`, delete `ensureSwapFitsSchedule` and `ensureGiveFitsSchedule` and their call sites in `ApproveShiftChangeRequest`. Verify: `cd backend && go build ./... && go vet ./...`.
- [x] 2.2 In `backend/internal/service/publication_pr4.go` `CreateAssignment`, delete `hasAssignmentTimeConflict` and its call site (the pre-tx conflict block). The flow becomes: validate input → resolve publication / slot / position → load user → call `LockAndCheckUserStatus` inside the existing tx → INSERT assignment → audit. Verify: `cd backend && go build ./... && go vet ./...`.
- [x] 2.3 Remove the now-orphaned `ListUserAssignmentsOnWeekdayInPublication` repo method (was only used by the deleted pre-tx checks). Verify: same.
- [x] 2.4 Remove service-level aliases `ErrShiftChangeTimeConflict` and `ErrAssignmentTimeConflict` AND the model-layer sentinels they alias, IF nothing else surfaces them. Search the codebase first; if any test or handler still references them after §3/§4/§5, leave the model sentinel and the alias intact. Verify: `cd backend && go build ./...`.

## 3. Update ApplyGive to use the simplified helper

- [x] 3.1 In `backend/internal/repository/shift_change.go` `ApplyGive`, change the helper call from `LockAndCheckUserSchedule(...)` to `LockAndCheckUserStatus(tx, publicationID, receiverUserID)`. Drop the `additions=[...]` and `excludeAssignmentIDs=[]` arguments. Verify: `cd backend && go build ./...`.
- [x] 3.2 Update the existing service-layer test for ApplyGive disabled-receiver mid-flight. Confirm the test still passes against the new helper signature. Verify: `cd backend && go test ./internal/service -run ApplyGiveDisabled -count=1`.

## 4. Update ApplySwap to use the simplified helper

- [x] 4.1 In `ApplySwap`, change both helper calls (one per side) to `LockAndCheckUserStatus`. Drop the conflict-related arguments. Verify: `cd backend && go build ./...`.
- [x] 4.2 Update or keep the swap disabled-counterpart mid-flight test. Verify: `cd backend && go test ./internal/service -run ApplySwapDisabled -count=1`.

## 5. Update CreateAssignment to use the simplified helper

- [x] 5.1 In `backend/internal/service/publication_pr4.go` `CreateAssignment`, change the helper call to `LockAndCheckUserStatus(tx, publicationID, userID)`. The repository-layer `repo.CreateAssignment(tx, ...)` (the with-tx variant introduced last change) continues unchanged. Verify: `cd backend && go build ./...`.
- [x] 5.2 Confirm the disabled-user mid-flight test for CreateAssignment still passes. Verify: `cd backend && go test ./internal/service -run CreateAssignmentDisabled -count=1`.

## 6. Remove the broken concurrent integration tests

- [x] 6.1 In `backend/internal/service/scheduling_edges_integration_test.go`, delete the test functions `TestConcurrentApplyGive`, `TestConcurrentApplySwap`, and `TestConcurrentCreateAssignment` along with any helper functions used only by them. Verify: `cd backend && go build ./... && go vet ./...`.
- [x] 6.2 Run the full integration test suite to confirm nothing else broke. Verify: `POSTGRES_HOST=... go test -tags=integration ./... -count=1`.

## 7. Remove the deadlock retry mapping IF unreachable, otherwise keep

- [x] 7.1 Audit whether `SCHEDULING_RETRYABLE` (`ErrSchedulingRetryable`, the `40P01` mapping) is reachable after §3/§4/§5. The single `users` row FOR UPDATE in `LockAndCheckUserStatus` can still produce a deadlock under unusual concurrency, so the mapping is likely still useful. If unreachable, remove backend mapping + frontend `api-error.ts` entry + i18n. If reachable, keep. Document the call. Verify: `cd backend && go build ./...`.

## 8. Final verification

- [x] 8.1 `cd backend && go build ./... && go vet ./... && go test ./... && govulncheck ./...` — all clean.
- [x] 8.2 `POSTGRES_HOST=localhost POSTGRES_PORT=${POSTGRES_PORT:-5433} POSTGRES_USER=rota POSTGRES_PASSWORD=pa55word POSTGRES_DB=rota go test -tags=integration ./... -count=1` — all clean. **Critically**: the 3 deleted concurrent tests are not silently skipped (they don't exist); the disabled-user mid-flight tests do still run and pass.
- [x] 8.3 `cd frontend && pnpm lint && pnpm test && pnpm build` — all clean. (Frontend should not need any changes.)
- [ ] 8.4 Push the branch; verify GitHub CI's `backend-test` job is green (it was failing on the 3 broken tests before this change).
