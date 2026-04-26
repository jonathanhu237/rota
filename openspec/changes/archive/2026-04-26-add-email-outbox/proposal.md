## Why

Today every email send site is a synchronous `s.emailer.Send(ctx, msg)` call after the business transaction commits. Two problems with this:

- **Blocking:** the HTTP request waits for the SMTP round-trip. With `EMAIL_MODE=log` (dev) it is invisible; under real SMTP it is 1-3 seconds in the happy path and 30+ seconds when the provider is sluggish. Admin clicks "邀请用户" and stares at a spinner.
- **Lost emails on crash:** the business transaction commits and *then* the program calls SMTP. If the process crashes / is killed / loses network between those two steps, the email is gone forever. For invitation and password-reset emails, that means the user account exists but the user can never log in until an admin notices and re-invites.

This change adds a Postgres-backed transactional outbox: producers write the email intent into the same DB transaction that creates / mutates the business row. A background worker pulls pending rows, sends them, and updates the row to `sent` or `failed` with retry bookkeeping. Crash-safety: the outbox row is committed atomically with the business row, so any crash either loses both or keeps both. UI latency: the producer no longer waits for SMTP — it writes a row and returns.

Decoupling is intentional and minimal — the existing `email.Emailer` interface and the four `Build*Message` template functions are untouched. The outbox stores the *rendered* `{To, Subject, Body}` triple (not "template name + payload"), so the worker is dumb and the producer keeps owning rendering.

## What Changes

- **New DB schema:** `email_outbox` table with `id BIGSERIAL`, optional `user_id BIGINT REFERENCES users(id) ON DELETE SET NULL`, `recipient TEXT NOT NULL`, `subject TEXT NOT NULL`, `body TEXT NOT NULL`, `status TEXT NOT NULL DEFAULT 'pending' CHECK IN ('pending','sent','failed')`, `retry_count INT NOT NULL DEFAULT 0`, `last_error TEXT`, `next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, `sent_at TIMESTAMPTZ NULL`, `failed_at TIMESTAMPTZ NULL`. A partial index on `(next_attempt_at)` `WHERE status = 'pending'` keeps the worker's poll fast. `user_id` is set for user-scoped emails so terminal-failure audit can target the user precisely.
- **New `OutboxRepository`** at `backend/internal/repository/email_outbox.go` exposing `EnqueueTx(ctx, tx, msg email.Message) error`, `Claim(ctx, batchSize int) ([]OutboxJob, error)` (uses `FOR UPDATE SKIP LOCKED` plus a short `next_attempt_at` visibility lease), `MarkSent(ctx, id int64) error`, and `MarkRetryable(ctx, id, lastErr, backoff) error` / `MarkFailed(ctx, id, lastErr) error`. The repo also exposes a tx-bound `Enqueue` for the few code paths that already hold a `*sql.Tx`.
- **Producer-side refactor (the load-bearing part):** every `s.emailer.Send(ctx, msg)` call site moves *into* the relevant business transaction and becomes `outboxRepo.EnqueueTx(ctx, tx, msg)`. Five call sites:
  - `service/setup.go` — `sendInvitation` (called from `UserService.CreateUser` and `UserService.ResendInvitation`) and `sendPasswordReset` (called from `AuthService.RequestPasswordReset`). The `setupFlowHelper` already runs inside `txManager.WithinTx`; the email enqueue moves into that callback.
  - `service/shift_change.go` — `notifyRequestReceived` (after SCRT create) and `notifyRequestResolved` (after approve/reject/cancel). Each currently sends post-commit; both move into the apply transaction in the relevant repository method, or the service splits the work so the SCRT mutation and the enqueue share one tx.
  - `service/publication_pr4.go` — the cascade-resolved notification on admin assignment delete. Same treatment: enqueue inside the assignment-delete transaction.
- **New worker goroutine in `cmd/server/main.go`:** mirrors the existing session-cleanup goroutine pattern. Polls every 5 seconds; on each tick `Claim`s up to N pending rows, calls `s.emailer.Send` for each, and either `MarkSent` on success or `MarkRetryable` (with exponential backoff up to a max retry count) / `MarkFailed` on terminal failure. Errors are slog-logged; the worker survives any single send failure.
- **Service-level injection:** every service that today takes `email.Emailer` (or sees emailer through `setupFlowHelper`) gains an `outboxRepo` dependency instead. The actual `email.Emailer` is owned by the worker only — services no longer hold a reference. Test mocks reduce to mocking the outbox repo.
- **`audit` capability:** existing audit actions (`user.invitation.email_failed` etc.) currently fire when the synchronous send fails. With outbox they shift meaning: emit when the row transitions to `failed` (after retries exhausted), not on the first SMTP error. The audit action name and metadata stay; only the trigger point moves into the worker.
- **Retry policy (see design D-3):** exponential backoff `2^retry_count` minutes capped at 1 hour, max 8 retries, then `failed`. Failed rows stay in the table indefinitely as a record; admin can inspect.
- **Spec changes:**
  - New `outbox` capability with one requirement covering the enqueue / claim / retry / mark-sent / mark-failed contract.
  - `auth` capability: the existing requirements that mention email send (e.g., invitation, resend, password-reset request) are reworded to say "enqueues an email via the outbox" rather than "sends an email synchronously". The user-visible behavior is unchanged (the user still receives the email, just possibly seconds later).
  - `audit` capability: the trigger-point shift for `user.invitation.email_failed` etc. is documented.

## Capabilities

### New Capabilities

- `outbox`: introduces the transactional email outbox primitive — enqueue inside a business tx, claim with `FOR UPDATE SKIP LOCKED`, retry with backoff, terminal `failed` state. Self-contained in `repository/email_outbox.go` + the worker goroutine.

### Modified Capabilities

- `auth`: redirects existing email send requirements (invitation, password-reset request) through the outbox without changing the user-visible flow.
- `audit`: existing email-failure audit actions fire from the worker (after retry exhaustion), not from the producer's first send attempt.

## Non-goals

- **Switching to a real message queue (Redis Streams, RabbitMQ, Kafka).** The whole point of the outbox is that it composes with the existing Postgres tx. A real MQ reintroduces the dual-write race we're solving. Future re-introduction (relay outbox → Kafka) is a separate change if it's ever needed.
- **Generic event publishing.** The outbox stores `{recipient, subject, body}` — emails only. We are not building a general "publish event of any kind" pipeline.
- **Templated payloads in the outbox.** The producer renders the email at enqueue time. Worker is template-blind.
- **Cleanup of old `sent` rows.** Sent rows stay forever for now (low volume). A retention sweep is left for a future change once the table actually grows.
- **Per-recipient rate limiting / batching.** Out of scope. Worker sends one message per claimed row, sequentially.
- **Multiple worker instances or horizontal scaling.** Single worker in the single backend process. `FOR UPDATE SKIP LOCKED` makes the schema safe for future scale-out, but no scale-out today.
- **Admin UI to view / retry / cancel outbox rows.** Could be useful; not now.

## Impact

- **Backend code:**
  - New: `migrations/000NN_create_email_outbox_table.sql`.
  - New: `backend/internal/repository/email_outbox.go` with `OutboxRepository`.
  - New: `backend/internal/repository/email_outbox_db_test.go` (`//go:build integration`).
  - Modified: `backend/internal/service/setup.go` — `sendInvitation` / `sendPasswordReset` enqueue inside the existing tx callback rather than calling emailer.
  - Modified: `backend/internal/service/shift_change.go` — `notifyRequestReceived` / `notifyRequestResolved` enqueue inside the apply tx (or via a tx-passing helper).
  - Modified: `backend/internal/service/publication_pr4.go` — cascade-resolved notification enqueues inside the delete tx.
  - Modified: `backend/internal/service/user.go`, `service/auth.go` — wiring updates so services hold the outbox repo, not the emailer.
  - Modified: `backend/cmd/server/main.go` — wires the OutboxRepository into services; adds the email-worker goroutine that consumes the outbox and calls the existing `email.Emailer`.
  - Modified: `backend/internal/audit/audit.go` and the call sites that emit `user.invitation.email_failed` — those move from the producer service to the worker, fired only when a row transitions to `failed`.
  - Modified: many `_test.go` files — mocks shift from `email.Emailer` to `outboxRepository` interface.
