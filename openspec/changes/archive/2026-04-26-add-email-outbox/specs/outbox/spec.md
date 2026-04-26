## ADDED Requirements

### Requirement: Email outbox table and enqueue contract

The system SHALL persist outgoing email intents in an `email_outbox` table with columns: `id BIGSERIAL primary key`, `user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL`, `recipient TEXT NOT NULL`, `subject TEXT NOT NULL`, `body TEXT NOT NULL`, `status TEXT NOT NULL DEFAULT 'pending'` (CHECK in `{pending, sent, failed}`), `retry_count INT NOT NULL DEFAULT 0`, `last_error TEXT`, `next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, `sent_at TIMESTAMPTZ NULL`, `failed_at TIMESTAMPTZ NULL`. A partial index `(next_attempt_at) WHERE status = 'pending'` SHALL exist to support the worker's poll.

Producers SHALL enqueue email intents via `OutboxRepository.EnqueueTx(ctx, tx, msg)`, where `tx` is the same `*sql.Tx` used for the producer's business write. The outbox INSERT SHALL be part of the producer's transaction so business state and email intent commit atomically.

The repository SHALL expose the operations needed by the worker:

- `Claim(ctx, batchSize)` — issues a `SELECT … FOR UPDATE SKIP LOCKED` over `pending` rows whose `next_attempt_at <= NOW()`, ordered by `next_attempt_at`, limit `batchSize`. It SHALL set a short visibility lease by moving each claimed row's `next_attempt_at` into the future before returning the rows. Concurrent workers SHALL NOT receive overlapping rows, and claimed rows SHALL become eligible again after the lease if the worker exits before marking them sent, retryable, or failed.
- `MarkSent(ctx, id)` — sets `status = 'sent'`, `sent_at = NOW()`.
- `MarkRetryable(ctx, id, lastError, nextAttemptAt)` — bumps `retry_count`, stores the error, advances `next_attempt_at`, leaves `status = 'pending'`.
- `MarkFailed(ctx, id, lastError)` — sets `status = 'failed'`, `failed_at = NOW()`, stores the error.

#### Scenario: Enqueue commits with the producer's transaction

- **GIVEN** a producer's business transaction that inserts a user row and calls `outboxRepo.EnqueueTx(ctx, tx, msg)` for the invitation email
- **WHEN** the transaction commits
- **THEN** the user row and the email_outbox row are both visible
- **WHEN** the transaction rolls back
- **THEN** neither row is visible

#### Scenario: Concurrent claims do not duplicate

- **GIVEN** two worker instances both calling `Claim(ctx, batchSize=10)` on the same outbox table that contains 15 pending rows
- **WHEN** the calls execute concurrently
- **THEN** the union of returned rows contains each pending row at most once
- **AND** the `FOR UPDATE SKIP LOCKED` clause causes one caller to skip rows the other has locked
- **AND** the claim lease prevents rows returned by the first completed call from being immediately returned by the second

#### Scenario: Claim respects next_attempt_at

- **GIVEN** an outbox row with `status = 'pending'` and `next_attempt_at` in the future
- **WHEN** `Claim(ctx, batchSize)` runs at a time before `next_attempt_at`
- **THEN** the row is NOT returned

### Requirement: Email outbox worker

The backend process SHALL run a single background goroutine that drains the outbox. The goroutine SHALL tick every five seconds, call `Claim(ctx, 10)`, and for each returned job call the configured `email.Emailer.Send`:

- On success the job is marked sent via `MarkSent`.
- On failure when `retry_count + 1 < maxRetries` (where `maxRetries = 8`), the job is marked retryable via `MarkRetryable` with `next_attempt_at = NOW() + min(2^(retry_count+1) minutes, 1 hour)` (exponential backoff capped at one hour).
- On failure when `retry_count + 1 >= maxRetries`, the job is marked failed via `MarkFailed`.

A panic during a single send SHALL NOT exit the goroutine; the panic SHALL be recovered, logged, and the goroutine SHALL continue with the next tick. Errors from `Claim`, `MarkSent`, `MarkRetryable`, or `MarkFailed` SHALL be logged via slog and SHALL NOT exit the goroutine.

The goroutine SHALL exit cleanly on context cancellation (process shutdown).

#### Scenario: Worker sends pending email and marks it sent

- **GIVEN** an outbox row with `status = 'pending'`, `next_attempt_at <= NOW()`
- **AND** the configured emailer's `Send` returns nil
- **WHEN** the worker tick runs
- **THEN** the row's `status` becomes `sent`
- **AND** the row's `sent_at` is set to NOW()

#### Scenario: Worker schedules retry on transient failure

- **GIVEN** an outbox row with `retry_count = 2`
- **AND** the configured emailer's `Send` returns an error
- **WHEN** the worker tick runs
- **THEN** the row's `retry_count` becomes 3
- **AND** the row's `last_error` is set to the error message
- **AND** the row's `next_attempt_at` is approximately NOW() + 8 minutes (= 2^3)
- **AND** the row's `status` remains `pending`

#### Scenario: Worker marks failed after retry exhaustion

- **GIVEN** an outbox row with `retry_count = 7`
- **AND** the configured emailer's `Send` returns an error
- **WHEN** the worker tick runs
- **THEN** the row's `status` becomes `failed`
- **AND** the row's `failed_at` is set to NOW()
- **AND** the row's `last_error` is set to the error message
- **AND** the row is no longer eligible for `Claim`

#### Scenario: Backoff caps at one hour

- **GIVEN** an outbox row with `retry_count = 6`
- **AND** the configured emailer's `Send` returns an error
- **WHEN** the worker tick runs
- **THEN** the row's `next_attempt_at` is approximately NOW() + 60 minutes (cap), not NOW() + 64 minutes (= 2^6)

#### Scenario: Worker survives a panic

- **GIVEN** the configured emailer panics on a particular row
- **WHEN** the worker tick runs
- **THEN** the panic is recovered and logged
- **AND** subsequent ticks continue to process the remaining rows

### Requirement: No synchronous SMTP from request paths

Producer-side request handlers SHALL NOT call `email.Emailer.Send` directly. The only path from producer code to SMTP SHALL be through `OutboxRepository.EnqueueTx`. The `email.Emailer` interface SHALL be held by the worker goroutine only; services and handlers SHALL receive the outbox repository instead.

#### Scenario: Producer enqueues, never sends directly

- **GIVEN** a service method that creates a user and triggers an invitation
- **WHEN** the producer logic runs to completion
- **THEN** the producer has called `outboxRepo.EnqueueTx(...)`
- **AND** the producer has NOT called `emailer.Send(...)`
