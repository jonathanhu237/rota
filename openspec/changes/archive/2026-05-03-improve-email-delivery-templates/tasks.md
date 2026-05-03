## 1. Outbox Schema and Repository

- [x] 1.1 Add a goose migration that adds `email_outbox.kind TEXT NOT NULL DEFAULT 'unknown'` and `email_outbox.html_body TEXT NULL`, with a Down migration that drops both columns. Verify: `make migrate-up && make migrate-status`.
- [x] 1.2 Update outbox repository models, `EnqueueTx`, `Claim`, and tests to persist and return `kind` plus optional `html_body` while keeping legacy plain-text rows valid. Verify: `cd backend && go test ./internal/repository -run 'Outbox'`.
- [x] 1.3 Update worker invitation-failure auditing to use `kind = 'invitation'` instead of subject matching. Verify: `cd backend && go test ./cmd/server -run 'OutboxWorker'`.

## 2. Email Message Model and SMTP Delivery

- [x] 2.1 Expand `email.Message` with stable `Kind` and `HTMLBody`, and update all compile-time call sites to set or handle those fields. Verify: `cd backend && go test ./internal/email ./cmd/server ./internal/service`.
- [x] 2.2 Implement SMTP MIME rendering: plain text for messages without HTML, `multipart/alternative` for messages with HTML, UTF-8 headers/body, and text part before HTML part. Verify: `cd backend && go test ./internal/email -run 'SMTPEmailer'`.
- [x] 2.3 Update `LoggerEmailer` to print text body plus `HTML: yes/no` without printing full HTML. Verify: `cd backend && go test ./internal/email -run 'LoggerEmailer'`.
- [x] 2.4 Add per-message send timeout support using `EMAIL_SEND_TIMEOUT`, and ensure timeout failures become retryable outbox errors. Verify: `cd backend && go test ./cmd/server -run 'OutboxWorker'`.

## 3. Embedded Templates

- [x] 3.1 Add embedded external `.html.tmpl` and `.txt.tmpl` files for all six email kinds in `en` and `zh`, with a shared business-minimal HTML layout and no remote assets. Verify: `cd backend && go test ./internal/email -run 'Template'`.
- [x] 3.2 Implement template rendering helpers that produce subject, text body, and HTML body for invitation, password reset, email-change confirmation, email-change notice, shift-change request received, and shift-change resolution. Verify: `cd backend && go test ./internal/email -run 'Build.*Message|Template'`.
- [x] 3.3 Add template verification tests covering every kind and language for `[Rota]` subject prefix, CTA URL or intentionally absent CTA, complete fallback link, localized footer, and HTML structure. Verify: `cd backend && go test ./internal/email`.

## 4. Language Resolution

- [x] 4.1 Implement `Accept-Language` parsing that only resolves `zh` or `en`, with unsupported/missing values falling back to `en`. Verify: `cd backend && go test ./internal/email -run 'Language'`.
- [x] 4.2 Thread request language fallback from handlers into services for invitation, resend invitation, password reset, email change, and user-triggered shift-change flows. Verify: `cd backend && go test ./internal/handler ./internal/service`.
- [x] 4.3 Update account-email language selection: invitee/admin/admin Accept-Language/en for invitations; recipient/request Accept-Language/en for reset and email-change emails. Verify: `cd backend && go test ./internal/service -run 'Invitation|PasswordReset|EmailChange'`.
- [x] 4.4 Update scheduling-email language selection: recipient/request Accept-Language/en for request-triggered emails, recipient/en for system cascade invalidations. Verify: `cd backend && go test ./internal/service -run 'ShiftChange|Publication'`.

## 5. Account Email Flows

- [x] 5.1 Update invitation and resend-invitation producers to enqueue `kind='invitation'`, localized subject/text/html, setup-password CTA, and complete fallback URL. Verify: `cd backend && go test ./internal/service -run 'CreateUser|ResendInvitation'`.
- [x] 5.2 Update password reset producer to enqueue `kind='password_reset'`, localized subject/text/html, reset CTA, and complete fallback URL while preserving anti-enumeration behavior. Verify: `cd backend && go test ./internal/service -run 'PasswordReset'`.
- [x] 5.3 Update email-change confirmation and notice producers to enqueue `kind='email_change_confirm'` and `kind='email_change_notice'`; confirmation includes CTA and fallback URL, notice includes masked email and no action link. Verify: `cd backend && go test ./internal/service ./internal/handler -run 'EmailChange'`.

## 6. Scheduling Email Flows

- [x] 6.1 Extend shift-change email data to carry concrete occurrence dates for requester and counterpart shifts. Verify: `cd backend && go test ./internal/service -run 'ShiftChange'`.
- [x] 6.2 Update request-received emails to enqueue `kind='shift_change_request_received'`, localized subject/text/html, occurrence date summaries, CTA, and complete requests URL. Verify: `cd backend && go test ./internal/service -run 'ShiftChange.*Create|RequestCreated'`.
- [x] 6.3 Update resolution and invalidation emails to enqueue `kind='shift_change_resolved'`, localized outcome labels, occurrence date summaries where known, CTA, and complete requests URL. Verify: `cd backend && go test ./internal/service -run 'ShiftChange.*Resolve|Publication.*Invalidat'`.

## 7. Config Validation

- [x] 7.1 Add `EMAIL_SEND_TIMEOUT` to config loading, `.env.example`, and README environment variable docs. Verify: `cd backend && go test ./internal/config`.
- [x] 7.2 Enforce SMTP required fields for `EMAIL_MODE=smtp`, production-safe `APP_BASE_URL`, and production rejection of remote `SMTP_TLS_MODE=none`. Verify: `cd backend && go test ./internal/config`.
- [x] 7.3 Add startup warnings for common SMTP port/TLS mismatches without failing otherwise valid configs. Verify: `cd backend && go test ./cmd/server -run 'SMTP|TLS|Warning'`.

## 8. End-to-End Verification

- [x] 8.1 Run backend unit and compile checks. Verify: `cd backend && go test ./... && go vet ./... && go build ./...`.
- [x] 8.2 Run SQL integration coverage for the outbox migration/repository changes when Postgres is available. Verify: `cd backend && go test -tags=integration ./...`.
- [x] 8.3 Run OpenSpec validation. Verify: `openspec validate --all --strict`.
- [x] 8.4 Confirm all task checkboxes are completed before archive. Verify: `rg -n '^- \\[ \\]' openspec/changes/improve-email-delivery-templates/tasks.md` returns no rows.
