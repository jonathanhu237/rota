## Context

The current email-send pattern is:

```
service:
    BEGIN tx
    business work
    COMMIT
    s.emailer.Send(ctx, msg)   ← synchronous, after commit
```

Two failure modes (proposal): blocks on SMTP, loses email if process dies post-commit. The fix is the transactional-outbox pattern — enqueue the email inside the same tx as the business write, and let a worker drain the queue asynchronously. This change implements that without introducing any new infrastructure: the queue is a Postgres table, the worker is a goroutine in the existing backend process.

The producer-side integration is the largest part of the work: five existing send-sites all live "after tx commit" today. Each one moves into the relevant transaction. The tx-routing details vary per site and are settled in D-2.

## Goals / Non-Goals

**Goals:**

- Eliminate dual-write between business write and email send: enqueue must commit atomically with the business row.
- Producers no longer wait for SMTP — the request returns after the enqueue.
- SMTP failures retried with backoff; permanent failure state is observable.
- No infrastructure additions; all in Postgres + the Go process.

**Non-Goals:**

- Real message queue (Redis Streams / RabbitMQ).
- Generic event publishing.
- Templated payloads in the outbox (producer pre-renders).
- Retention sweep of old `sent` rows.
- Multi-instance worker / horizontal scaling.
- Admin UI for outbox.

## Decisions

### D-1. Schema

```sql
CREATE TABLE email_outbox (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT REFERENCES users(id) ON DELETE SET NULL,
    recipient       TEXT NOT NULL,
    subject         TEXT NOT NULL,
    body            TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'sent', 'failed')),
    retry_count     INT NOT NULL DEFAULT 0,
    last_error      TEXT,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at         TIMESTAMPTZ,
    failed_at       TIMESTAMPTZ
);

-- Hot path: the worker's poll. Partial index keeps it tiny and fast.
CREATE INDEX email_outbox_pending_idx
    ON email_outbox (next_attempt_at)
    WHERE status = 'pending';
```

`recipient`, `subject`, `body` mirror the existing `email.Message` struct exactly. Producer continues to call `email.Build*Message(...)` as it does today; the resulting `Message` is what gets stored.

`user_id` is nullable metadata for audit. Invitation and password-reset rows set it to the relevant user id. Future non-user emails can leave it NULL. `ON DELETE SET NULL` preserves the outbox record if the user is deleted before delivery.

`next_attempt_at` drives the worker. On enqueue it is `NOW()`; on retry the worker bumps it forward by the backoff interval; on permanent failure the row is no longer eligible (`status='failed'` filters it out).

`retry_count` ticks per send attempt. `last_error` holds the most recent error message (truncated reasonably client-side, no special length cap on the column).

`created_at` is informational; never read in the hot path.

`sent_at` / `failed_at` are filled when the row reaches a terminal state.

**Alternative considered — store `template_name + payload_json`** so the worker could re-render: rejected. The producer already has all the rendering data and renders it once at enqueue; storing the rendered output is simpler, and the existing `Build*Message` functions don't need to be re-callable from the worker without their full Go-typed inputs.

### D-2. Producer integration per call site

Five sites, each with its own tx-routing.

**(a) `setup.go::sendInvitation` — called from `UserService.CreateUser` and `UserService.ResendInvitation`.**

Today: `setupFlowHelper.sendInvitation` calls `h.emailer.Send` *after* `txManager.WithinTx` returns. Move the call *into* the `WithinTx` callback. The `WithinTx` signature already passes `txUserRepo` and `txTokenRepo`; extend it to also pass `txOutboxRepo`. Inside the callback, after `activatePassword` (or after `createUser` / `issueToken`) but before the tx commits, call `txOutboxRepo.Enqueue(ctx, msg)` where `msg = email.BuildInvitationMessage(...)` is built using the user data already in scope.

Effect: user creation + invitation enqueue are atomic.

**(b) `setup.go::sendPasswordReset` — called from `AuthService.RequestPasswordReset`.**

Same pattern as (a). The `RequestPasswordReset` flow runs inside `txManager.WithinTx` to create the password-reset token; the outbox enqueue joins that callback.

Anti-enumeration constraint: the existing requirement says the response is identical regardless of whether a token was issued. The outbox enqueue happens only when a token is issued (active user); the response timing is unaffected because enqueue is one DB INSERT, not an SMTP round-trip.

