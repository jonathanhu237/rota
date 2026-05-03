## ADDED Requirements

### Requirement: Application branding settings

The system SHALL persist a single application-wide branding record with `product_name`, `organization_name`, `version`, `created_at`, and `updated_at`. `product_name` SHALL be required after trimming and SHALL be at most 60 runes. `organization_name` SHALL be optional after trimming and SHALL be at most 100 runes. The system SHALL allow arbitrary Unicode text within those length constraints and SHALL NOT truncate submitted values. When no customized value exists, the system SHALL use `product_name = "Rota"` and `organization_name = ""`.

The system SHALL expose `GET /branding` as a public read endpoint returning the current branding fields and version. The system SHALL expose `PUT /branding` as an admin-only endpoint that replaces `product_name` and `organization_name` using optimistic concurrency through the submitted `version`. Database branding values SHALL be the only product/organization name source; environment variables SHALL NOT override or provide fallback branding names.

#### Scenario: Defaults are returned before customization

- **GIVEN** no administrator has customized branding
- **WHEN** any client calls `GET /branding`
- **THEN** the response includes `product_name = "Rota"`
- **AND** the response includes `organization_name = ""`
- **AND** the response includes the current branding `version`

#### Scenario: Admin updates branding

- **GIVEN** an authenticated administrator has read branding version `V`
- **WHEN** the administrator calls `PUT /branding` with `product_name = "排班系统"`, `organization_name = "运营部"`, and `version = V`
- **THEN** the response includes `product_name = "排班系统"`
- **AND** the response includes `organization_name = "运营部"`
- **AND** the returned version is greater than `V`
- **AND** subsequent `GET /branding` calls return the updated values

#### Scenario: Non-admin cannot update branding

- **GIVEN** an authenticated non-admin user
- **WHEN** the user calls `PUT /branding`
- **THEN** the response is HTTP 403 with error code `FORBIDDEN`
- **AND** branding values are not changed

#### Scenario: Anonymous update is rejected

- **GIVEN** no authenticated session
- **WHEN** the client calls `PUT /branding`
- **THEN** the response is HTTP 401 with error code `UNAUTHORIZED`
- **AND** branding values are not changed

#### Scenario: Invalid branding input is rejected

- **WHEN** an administrator calls `PUT /branding` with a blank `product_name`
- **THEN** the response is HTTP 400 with error code `INVALID_REQUEST`
- **WHEN** an administrator calls `PUT /branding` with a product or organization name longer than the allowed maximum
- **THEN** the response is HTTP 400 with error code `INVALID_REQUEST`

#### Scenario: Unicode branding is accepted without truncation

- **WHEN** an administrator calls `PUT /branding` with `product_name = "排班系统（门店 A）"` and `organization_name = "运营部 - 上海"`
- **THEN** the response includes those exact trimmed values
- **AND** neither value is truncated or character-filtered

#### Scenario: Stale branding version is rejected

- **GIVEN** branding is currently at version `V + 1`
- **WHEN** an administrator calls `PUT /branding` with version `V`
- **THEN** the response is HTTP 409 with error code `VERSION_CONFLICT`
- **AND** branding values are not changed
