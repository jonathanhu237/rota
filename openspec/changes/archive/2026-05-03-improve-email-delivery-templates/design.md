## Context

Rota already sends account and scheduling emails through an outbox table and a background worker. That keeps request paths decoupled from SMTP, but the current message model only stores one plain-text body and the SMTP writer emits a minimal RFC-unsafe payload with no MIME structure. The user-visible result is functional but unpolished, and several flows are hard-coded to English despite users having a persisted `language_preference`.

This change spans the email package, outbox repository and worker, auth/user services, scheduling services, config validation, SQL migrations, and tests. It does not alter the publication state machine or assignment correctness rules.

## Goals / Non-Goals

**Goals:**

- Render every user-facing transactional email as business-minimal HTML plus text/plain fallback.
- Keep all email content asynchronous and persisted before worker delivery.
- Fully localize supported email content for `en` and `zh`.
- Make language selection deterministic from recipient preference, request fallback, and English default.
- Add enough metadata to the outbox for reliable worker behavior and future debugging.
- Prevent obvious SMTP/App URL misconfiguration from silently breaking delivery.
- Avoid long-running SMTP sends blocking the worker indefinitely.

**Non-Goals:**

- Product name or organization name customization.
- Product UI or HTTP endpoint for email template previews.
- New email delivery provider integrations or non-email channels.
- Additional user-facing language preferences beyond `en` and `zh`.
- Frontend branding changes.

## Decisions

### External template files with standard-library rendering

Email templates will live under `backend/internal/email/templates/` and be embedded with `//go:embed`. HTML templates will use `html/template`; text templates will use `text/template`. Both formats are external files.

Rejected alternative: Go string builders. They are simpler initially, but they make HTML hard to inspect and make complete bilingual content harder to maintain.

Rejected alternative: adding MJML, React Email, or another template framework. Those tools improve authoring ergonomics for larger email systems, but they add a dependency, build step, or runtime model this backend does not currently need.

### Outbox persists rendered text, rendered HTML, and kind

`email_outbox` will retain `body TEXT NOT NULL` as the text/plain body and add:

- `kind TEXT NOT NULL DEFAULT 'unknown'`
- `html_body TEXT NULL`

New producers will explicitly enqueue a stable kind and rendered HTML body. Existing rows are compatible because `kind` defaults to `unknown` and `html_body` may be NULL.

Rejected alternative: store template name and variables, then render in the worker. That makes worker output depend on code deployed after enqueue time and can change already-queued emails unexpectedly.

### SMTP sends multipart only when HTML exists

`email.Message` will gain `Kind` and `HTMLBody`. SMTP delivery will send:

- `text/plain; charset=UTF-8` when `HTMLBody == ""`
- `multipart/alternative` containing text/plain first and text/html second when `HTMLBody != ""`

Headers will include MIME-Version, Content-Type, and Content-Transfer-Encoding. The existing logger emailer will print To, Subject, the text body, and `HTML: yes/no`; it will not dump full HTML to stdout.

### Language selection is explicit and local

Email builders will receive a resolved language string (`en` or `zh`). Services are responsible for resolving language near the triggering request:

- Invitation: invited user preference, then admin preference, then admin `Accept-Language`, then `en`.
- Password reset and request-triggered account emails: recipient preference, then request `Accept-Language`, then `en`.
- System-triggered scheduling cascade emails: recipient preference, then `en`.
- Shift-change request/resolution emails triggered by a user request: recipient preference, then request `Accept-Language`, then `en`.

`Accept-Language` parsing will only select `zh` or `en`; unsupported languages fall back to `en`.

Rejected alternative: infer language from email domain or recipient name. That is unreliable and hard to test.

### Shift-change emails include occurrence dates

`ShiftRef` will carry the concrete occurrence date where available. Existing scheduling data already records `occurrence_date` on shift-change requests, so the service can pass it into template data. The rendered format is locale-specific:

- English: `Mon, May 4, 2026, 09:00-12:00 Front Desk Assistant`
- Chinese: `2026-05-04（周一）09:00-12:00 前台助理`

### Configuration hardening

New config:

- `EMAIL_SEND_TIMEOUT`, default `30s`.

Validation and startup behavior:

- `EMAIL_MODE=smtp` with empty `SMTP_HOST` or `SMTP_FROM` fails startup.
- `APP_ENV=production` with missing, localhost, or loopback `APP_BASE_URL` fails startup.
- `SMTP_TLS_MODE=none` fails in production unless host is localhost/loopback.
- Port/TLS mismatches log warnings, not startup failures: port 465 without `implicit`, or port 587 with `implicit`.

No new HTTP error codes are introduced; these are startup configuration errors.

### Template verification by tests, not product UI

Tests will render every supported email kind in both languages and assert stable key content: subject prefix, CTA URL, fallback URL, HTML structure, text body, localized weekday/duration/status labels, and absence of raw token outside intended links. The change will not add a product route for previewing emails.

## Risks / Trade-offs

- HTML email compatibility is uneven across clients -> Use simple table/block layout, inline-safe CSS, no remote images, no external fonts, no JavaScript, and always include text fallback.
- More templates increase maintenance surface -> Keep shared layout partials and test every kind/language combination.
- `Accept-Language` can vary between devices -> Use it only after persisted recipient/admin preference is unavailable.
- Migration adds columns to a live outbox table -> Add nullable/defaulted columns with no destructive rewrite; existing pending rows remain sendable as plain text.
- Subject prefixes change audit logic risk -> Use outbox `kind` for invitation failure auditing instead of subject matching.

## Migration Plan

1. Add a goose migration:
   - Up: `ALTER TABLE email_outbox ADD COLUMN kind TEXT NOT NULL DEFAULT 'unknown'; ADD COLUMN html_body TEXT NULL;`
   - Down: drop `html_body`, then `kind`.
2. Update repository structs and queries to read/write `kind` and `html_body`.
3. Update producers to set explicit kinds for all new emails.
4. Update worker to send `HTMLBody` when present and audit invitation failures by `kind`.
5. Deploy code and migration together. Existing rows with `kind='unknown'` and `html_body IS NULL` continue to send as legacy plain-text emails.

Rollback is safe while no code relies on the new columns. After rollback, any emails enqueued only in the old shape still send; HTML bodies queued during the new version are lost if the Down migration runs.

## Open Questions

None. Product/organization name customization is intentionally deferred.