**(c) `shift_change.go::notifyRequestReceived` — called after `ShiftChangeService.CreateRequest`.**

Today: the SCRT create path runs `repository.CreateShiftChangeRequest` (its own tx), then post-commit the service composes the email and calls `s.emailer.Send`. Move the enqueue into the same tx as the SCRT insert.

Implementation: `repository.CreateShiftChangeRequest` becomes `CreateShiftChangeRequestTx` taking an in-tx callback, OR the repo method accepts an `enqueueAfterInsert` function pointer. Cleanest shape: rename the repo helper to operate inside the service's caller-provided tx — i.e., the service builds the email message, opens a tx, calls the repo `InsertTx` and then `outboxRepo.EnqueueTx`, then commits.

**(d) `shift_change.go::notifyRequestResolved` — called after approve / reject / cancel.**

Same shape as (c). The apply paths (`ApplyGive`, `ApplySwap`) live in the repository layer with their own tx; the cleanest refactor is to have the repository return whatever it changed, and have the *service* (which knows what email to send) open a tx, call the repository, enqueue the email, commit. This matches how `notifyRequestReceived` is best handled.

If pulling the apply tx into the service is too invasive, an alternative is to pass an `enqueueOnSuccess func(tx *sql.Tx) error` callback into the repository method, called inside the apply tx after the state transition succeeds.

Both shapes work; the design picks the callback approach if changing the apply contract is risky and the tx-pulled-into-service approach if the repo is stable. *Implementation discretion.*

**(e) `publication_pr4.go::cascade-resolved notification on assignment delete.**

The cascade (when admin deletes an assignment, every pending SCRT referring to it transitions to `invalidated`, and each requester gets an email) currently fires emails post-commit. Same shape as (d): pull the tx up to the service or use the callback.

### D-3. Retry policy

```
backoff(retry_count) = min(2^retry_count minutes, 1 hour)

retry_count = 0 → fresh attempt (NOW)
            1 → +2 min
            2 → +4 min
            3 → +8 min
            4 → +16 min
            5 → +32 min
            6 → +60 min  (capped)
            7 → +60 min
            8 → terminal failure (status='failed')
```

After 8 attempts (worst case ~3 hours of wall time) the row goes to `status='failed'`. The audit event `user.invitation.email_failed` fires *here*, not on each individual SMTP error. This makes the audit log signal-rich: "we tried hard, we gave up" beats "single attempt blipped".

Errors that should *not* retry (4xx-class permanent rejection from SMTP — e.g., recipient address rejected) could be detected and immediately moved to `failed`. **Decision:** out of scope for v1; treat all errors as retryable. If permanent-rejection bombs the queue, can add detection later.

### D-4. Worker shape

```go
// In cmd/server/main.go after building dependencies:
go func() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            jobs, err := outboxRepo.Claim(ctx, 10)
            if err != nil {
                logger.Error("outbox claim failed", "error", err)
                continue
            }
            for _, job := range jobs {
                err := emailer.Send(ctx, email.Message{
                    To: job.Recipient, Subject: job.Subject, Body: job.Body,
                })
                if err == nil {
                    _ = outboxRepo.MarkSent(ctx, job.ID)
                    continue
                }
                next := computeBackoff(job.RetryCount + 1)
                if job.RetryCount + 1 >= maxRetries {
                    _ = outboxRepo.MarkFailed(ctx, job.ID, err.Error())
                    audit.Record(ctx, audit.Event{
                        Action: audit.ActionUserInvitationEmailFailed, // or context-derived
                        ...
                    })
                } else {
                    _ = outboxRepo.MarkRetryable(ctx, job.ID, err.Error(), next)
                }
            }
        }
    }
}()
```

`Claim` issues:
```sql
UPDATE email_outbox
SET next_attempt_at = NOW() + INTERVAL '5 minutes' -- short visibility lease
WHERE id IN (
    SELECT id FROM email_outbox
    WHERE status = 'pending' AND next_attempt_at <= NOW()
    ORDER BY next_attempt_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, recipient, subject, body, retry_count;
```

The `FOR UPDATE SKIP LOCKED` makes simultaneous claims skip rows another worker is locking. The short visibility lease makes the reservation survive statement commit, so a second worker cannot immediately reclaim the same rows after the first `Claim` returns. If a process dies after claiming but before `MarkSent` / `MarkRetryable` / `MarkFailed`, the pending rows become eligible again after the lease. Today we still run one worker: single ticker, single batch, sequential sends per batch. 100-person org volume means each tick almost always claims zero rows.

### D-5. Audit-action firing point

Today: `user.invitation.email_failed` fires from `recordInvitationEmailFailure` in `service/user.go` immediately after the producer-side `s.emailer.Send` errors.

Tomorrow: same audit action, same metadata schema, but fires from the worker when a row transitions to `failed` after retry exhaustion. The trigger metadata comes from the outbox row (recipient, error). The audit event records "we permanently couldn't deliver this invitation", not "the first SMTP attempt blipped".

The producer-side `recordInvitationEmailFailure` and equivalent helpers are removed.

### D-6. Service-level injection refactor

```
Before:
  AuthService.emailer email.Emailer
  UserService.emailer email.Emailer (via setupFlows)
  ShiftChangeService.emailer email.Emailer
  PublicationService.emailer email.Emailer

After:
  AuthService.outboxRepo outbox.Repo
  UserService.outboxRepo (via setupFlows)
  ShiftChangeService.outboxRepo
  PublicationService.outboxRepo
  
  email.Emailer is held only by the worker goroutine in main.go.
```

Tests: most service tests mock the emailer today. After the refactor, they mock the outbox repo. A small mock that records `EnqueueTx` calls plus a "no error" / "error" stub is enough for ~all service tests.

### D-7. Migration

```sql
-- +goose Up
-- +goose StatementBegin

CREATE TABLE email_outbox (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT REFERENCES users(id) ON DELETE SET NULL,
    recipient       TEXT NOT NULL,
    subject         TEXT NOT NULL,
    body            TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'sent', 'failed')),
    retry_count     INT NOT NULL DEFAULT 0,
    last_error      TEXT,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at         TIMESTAMPTZ,
    failed_at       TIMESTAMPTZ
);

