## 1. A3 — qualification filter at read time

- [x] 1.1 In `backend/internal/repository/assignment.go`, modify the `ListAssignmentCandidates` SQL: add `INNER JOIN user_positions up ON up.user_id = asub.user_id AND up.position_id = asub.position_id` and `AND u.status = 'active'` in the WHERE clause. Keep the existing ORDER BY. Verify: `cd backend && go build ./...`.
- [x] 1.2 Add a service-layer test in `backend/internal/service/publication_pr5_test.go` (or similar): seed a publication with COLLECTING-era submissions for user `U` at position `P`. Then remove `P` from `U`'s `user_positions`. Run `AutoAssignPublication`. Assert `U` is NOT assigned to `(slot, P)`. Verify: `cd backend && go test ./internal/service -run AutoAssignSkipsRevokedQualification -count=1`.
- [x] 1.3 Add a parallel test for `u.status = disabled`. Verify: `cd backend && go test ./internal/service -run AutoAssignSkipsDisabled -count=1`.
- [x] 1.4 Add a repository integration test confirming the SQL filter works directly. Verify: `POSTGRES_HOST=localhost POSTGRES_PORT=${POSTGRES_PORT:-5432} POSTGRES_USER=rota POSTGRES_PASSWORD=pa55word POSTGRES_DB=rota go test -tags=integration ./internal/repository -run ListAssignmentCandidatesFiltered -count=1`.

## 2. New shared helper — lock-and-check user schedule + status (in-tx)

