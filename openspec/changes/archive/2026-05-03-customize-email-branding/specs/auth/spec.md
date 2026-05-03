## MODIFIED Requirements

### Requirement: Account emails render localized HTML and text bodies

Invitation, password-reset, email-change confirmation, and email-change notice emails SHALL be rendered from embedded external template files into both a text/plain body and an HTML body. Each account email subject SHALL be prefixed with `[<product_name>]`, where `<product_name>` is the current normalized branding product name. Each actionable email SHALL include a primary CTA in HTML and the complete action URL in both HTML and text bodies. Email-change notice emails SHALL NOT include an action link or raw token.

The system SHALL support exactly the existing user preference languages for account email rendering: `en` and `zh`. Unsupported or missing language input SHALL resolve to `en`. Account email bodies, headers, footers, and security guidance SHALL use the configured product name instead of hard-coded `Rota`. When `organization_name` is configured, invitation emails SHALL identify the inviting organization in both text and HTML bodies. Other account emails SHALL include organization name only in the footer. When `organization_name` is blank, copy SHALL remain grammatical and SHALL NOT render an empty organization placeholder.

#### Scenario: Invitation email contains localized CTA, fallback URL, and organization

- **GIVEN** branding has `product_name = "排班系统"` and `organization_name = "运营部"`
- **AND** an admin creates or resends an invitation for a pending user
- **WHEN** the invitation email is enqueued
- **THEN** the outbox row has `kind = 'invitation'`
- **AND** the subject starts with `[排班系统]`
- **AND** the text body contains the setup-password URL
- **AND** the HTML body contains a CTA for setting the password and the complete setup-password URL
- **AND** the text and HTML bodies identify `运营部` as the inviting organization

#### Scenario: Invitation email omits blank organization cleanly

- **GIVEN** branding has `product_name = "Scheduling Portal"` and `organization_name = ""`
- **WHEN** an invitation email is rendered in English
- **THEN** the subject starts with `[Scheduling Portal]`
- **AND** the text and HTML bodies include `Scheduling Portal`
- **AND** neither body contains an empty organization placeholder or doubled punctuation

#### Scenario: Password reset email contains localized CTA and fallback URL

- **GIVEN** branding has `product_name = "排班系统"` and `organization_name = "运营部"`
- **AND** an active user requests a password reset
- **WHEN** the password reset email is enqueued
- **THEN** the outbox row has `kind = 'password_reset'`
- **AND** the subject starts with `[排班系统]`
- **AND** the text body contains the setup-password URL
- **AND** the HTML body contains a CTA for resetting the password and the complete setup-password URL
- **AND** the footer identifies `运营部`
- **AND** the main password-reset body copy does not add organization-specific instructions outside the footer

#### Scenario: Email-change confirmation contains localized CTA and fallback URL

- **GIVEN** branding has `product_name = "排班系统"`
- **AND** an authenticated user requests an email change
- **WHEN** the confirmation email to the new address is enqueued
- **THEN** the outbox row has `kind = 'email_change_confirm'`
- **AND** the subject starts with `[排班系统]`
- **AND** the text body contains the email-change confirmation URL
- **AND** the HTML body contains a CTA for confirming the email change and the complete confirmation URL

#### Scenario: Email-change notice omits action link

- **GIVEN** branding has `product_name = "排班系统"`
- **AND** an authenticated user requests an email change
- **WHEN** the notice email to the current address is enqueued
- **THEN** the outbox row has `kind = 'email_change_notice'`
- **AND** the subject starts with `[排班系统]`
- **AND** the text and HTML bodies include a partially masked rendering of the requested new address
- **AND** neither body contains the raw email-change token
- **AND** neither body contains a confirmation CTA

#### Scenario: Chinese account email is fully localized with branding

- **GIVEN** an account email resolves to language `zh`
- **AND** branding has `product_name = "排班系统"` and `organization_name = "运营部"`
- **WHEN** the message is rendered
- **THEN** the subject after `[排班系统]` is Chinese
- **AND** the CTA, duration text, security guidance, fallback-link label, and footer are Chinese
- **AND** the product and organization names appear where relevant
- **AND** the message does not mix English duration or action labels into the Chinese body