CREATE INDEX email_outbox_pending_idx
    ON email_outbox (next_attempt_at)
    WHERE status = 'pending';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS email_outbox_pending_idx;
DROP TABLE IF EXISTS email_outbox;

-- +goose StatementEnd
```

No data migration. CI's `migrations-roundtrip` covers Up/Down.

### D-8. Configuration

No new env vars. Existing `EMAIL_MODE` continues to drive whether the worker uses the SMTP emailer or the log-only emailer; producer behavior is identical either way. Existing `SMTP_*` config also unchanged.

The poll interval (5 seconds) and max-retries (8) are constants in the worker source for v1; can become env vars in a follow-up if anyone needs to tune them.

### D-9. Test approach

- **Unit tests** for backoff math, retry-vs-fail decision (in `email_outbox_test.go`).
- **Integration test** for the repository's Enqueue / Claim / MarkSent / MarkRetryable / MarkFailed full cycle, including the partial-index correctness for `pending` rows only (in `email_outbox_db_test.go`).
- **Service tests** swap the emailer mock for an outbox-repo mock; existing test shape preserved with a one-line change in setup.
- **Worker integration test** is optional: the loop structure is simple, the building blocks are tested individually. If we add one, it covers "claim → send-fail → schedule retry → claim again succeeds".

## Risks / Trade-offs

- **Risk:** the producer-side refactor for sites (c)-(e) requires moving tx ownership from repository to service layer. → Mitigation: design D-2 names a fallback (callback-passing) per site so Codex can choose the less invasive option case-by-case. The change is large in line count but each site's modification is mechanical once the pattern is clear.
- **Risk:** worker stops sending if the goroutine panics on an unanticipated input. → Mitigation: wrap each per-job send in `defer recover()`; on panic, log and `MarkFailed` (or skip and continue). The supervising goroutine survives.
- **Risk:** queue can be inspected by anyone with DB access including subject/body content. → Same risk profile as having user data in the users table; we accept it.
- **Trade-off:** sent rows are kept forever. → At the project's email volume this is a non-issue for years. A retention sweep can be added without behavior change.

## Migration Plan

Single shipping unit. After merge:

1. `make migrate-up` adds the `email_outbox` table.
2. The new backend image starts up; the worker goroutine begins polling immediately.
3. Producer code paths now enqueue instead of send-then-and-there.
4. No deprecation period; the change is committed atomically.

If a critical bug is discovered post-merge, a single `git revert` + `make migrate-down` rolls everything back. No persistent data is lost (sessions are ephemeral; the outbox table contains only in-flight emails which would re-trigger on next user action).

## Open Questions

None.