- [x] 2.1 In `backend/internal/repository/assignment.go`, add a new exported helper function `LockAndCheckUserSchedule(ctx, tx *sql.Tx, publicationID, userID int64, additions []SlotTimeWindow, excludeAssignmentIDs []int64) error`. The helper SHALL:
  - Run `SELECT a.id, ts.weekday, ts.start_time, ts.end_time FROM assignments a INNER JOIN template_slots ts ON ts.id = a.slot_id WHERE a.publication_id = $1 AND a.user_id = $2 ORDER BY a.id ASC FOR UPDATE OF a` (sorting by `a.id` to enforce a deterministic lock order across concurrent transactions, reducing deadlock risk).
  - Filter out rows whose `id` is in `excludeAssignmentIDs`.
  - Append the `additions` (caller-supplied future slots) to the resulting list.
  - Run the existing time-conflict predicate (same-weekday + `a.start < b.end && b.start < a.end`) over every pair.
  - On conflict, return `ErrTimeConflict` (a new sentinel exported from the repository package, OR map onto callers' existing sentinels at the service layer — choose the simpler path).
  - Run a separate `SELECT status FROM users WHERE id = $userID FOR UPDATE`. If `status != 'active'`, return `ErrUserDisabled`.
  Verify: `cd backend && go build ./...`.
- [x] 2.2 Define a small `SlotTimeWindow` struct in the repository package `{ Weekday int; StartTime, EndTime string }`. Use it both as the helper's input shape and as the return shape of the lock-read step. Verify: same.
- [x] 2.3 Add a repository integration test for the helper itself: seed a user with two assignments, call the helper with `additions = [overlapping window]`, expect `ErrTimeConflict`. Then `additions = [non-overlapping window]`, expect `nil`. Then disable the user, expect `ErrUserDisabled`. Verify: `POSTGRES_HOST=... go test -tags=integration ./internal/repository -run LockAndCheckUserSchedule -count=1`.

## 3. Wire helper into ApplyGive

- [x] 3.1 In `backend/internal/repository/shift_change.go` `ApplyGive`, after `lockAssignment(requesterAssignmentID)` succeeds, call `LockAndCheckUserSchedule(tx, publicationID, receiverUserID, additions=[requester's slot's window], excludeAssignmentIDs=[])`. The `additions` slice has one entry — the slot the receiver is taking on. The receiver is NOT giving up any assignment in a give path, so `excludeAssignmentIDs` is empty. On `ErrTimeConflict`, return `ErrShiftChangeTimeConflict` (existing sentinel). On `ErrUserDisabled`, return `ErrUserDisabled`. Verify: `cd backend && go build ./... && go vet ./...`.
- [x] 3.2 Update the service layer `ApproveShiftChangeRequest` (`backend/internal/service/shift_change.go`) so the existing pre-tx `ensureGiveFitsSchedule` call remains as a fast-fail (preserved for UX latency), but add a comment that the in-tx check is the correctness floor. Verify: `cd backend && go test ./internal/service -run ApproveGive -count=1`.
- [x] 3.3 Add a service test that exercises the race: two concurrent `ApplyGive` calls for the same receiver where each gives them an overlapping slot. Use `sync.WaitGroup` + `goroutine` + a real Postgres connection (so this is integration-level). Assert: exactly one succeeds, the other returns `ErrShiftChangeTimeConflict`. Receiver ends up with at most one of the two slots. Verify: `POSTGRES_HOST=... go test -tags=integration ./internal/service -run ConcurrentApplyGive -count=1`.

## 4. Wire helper into ApplySwap

- [x] 4.1 In `ApplySwap`, after `lockAssignment(requester_assignment_id)` and `lockAssignment(counterpart_assignment_id)` both succeed, call `LockAndCheckUserSchedule` for each side: for the requester, `additions=[counterpart slot]` and `excludeAssignmentIDs=[requesterAssignmentID]` (the requester gives up their slot). For the counterpart, mirror it. On either side returning conflict, return `ErrShiftChangeTimeConflict`. Verify: `cd backend && go build ./...`.
- [x] 4.2 Add an integration-level service test for concurrent ApplySwap: race two swaps that would each, individually, leave one user with non-conflicting schedule, but together produce conflict. Verify: `POSTGRES_HOST=... go test -tags=integration ./internal/service -run ConcurrentApplySwap -count=1`.

## 5. Wire helper into CreateAssignment

- [x] 5.1 In `backend/internal/service/publication_pr4.go` `CreateAssignment`, before the `repo.CreateAssignment` call, wrap the section in a transaction (the repository's CreateAssignment is currently a single statement; wrap it in `r.db.BeginTx` and call `LockAndCheckUserSchedule(tx, publicationID, userID, additions=[new slot's window], excludeAssignmentIDs=[])`, then run the INSERT inside the same tx). Map `ErrTimeConflict` → `ErrAssignmentTimeConflict` and `ErrUserDisabled` → `ErrUserDisabled` (existing sentinels). The pre-tx fast-fail (existing `hasAssignmentTimeConflict`) is preserved. Verify: `cd backend && go build ./...`.
- [x] 5.2 Restructure the repository's CreateAssignment so it accepts an optional `*sql.Tx` (or split into a "with tx" and a "owns its own tx" variant). The service layer calls the with-tx variant. Avoid breaking other callers; auto-assign's `ReplaceAssignments` is a different code path and is unaffected. Verify: `cd backend && go build ./...`.
- [x] 5.3 Service test: concurrent admin CreateAssignment + ApplyGive for the same target user where the additions overlap. Assert: only one succeeds. Verify: `POSTGRES_HOST=... go test -tags=integration ./internal/service -run ConcurrentCreateAssignment -count=1`.

## 6. User-status in-tx check

- [x] 6.1 The `LockAndCheckUserSchedule` helper from §2.1 already includes `SELECT status FROM users WHERE id = ? FOR UPDATE`. Confirm all three call sites (ApplyGive, ApplySwap both sides, CreateAssignment) pick up `ErrUserDisabled` from the helper and return it correctly. Handler maps `ErrUserDisabled` → `USER_DISABLED` (409); already exists. Verify: `cd backend && go test ./internal/service -count=1 && go test ./internal/handler -count=1`.
- [x] 6.2 Service test: receiver of ApplyGive was active at request creation, then admin disables them, then their (still-valid) session approves. Assert: in-tx check observes `disabled`, returns `ErrUserDisabled`, request is NOT applied. Verify: `cd backend && go test ./internal/service -run ApplyGiveDisabledReceiver -count=1`.
- [x] 6.3 Parallel test for ApplySwap (counterpart disabled mid-flight). Verify: same.
- [x] 6.4 Parallel test for CreateAssignment (target user disabled between pre-tx check and insert). This requires injecting a "disable in the middle" step; use a stub repo or a real DB integration test. Verify: as appropriate.

## 7. Deadlock retry mapping

- [x] 7.1 In all three apply / create paths, detect Postgres `pq.Error` with code `40P01` (deadlock_detected) and map to a transient-retry sentinel `ErrSchedulingRetryable`. Handler maps to HTTP 503 with error code `SCHEDULING_RETRYABLE` (new in error catalog) so the client knows it can safely retry. The lock-ordering by `a.id ASC` in §2.1 makes deadlock unlikely; this is defense-in-depth. Verify: `cd backend && go build ./...`.
- [x] 7.2 Frontend: add `SCHEDULING_RETRYABLE` to `frontend/src/lib/api-error.ts` and i18n copy (en + zh). Verify: `cd frontend && pnpm tsc --noEmit && pnpm build`.

## 8. Spec sync (no code, archival prep)

- [x] 8.1 Confirm the change's delta spec at `openspec/changes/tighten-scheduling-edges/specs/scheduling/spec.md` is correct: 4 MODIFIED requirements with full updated content + new scenarios. `openspec validate tighten-scheduling-edges --strict` passes. Verify: `openspec validate tighten-scheduling-edges --strict`.

## 9. Final verification

- [x] 9.1 `cd backend && go build ./... && go vet ./... && go test ./... && govulncheck ./...` — all clean.
- [x] 9.2 `POSTGRES_HOST=localhost POSTGRES_PORT=${POSTGRES_PORT:-5432} POSTGRES_USER=rota POSTGRES_PASSWORD=pa55word POSTGRES_DB=rota go test -tags=integration ./... -count=1` — all clean.
- [x] 9.3 `cd frontend && pnpm lint && pnpm test && pnpm build` — all clean.
- [x] 9.4 Smoke test against `docker compose -f docker-compose.prod.yml up`: (a) Create a user, submit availability for a position, admin removes that position, admin runs auto-assign, verify user is not assigned. (b) Create two pending give_directs to the same user with overlapping slots, fire two parallel approve curls, verify exactly one succeeds with 204 and the other returns 409 `SHIFT_CHANGE_TIME_CONFLICT`. (c) Verify disabled-user rejection at the API boundary with admin `CreateAssignment` returning 409 `USER_DISABLED`, and verify the already-authorized in-flight give race with `TestApplyGiveDisabledReceiverWhileScheduleLocked` because auth middleware rejects already-disabled users before the shift-change handler. (d) Race admin `CreateAssignment` and a give for the same user with overlapping slots, verify only one succeeds.
