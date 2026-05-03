## ADDED Requirements

### Requirement: Shift-change emails render localized HTML and text bodies

Shift-change request-received and shift-change resolution emails SHALL be rendered from embedded external template files into both a text/plain body and an HTML body. Each shift-change email subject SHALL be prefixed with `[Rota]`. The email body SHALL contain a CTA to view requests and the complete requests URL in both HTML and text bodies.

Shift-change email language SHALL resolve in this order for user-request-triggered emails: recipient user's `language_preference`, triggering request `Accept-Language`, then `en`. Shift-change emails triggered by system cascade work without a current request, such as assignment deletion invalidation, SHALL resolve in this order: recipient user's `language_preference`, then `en`.

#### Scenario: Direct request email contains localized CTA and fallback URL

- **WHEN** an employee creates a `swap` or `give_direct` request
- **THEN** an email is enqueued to the counterpart with `kind = 'shift_change_request_received'`
- **AND** the subject starts with `[Rota]`
- **AND** the text body contains the requests URL
- **AND** the HTML body contains a CTA to view the request and the complete requests URL

#### Scenario: Pool creation still sends no email

- **WHEN** an employee creates a `give_pool` request
- **THEN** no email is sent at creation

#### Scenario: Resolution email contains localized outcome

- **WHEN** a shift-change request is approved, rejected, claimed, cancelled, or invalidated
- **THEN** an email is enqueued to the requester with `kind = 'shift_change_resolved'`
- **AND** the subject starts with `[Rota]`
- **AND** the subject and body contain the localized outcome label
- **AND** the text and HTML bodies contain the requests URL

#### Scenario: Persisted recipient language beats request language

- **GIVEN** a shift-change email recipient has `language_preference = 'en'`
- **AND** the triggering request has `Accept-Language: zh-CN,zh;q=0.9`
- **WHEN** the shift-change email is enqueued
- **THEN** the rendered email language is `en`

#### Scenario: Request language is used when recipient has no preference

- **GIVEN** a shift-change email recipient has `language_preference IS NULL`
- **AND** the triggering request has `Accept-Language: zh-CN,zh;q=0.9`
- **WHEN** the shift-change email is enqueued
- **THEN** the rendered email language is `zh`

#### Scenario: System cascade falls back to English

- **GIVEN** an assignment deletion invalidates a pending shift-change request
- **AND** the requester has `language_preference IS NULL`
- **WHEN** the invalidation email is enqueued
- **THEN** the rendered email language is `en`

### Requirement: Shift-change emails identify concrete occurrences

Shift-change emails SHALL include the concrete occurrence date for every referenced requester or counterpart shift when that date is known. The rendered date label SHALL be localized:

- English format: `Mon, May 4, 2026, 09:00-12:00 Front Desk Assistant`
- Chinese format: `2026-05-04（周一）09:00-12:00 前台助理`

The system SHALL NOT replace the scheduling validity rules for `occurrence_date`; it SHALL reuse the already-authoritative shift-change request occurrence data for display.

#### Scenario: Request email includes requester occurrence date

- **GIVEN** Alice creates a `give_direct` request for occurrence date `2026-05-04`
- **WHEN** the counterpart email is rendered in English
- **THEN** the shift summary includes `Mon, May 4, 2026`
- **AND** the shift summary includes the shift time range and position name

#### Scenario: Swap email includes both occurrence dates

- **GIVEN** Alice creates a `swap` request with requester occurrence date `2026-05-04`
- **AND** Bob's counterpart occurrence date is `2026-05-05`
- **WHEN** the counterpart email is rendered
- **THEN** the requester shift summary contains `2026-05-04`
- **AND** the counterpart shift summary contains `2026-05-05`

#### Scenario: Chinese shift summary localizes weekday

- **GIVEN** a shift-change email resolves to language `zh`
- **AND** the referenced occurrence date is Monday `2026-05-04`
- **WHEN** the message is rendered
- **THEN** the shift summary contains `2026-05-04（周一）`
- **AND** the summary does not use the English weekday label `Mon`
