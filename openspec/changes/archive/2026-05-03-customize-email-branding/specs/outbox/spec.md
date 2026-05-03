## ADDED Requirements

### Requirement: Transactional email branding context

Transactional email producers SHALL render future email subjects, text bodies, and HTML bodies with the current application branding before enqueueing the outbox row. The rendered outbox row SHALL remain self-contained: `subject`, `body`, and `html_body` SHALL NOT require the worker to read current branding settings at send time.

The normalized branding context SHALL always include a non-empty product name and MAY include an organization name. When branding lookup fails during an email-producing transaction, the producer SHALL return an internal error rather than enqueueing an email with partially missing branding. Existing legacy outbox rows SHALL remain valid and sendable without branding metadata. Branding SHALL NOT modify the SMTP sender or display name; `SMTP_FROM` remains the only source for the `From` header.

#### Scenario: Future outbox rows persist rendered branding

- **GIVEN** current branding has `product_name = "排班系统"` and `organization_name = "运营部"`
- **WHEN** a service enqueues a transactional email
- **THEN** the email outbox row stores a subject rendered with `[排班系统]`
- **AND** the row stores text and HTML bodies rendered with the configured product name
- **AND** the worker sends the persisted values without reading branding settings

#### Scenario: Existing queued rows keep prior branding

- **GIVEN** an outbox row was enqueued with subject `[Rota] Invitation to Rota`
- **WHEN** an administrator later changes `product_name` to `排班系统`
- **THEN** the existing outbox row subject remains `[Rota] Invitation to Rota`
- **AND** the worker sends the existing rendered row unchanged

#### Scenario: Branding does not change SMTP From

- **GIVEN** current branding has `product_name = "排班系统"`
- **AND** `SMTP_FROM = "Rota <noreply@example.com>"`
- **WHEN** the SMTP emailer sends a branded message
- **THEN** the `From` header remains `Rota <noreply@example.com>`
- **AND** only the rendered subject/body/header/footer content reflects `排班系统`

#### Scenario: Branding lookup failure prevents enqueue

- **GIVEN** a transactional email producer cannot load branding settings because the repository returns an error
- **WHEN** the producer attempts to enqueue an email
- **THEN** the producer returns an internal error
- **AND** no outbox row is enqueued with incomplete branding data
