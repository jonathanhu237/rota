## MODIFIED Requirements

### Requirement: Email outbox table and enqueue contract

The system SHALL persist outgoing email intents in an `email_outbox` table with columns: `id BIGSERIAL primary key`, `user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL`, `kind TEXT NOT NULL DEFAULT 'unknown'`, `recipient TEXT NOT NULL`, `subject TEXT NOT NULL`, `body TEXT NOT NULL`, `html_body TEXT NULL`, `status TEXT NOT NULL DEFAULT 'pending'` (CHECK in `{pending, sent, failed}`), `retry_count INT NOT NULL DEFAULT 0`, `last_error TEXT`, `next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, `sent_at TIMESTAMPTZ NULL`, `failed_at TIMESTAMPTZ NULL`. A partial index `(next_attempt_at) WHERE status = 'pending'` SHALL exist to support the worker's poll.

Producers SHALL enqueue email intents via `OutboxRepository.EnqueueTx(ctx, tx, msg)`, where `tx` is the same `*sql.Tx` used for the producer's business write. The outbox INSERT SHALL be part of the producer's transaction so business state and email intent commit atomically. New producer paths SHALL set a stable `kind` value and MAY set `html_body`; legacy rows with `kind = 'unknown'` and `html_body IS NULL` SHALL remain valid.

The repository SHALL expose the operations needed by the worker:

- `Claim(ctx, batchSize)` — issues a `SELECT … FOR UPDATE SKIP LOCKED` over `pending` rows whose `next_attempt_at <= NOW()`, ordered by `next_attempt_at`, limit `batchSize`. It SHALL set a short visibility lease by moving each claimed row's `next_attempt_at` into the future before returning the rows. Concurrent workers SHALL NOT receive overlapping rows, and claimed rows SHALL become eligible again after the lease if the worker exits before marking them sent, retryable, or failed. Claimed jobs SHALL include `kind`, text body, and optional HTML body.
- `MarkSent(ctx, id)` — sets `status = 'sent'`, `sent_at = NOW()`.
- `MarkRetryable(ctx, id, lastError, nextAttemptAt)` — bumps `retry_count`, stores the error, advances `next_attempt_at`, leaves `status = 'pending'`.
- `MarkFailed(ctx, id, lastError)` — sets `status = 'failed'`, `failed_at = NOW()`, stores the error.

#### Scenario: Enqueue commits with the producer's transaction

- **GIVEN** a producer's business transaction that inserts a user row and calls `outboxRepo.EnqueueTx(ctx, tx, msg)` for the invitation email
- **WHEN** the transaction commits
- **THEN** the user row and the email_outbox row are both visible
- **AND** the email_outbox row stores the message kind and rendered text body
- **AND** the email_outbox row stores the rendered HTML body when the message has one
- **WHEN** the transaction rolls back
- **THEN** neither row is visible

#### Scenario: Legacy outbox rows remain valid

- **GIVEN** an outbox row created before this change
- **WHEN** the migration is applied
- **THEN** the row has `kind = 'unknown'`
- **AND** the row has `html_body IS NULL`
- **AND** the worker can still claim and send the row as a plain-text email

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

The backend process SHALL run a single background goroutine that drains the outbox. The goroutine SHALL tick every five seconds, call `Claim(ctx, 10)`, and for each returned job call the configured `email.Emailer.Send` with a per-message timeout (default `30s`, configurable by `EMAIL_SEND_TIMEOUT`):

- On success the job is marked sent via `MarkSent`.
- On failure when `retry_count + 1 < maxRetries` (where `maxRetries = 8`), the job is marked retryable via `MarkRetryable` with `next_attempt_at = NOW() + min(2^(retry_count+1) minutes, 1 hour)` (exponential backoff capped at one hour).
- On failure when `retry_count + 1 >= maxRetries`, the job is marked failed via `MarkFailed`.

A panic during a single send SHALL NOT exit the goroutine; the panic SHALL be recovered, logged, and the goroutine SHALL continue with the next tick. Errors from `Claim`, `MarkSent`, `MarkRetryable`, or `MarkFailed` SHALL be logged via slog and SHALL NOT exit the goroutine.

The goroutine SHALL exit cleanly on context cancellation (process shutdown). Invitation terminal-failure audit detection SHALL use the outbox job's stable `kind`, not localized subject text.

#### Scenario: Worker sends pending email and marks it sent

- **GIVEN** an outbox row with `status = 'pending'`, `next_attempt_at <= NOW()`
- **AND** the configured emailer's `Send` returns nil
- **WHEN** the worker tick runs
- **THEN** the row's `status` becomes `sent`
- **AND** the row's `sent_at` is set to NOW()

#### Scenario: Worker sends HTML email as multipart

- **GIVEN** an outbox row with a non-empty `html_body`
- **AND** the configured emailer's `Send` returns nil
- **WHEN** the worker tick runs
- **THEN** the emailer receives both the text body and HTML body
- **AND** the row is marked sent

#### Scenario: Worker applies send timeout

- **GIVEN** an outbox row with `status = 'pending'`, `next_attempt_at <= NOW()`
- **AND** the configured emailer does not return before `EMAIL_SEND_TIMEOUT`
- **WHEN** the worker tick runs
- **THEN** the send attempt is cancelled
- **AND** the row is marked retryable with a timeout error in `last_error`
- **AND** subsequent rows are not blocked indefinitely by that send attempt

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

#### Scenario: Invitation failure audit uses kind

- **GIVEN** an outbox row with `kind = 'invitation'` and `retry_count = 7`
- **AND** the configured emailer's `Send` returns an error
- **WHEN** the worker marks the row failed
- **THEN** a `user.invitation.email_failed` audit event is recorded for that row's user
- **AND** the audit does not depend on matching the row's localized `subject`

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

## ADDED Requirements

### Requirement: SMTP email rendering and logger behavior

The SMTP emailer SHALL emit standards-compliant UTF-8 messages. When `email.Message.HTMLBody` is empty, it SHALL send a plain `text/plain; charset=UTF-8` message. When `HTMLBody` is non-empty, it SHALL send `multipart/alternative` with the text/plain part first and the text/html part second. The logger emailer SHALL print recipient, subject, the text body, and whether an HTML body exists; it SHALL NOT dump full HTML to stdout.

#### Scenario: HTML message uses multipart alternative

- **GIVEN** an email message with both text body and HTML body
- **WHEN** the SMTP emailer sends the message
- **THEN** the SMTP payload includes `MIME-Version: 1.0`
- **AND** the top-level content type is `multipart/alternative`
- **AND** a `text/plain; charset=UTF-8` part appears before a `text/html; charset=UTF-8` part

#### Scenario: Plain text legacy message remains plain

- **GIVEN** an email message with an empty HTML body
- **WHEN** the SMTP emailer sends the message
- **THEN** the SMTP payload is a text/plain message
- **AND** no text/html part is emitted

#### Scenario: Logger marks HTML without printing it

- **GIVEN** an email message with a non-empty HTML body
- **WHEN** the logger emailer sends the message
- **THEN** the logger output includes `HTML: yes`
- **AND** the logger output includes the text body
- **AND** the logger output does NOT include the full HTML body

### Requirement: Email configuration validation

The server SHALL validate email-related configuration at startup. When `EMAIL_MODE=smtp`, `SMTP_HOST` and `SMTP_FROM` SHALL be non-empty. `EMAIL_SEND_TIMEOUT` SHALL default to `30s` and SHALL reject non-positive durations. In production, `APP_BASE_URL` SHALL NOT be empty, localhost, or loopback, and `SMTP_TLS_MODE=none` SHALL be rejected unless `SMTP_HOST` is localhost or loopback. The server SHALL log a warning, but still start, for common port/TLS mismatches: port 465 without `implicit`, or port 587 with `implicit`.

#### Scenario: SMTP mode requires host and sender

- **GIVEN** `EMAIL_MODE=smtp`
- **WHEN** `SMTP_HOST` or `SMTP_FROM` is empty
- **THEN** config loading fails
- **AND** the server does not start

#### Scenario: Production rejects localhost app base URL

- **GIVEN** `APP_ENV=production`
- **WHEN** `APP_BASE_URL` points at localhost or loopback
- **THEN** config loading fails
- **AND** the server does not start

#### Scenario: Production rejects insecure remote SMTP

- **GIVEN** `APP_ENV=production`
- **AND** `SMTP_TLS_MODE=none`
- **WHEN** `SMTP_HOST` is not localhost or loopback
- **THEN** config loading fails
- **AND** the server does not start

#### Scenario: Port TLS mismatch is warned

- **WHEN** the server starts with `SMTP_PORT=465` and `SMTP_TLS_MODE=starttls`
- **THEN** startup logs a warning about the common port/TLS mismatch
- **AND** the server may still start if all required SMTP fields are valid