- **Infrastructure / config:** none. Existing `EMAIL_MODE` / `SMTP_*` envs continue to drive the worker's outbound behavior.
- **Spec:**
  - New `openspec/specs/outbox/spec.md` (one requirement, several scenarios).
  - Modified `auth` and `audit` requirements where email-send wording is now outbox-mediated.
- **No third-party dependencies added.**
- **No frontend changes.**

## Risks / safeguards

- **Risk: producer-side refactor is invasive.** Five send-sites each have to move into a tx context — for `shift_change` apply paths the tx currently lives inside the repository, so the cleanest fix is for the apply repository methods to accept an "and-also-enqueue-this-message" hook that runs in the same tx. **Mitigation:** the design (D-2) settles the exact integration shape per call site so Codex doesn't have to invent it.
- **Risk: worker dies → emails stop sending.** **Mitigation:** the goroutine is restart-on-process-restart; rows stay in `pending` until consumed. Process-level supervision is the responsibility of the deployment (systemd / docker compose `restart: always` etc., already in place for prod). Lazy correctness: a backed-up queue drains on next process start.
- **Risk: send failure storms (SMTP outage) build queue depth.** **Mitigation:** retries use exponential backoff + max attempts; after 8 failures a row goes to `failed` and stops consuming worker time. Queue depth is bounded by the `failed` ceiling.
- **Risk: subject / body in the outbox could leak sensitive data into the DB.** **Mitigation:** the same data was previously transmitted via SMTP unencrypted-by-default; storing it transiently in Postgres is no worse from a confidentiality standpoint, and the DB is already trusted (it stores password hashes, audit logs, etc.). Sent rows could be redacted post-send if this becomes a concern; not in scope.
- **Risk: shift in audit action firing point might surprise admins reading the log.** **Mitigation:** the audit-spec change is explicit about the new firing point. A `user.invitation.email_failed` event now means "we tried 8 times over 4 hours and gave up", which is a clearer signal than "single SMTP attempt errored".
