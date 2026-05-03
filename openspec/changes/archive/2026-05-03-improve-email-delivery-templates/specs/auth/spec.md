## ADDED Requirements

### Requirement: Account emails render localized HTML and text bodies

Invitation, password-reset, email-change confirmation, and email-change notice emails SHALL be rendered from embedded external template files into both a text/plain body and an HTML body. Each account email subject SHALL be prefixed with `[Rota]`. Each actionable email SHALL include a primary CTA in HTML and the complete action URL in both HTML and text bodies. Email-change notice emails SHALL NOT include an action link or raw token.

The system SHALL support exactly the existing user preference languages for account email rendering: `en` and `zh`. Unsupported or missing language input SHALL resolve to `en`.

#### Scenario: Invitation email contains localized CTA and fallback URL

- **GIVEN** an admin creates or resends an invitation for a pending user
- **WHEN** the invitation email is enqueued
- **THEN** the outbox row has `kind = 'invitation'`
- **AND** the subject starts with `[Rota]`
- **AND** the text body contains the setup-password URL
- **AND** the HTML body contains a CTA for setting the password and the complete setup-password URL

#### Scenario: Password reset email contains localized CTA and fallback URL

- **GIVEN** an active user requests a password reset
- **WHEN** the password reset email is enqueued
- **THEN** the outbox row has `kind = 'password_reset'`
- **AND** the subject starts with `[Rota]`
- **AND** the text body contains the setup-password URL
- **AND** the HTML body contains a CTA for resetting the password and the complete setup-password URL

#### Scenario: Email-change confirmation contains localized CTA and fallback URL

- **GIVEN** an authenticated user requests an email change
- **WHEN** the confirmation email to the new address is enqueued
- **THEN** the outbox row has `kind = 'email_change_confirm'`
- **AND** the subject starts with `[Rota]`
- **AND** the text body contains the email-change confirmation URL
- **AND** the HTML body contains a CTA for confirming the email change and the complete confirmation URL

#### Scenario: Email-change notice omits action link

- **GIVEN** an authenticated user requests an email change
- **WHEN** the notice email to the current address is enqueued
- **THEN** the outbox row has `kind = 'email_change_notice'`
- **AND** the subject starts with `[Rota]`
- **AND** the text and HTML bodies include a partially masked rendering of the requested new address
- **AND** neither body contains the raw email-change token
- **AND** neither body contains a confirmation CTA

#### Scenario: Chinese account email is fully localized

- **GIVEN** an account email resolves to language `zh`
- **WHEN** the message is rendered
- **THEN** the subject after `[Rota]` is Chinese
- **AND** the CTA, duration text, security guidance, fallback-link label, and footer are Chinese
- **AND** the message does not mix English duration or action labels into the Chinese body

### Requirement: Account email language selection

The system SHALL choose account email language deterministically. Invitation email language SHALL resolve in this order: invited user's `language_preference`, triggering admin's `language_preference`, triggering admin request `Accept-Language`, then `en`. Password reset email language SHALL resolve in this order: recipient user's `language_preference`, request `Accept-Language`, then `en`. Email-change confirmation and notice email language SHALL resolve in this order: current user's `language_preference`, request `Accept-Language`, then `en`.

`Accept-Language` parsing SHALL only return `zh` or `en`. A header such as `zh-CN,zh;q=0.9,en;q=0.8` SHALL resolve to `zh`; a header such as `en-US,en;q=0.9` SHALL resolve to `en`; unsupported languages SHALL resolve to `en` unless an earlier persisted preference selected another language.

#### Scenario: Invitation uses admin language when invitee has none

- **GIVEN** a pending invitee with `language_preference IS NULL`
- **AND** the admin creating the invitation has `language_preference = 'zh'`
- **WHEN** the invitation email is enqueued
- **THEN** the rendered email language is `zh`

#### Scenario: Invitation uses Accept-Language after persisted preferences

- **GIVEN** a pending invitee with `language_preference IS NULL`
- **AND** the admin creating the invitation has `language_preference IS NULL`
- **AND** the request has `Accept-Language: zh-CN,zh;q=0.9,en;q=0.8`
- **WHEN** the invitation email is enqueued
- **THEN** the rendered email language is `zh`

#### Scenario: Password reset falls back to request language

- **GIVEN** an active user with `language_preference IS NULL`
- **AND** the password-reset request has `Accept-Language: zh-CN,zh;q=0.9`
- **WHEN** the password reset email is enqueued
- **THEN** the rendered email language is `zh`

#### Scenario: Persisted preference beats Accept-Language

- **GIVEN** an active user with `language_preference = 'en'`
- **AND** the triggering request has `Accept-Language: zh-CN,zh;q=0.9`
- **WHEN** an account email is enqueued for that user
- **THEN** the rendered email language is `en`

#### Scenario: Unsupported language falls back to English

- **GIVEN** an account email recipient with `language_preference IS NULL`
- **AND** the triggering request has `Accept-Language: fr-FR,fr;q=0.9`
- **WHEN** the account email is enqueued
- **THEN** the rendered email language is `en`
