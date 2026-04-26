## 1. Database migration

- [x] 1.1 Add `migrations/000NN_create_email_outbox_table.sql` (next sequential, likely `00014`) per design D-7. Up creates `email_outbox` with the columns + CHECK + partial index. Down drops them in reverse. Verify: `make migrate-down && make migrate-up && make migrate-down` clean; CI's `migrations-roundtrip` exercises Up/Down.

## 2. OutboxRepository

- [x] 2.1 Add `backend/internal/repository/email_outbox.go` with:
    - `OutboxJob` struct: `{ ID int64; Recipient, Subject, Body string; RetryCount int }`.
    - `OutboxRepository` struct holding `*sql.DB`.
    - `NewOutboxRepository(db *sql.DB) *OutboxRepository`.
    - `EnqueueTx(ctx, tx *sql.Tx, msg email.Message) error` — single INSERT, default values.
    - `Claim(ctx, batchSize int) ([]OutboxJob, error)` — `UPDATE … RETURNING …` over a `SELECT … FOR UPDATE SKIP LOCKED` subquery filtered by `status='pending' AND next_attempt_at <= NOW()`, ordered by `next_attempt_at`, limit batchSize; set a short visibility lease by moving claimed rows' `next_attempt_at` into the future.
    - `MarkSent(ctx, id int64) error` — sets `status='sent'`, `sent_at=NOW()`.
    - `MarkRetryable(ctx, id int64, lastError string, nextAttemptAt time.Time) error` — bumps `retry_count`, stores error, advances `next_attempt_at`, keeps `status='pending'`.
    - `MarkFailed(ctx, id int64, lastError string) error` — sets `status='failed'`, `failed_at=NOW()`, stores error.
  Verify: `cd backend && go build ./...`.

- [x] 2.2 Add `backend/internal/repository/email_outbox_db_test.go` with `//go:build integration` covering: enqueue + claim round-trip; claim respects `next_attempt_at` (future rows skipped); claim FOR UPDATE SKIP LOCKED behavior with a concurrent goroutine; MarkSent / MarkRetryable / MarkFailed each transition correctly; partial index excludes `sent` / `failed` rows from claim. Verify: `go test -tags=integration ./internal/repository -run Outbox -count=1`.

## 3. Worker goroutine

- [x] 3.1 Add `backend/cmd/server/outbox_worker.go` with `RunOutboxWorker(ctx, outboxRepo, emailer, logger)` exposing:
    - 5-second ticker
    - per-tick `Claim(ctx, 10)`
    - per-job send + Mark*
    - exponential backoff math: `min(2^(retry_count+1) minutes, 1 hour)` capped
    - max retries = 8 → MarkFailed + emit audit event
    - panic recovery in the per-job loop
    - clean exit on ctx cancellation
  Decision: keep this in `cmd/server`, next to the existing process-lifecycle wiring, because the worker coordinates repository, emailer, audit, and shutdown context. Verify: unit tests for backoff math and retry-vs-fail decision; integration tests for claim → fail → retry → succeed and terminal fail → audit.

- [x] 3.2 Wire `RunOutboxWorker` into `backend/cmd/server/main.go` immediately after the existing `startSessionCleanup` call. Use the same `cleanupCtx` (or a fresh background ctx with cancel on shutdown). Verify: `go test ./cmd/server -count=1` and `POSTGRES_PORT=5432 go test -tags=integration ./cmd/server -run OutboxWorkerIntegration -count=1`.

## 4. Producer-side refactor

- [x] 4.1 Update `backend/internal/service/setup.go`: replace the direct `h.emailer.Send` calls in `sendInvitation` and `sendPasswordReset` with `h.outboxRepo.EnqueueTx(ctx, tx, msg)`. The setup helper's signature changes: it now accepts an `outboxRepo` instead of `emailer`. The callers (in `txManager.WithinTx` callbacks in `service/user.go::CreateUser`, `service/user.go::ResendInvitation`, and `service/auth.go::RequestPasswordReset`) pass the in-flight `tx` so the enqueue joins their transaction. Verify: `go build ./...`; service tests adapted to mock `outboxRepo` instead of `emailer`.

