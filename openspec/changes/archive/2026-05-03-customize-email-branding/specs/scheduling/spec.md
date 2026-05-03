## MODIFIED Requirements

### Requirement: Shift-change emails render localized HTML and text bodies

Shift-change request-received and shift-change resolution emails SHALL be rendered from embedded external template files into both a text/plain body and an HTML body. Each shift-change email subject SHALL be prefixed with `[<product_name>]`, where `<product_name>` is the current normalized branding product name. The email body SHALL contain a CTA to view requests and the complete requests URL in both HTML and text bodies. Shift-change email layout headers and footers SHALL use the configured product name instead of hard-coded `Rota`. When `organization_name` is configured, shift-change email footers SHALL include it; main shift-change body copy SHALL NOT add organization-specific scheduling facts outside the footer.

Shift-change email language SHALL resolve in this order for user-request-triggered emails: recipient user's `language_preference`, triggering request `Accept-Language`, then `en`. Shift-change emails triggered by system cascade work without a current request, such as assignment deletion invalidation, SHALL resolve in this order: recipient user's `language_preference`, then `en`.

#### Scenario: Direct request email contains localized CTA and fallback URL

- **GIVEN** branding has `product_name = "排班系统"`
- **WHEN** an employee creates a `swap` or `give_direct` request
- **THEN** an email is enqueued to the counterpart with `kind = 'shift_change_request_received'`
- **AND** the subject starts with `[排班系统]`
- **AND** the text body contains the requests URL
- **AND** the HTML body contains a CTA to view the request and the complete requests URL

#### Scenario: Pool creation still sends no email

- **WHEN** an employee creates a `give_pool` request
- **THEN** no email is sent at creation

#### Scenario: Resolution email contains localized outcome and branded footer

- **GIVEN** branding has `product_name = "Scheduling Portal"` and `organization_name = "Operations"`
- **WHEN** a shift-change request is approved, rejected, claimed, cancelled, or invalidated
- **THEN** an email is enqueued to the requester with `kind = 'shift_change_resolved'`
- **AND** the subject starts with `[Scheduling Portal]`
- **AND** the subject and body contain the localized outcome label
- **AND** the text and HTML bodies contain the requests URL
- **AND** the footer identifies `Operations`

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
