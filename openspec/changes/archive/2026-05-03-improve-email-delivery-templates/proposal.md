## Why

Current transactional emails are plain text, English-only in several flows, and difficult to trust visually in a production handoff. SMTP delivery also has weak guardrails: bad TLS/port combinations can leave the outbox worker stuck on a send attempt, and the worker identifies invitation failures by subject text that will become brittle once subjects are localized and prefixed.

## What Changes

- Render all user-facing transactional emails as business-minimal HTML with a text/plain fallback.
- Cover invitation, password reset, email-change confirmation, email-change notice, shift-change request received, and shift-change resolution emails.
- Localize every subject, body, CTA, weekday, duration, footer, and status label for English and Chinese.
- Prefix all subjects with `[Rota]`.
- Select email language from persisted user preference first, then request `Accept-Language` where available, then English.
- Include concrete occurrence dates in shift-change emails.
- Store email kind and optional HTML body in the outbox so asynchronous sending does not need to reconstruct template state.
- Send multipart/alternative SMTP payloads when an HTML body exists; keep plain text behavior for legacy rows.
- Add per-message send timeout and stricter SMTP/App URL configuration validation.
- Use external `.html.tmpl` and `.txt.tmpl` template files embedded into the Go binary.
- Add test-level template verification for all email kinds and both supported languages.

## Non-goals

- Do not add product name or organization name customization in this change; subjects and templates continue to use `Rota`.
- Do not add a product UI or HTTP endpoint for email template preview.
- Do not add new delivery channels such as push notifications, SMS, or WebSockets.
- Do not add new user preference languages beyond existing `zh` and `en`.
- Do not change frontend branding, seed data names, README project naming, or historical OpenSpec documents.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `outbox`: email intents gain stable kind metadata, optional HTML body persistence, multipart sending, send timeout, and safer SMTP configuration behavior.
- `auth`: invitation, password reset, and email-change emails gain localized HTML/text templates, subject prefixes, CTA fallback links, and language selection rules.
- `scheduling`: shift-change notification emails gain localized HTML/text templates, subject prefixes, concrete occurrence dates, and language selection from the email recipient.

## Impact

- Backend config: add `EMAIL_SEND_TIMEOUT` and validation for SMTP/App URL combinations.
- Database: migrate `email_outbox` with `kind TEXT NOT NULL DEFAULT 'unknown'` and `html_body TEXT NULL`.
- Backend email package: expand `email.Message`, introduce embedded template rendering, produce multipart SMTP payloads, and keep logger output readable without dumping HTML.
- Backend services/handlers: pass request language fallback and recipient language preference into email builders.
- Backend repositories/outbox worker: persist and read `kind`/`html_body`; audit invitation failures by `kind` rather than subject text.
- Tests: update unit/integration tests for outbox schema, SMTP payloads, language selection, template rendering, shift-change occurrence dates, and config validation.