- [x] 4.2 Update `backend/internal/service/shift_change.go`: refactor `notifyRequestReceived` and `notifyRequestResolved` to enqueue inside the SCRT mutation transaction. Per design D-2, two valid shapes — pick whichever matches existing repository contract style:
    - **Shape A (preferred):** the service opens the tx, calls the repo's mutation method (now taking a `*sql.Tx` instead of opening its own), and then calls `outboxRepo.EnqueueTx(ctx, tx, msg)` before commit.
    - **Shape B (fallback):** the repository's apply method accepts an `enqueueOnSuccess func(tx *sql.Tx) error` callback that runs inside the apply tx after the state transition.
  Verify: `go test ./internal/service -run ShiftChange -count=1` and integration tests cover the new path; `go test -tags=integration ./internal/repository -run ShiftChange -count=1` exercises the schema interaction.

- [x] 4.3 Update `backend/internal/service/publication_pr4.go`: the cascade-resolved notification on admin assignment delete moves into the delete transaction the same way (Shape A or B). Verify: existing assignment-delete tests still pass; the cascade test asserts an outbox row was enqueued instead of asserting an email was sent.

- [x] 4.4 Update `backend/internal/service/auth.go` and `backend/internal/service/user.go` constructors / wiring: replace `email.Emailer` parameter with `*repository.OutboxRepository` (or interface). The `Emailer` is no longer reachable from these services. Verify: `go build ./...`.

## 5. Audit-action firing point shift

- [x] 5.1 Remove the producer-side audit emission for `user.invitation.email_failed` (currently in `service/user.go::recordInvitationEmailFailure` or similar). The producer no longer sees SMTP errors; there's nothing to audit at producer time. Verify: `grep -nE "ActionUserInvitationEmailFailed" backend/internal/service` shows no matches outside of test mocks.

- [x] 5.2 Add audit emission at the worker's `MarkFailed` path: when an invitation-purpose row transitions to `failed`, emit `user.invitation.email_failed` with `target_type=user`, `target_id=<user_id>`, and metadata `{ email, error }`. The worker needs a way to recover the `user_id` from the outbox row — either by adding an optional `user_id` column or by extracting it from the body via a helper. **Decision:** add an optional `user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL` column to `email_outbox` so audit metadata is precise; leave `NULL` for non-user emails (none today, but possible in future). Update the migration in §1.1 to include this column. Verify: an integration test simulates 8 send failures and asserts one audit event is recorded.

- [x] 5.3 Update the audit-action catalog in `backend/internal/audit/audit.go` if anything references the producer firing point in a comment. Verify: comments and constants accurately describe the new behavior.

## 6. Service-test refactor

- [x] 6.1 Update `backend/internal/service/auth_test.go`, `service/user_test.go`, `service/shift_change_test.go`, and any other affected service tests:
    - Replace the `email.Emailer` mock with an `outboxRepository` mock.
    - The mock records `EnqueueTx` calls and returns nil / an error per test case.
    - Tests that previously asserted "emailer was called with subject X" now assert "outbox was enqueued with subject X".
    - Tests that exercised the audit-emission path on email failure (`recordInvitationEmailFailure`) move to a worker-level test, since that's where the audit fires now.
  Verify: `go test ./internal/service -count=1` passes.

## 7. Final verification

- [x] 7.1 Backend clean: `cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./... && govulncheck ./...` — every step exits 0.
- [x] 7.2 Frontend untouched: `cd frontend && pnpm lint && pnpm test && pnpm build` (regression check; this change touches no frontend code).
- [x] 7.3 Migrations roundtrip: apply all migrations on a throwaway Postgres database, run `goose down`, then run `goose up` again.
- [x] 7.4 Automated smoke coverage:
    - (a) `POSTGRES_PORT=5432 go test -tags=integration ./cmd/server -run OutboxWorkerIntegration -count=1` covers a real outbox row transitioning `pending → retryable → sent`.
    - (b) The same integration test covers an invitation row with `retry_count=7` transitioning to `failed` and writing `user.invitation.email_failed`.
    - (c) `POSTGRES_PORT=5432 go test -tags=integration ./internal/repository -run Outbox -count=1` covers transactional enqueue, claim lease, terminal-row exclusion, and concurrent claim behavior.
    - (d) `go test ./internal/service -count=1` covers password reset, invitation, shift-change, and assignment-delete producers enqueueing through the outbox mocks.
- [x] 7.5 Confirm zero direct calls to `emailer.Send` from non-worker production code: `rg -n "emailer\\.Send|email\\.Emailer|Emailer:" backend/internal/service backend/internal/handler --glob '!**/*_test.go'` returns no hits (only the worker should hold the Emailer reference).
- [x] 7.6 Confirm local CI-equivalent checks are green on the `change/add-email-outbox` branch: backend build/vet/test/integration/govulncheck, frontend lint/test/build, migrations roundtrip. Remote CI remains a post-push handoff check.
