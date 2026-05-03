## ADDED Requirements

### Requirement: Application branding appears in UI and settings

The frontend SHALL load application branding through the branding API and use `product_name` instead of hard-coded `Rota` in public entry pages and authenticated shell chrome. While branding is loading or unavailable, the frontend SHALL fall back to `product_name = "Rota"` and `organization_name = ""` so pages remain usable. Frontend product identity rendering SHALL NOT rewrite business data such as template names, publication names, seed data labels, or historical records that happen to contain the word `Rota`.

The authenticated Settings page SHALL render an admin-only Branding section. The section SHALL use the existing shadcn form style and SHALL let administrators edit `product_name` and `organization_name`. Non-admin users SHALL NOT see the Branding section. The form SHALL preserve optimistic concurrency by submitting the current branding version and surfacing a localized conflict error when the backend returns `VERSION_CONFLICT`. Outside this admin Settings section, the frontend SHALL NOT render `organization_name` in the sidebar or shell header in this change.

#### Scenario: Public entry pages use configured product name

- **GIVEN** branding has `product_name = "排班系统"`
- **WHEN** a visitor loads the login, forgot-password, or setup-password page
- **THEN** the page renders `排班系统` in the product identity text
- **AND** no hard-coded `Rota` product identity is shown

#### Scenario: Authenticated shell uses configured product name

- **GIVEN** branding has `product_name = "Scheduling Portal"` and `organization_name = "Operations"`
- **WHEN** an authenticated user loads any authenticated page
- **THEN** the sidebar header renders `Scheduling Portal`
- **AND** shell product identity text does not render hard-coded `Rota`
- **AND** the sidebar header does not render `Operations`

#### Scenario: Business data containing Rota is not rewritten

- **GIVEN** branding has `product_name = "Scheduling Portal"`
- **AND** a template is named `Default Rota`
- **WHEN** the template appears in breadcrumbs, tables, or detail views
- **THEN** the template name still renders as `Default Rota`
- **AND** it is not rewritten to `Default Scheduling Portal`

#### Scenario: Admin edits branding from Settings

- **GIVEN** an authenticated administrator opens `/settings`
- **WHEN** the Branding section loads
- **THEN** the section displays current `product_name` and `organization_name`
- **WHEN** the administrator saves valid updated values
- **THEN** the branding mutation sends both names and the current version
- **AND** the Settings page and shell update to the returned branding values

#### Scenario: Non-admin settings hide branding form

- **GIVEN** an authenticated non-admin user opens `/settings`
- **THEN** the Branding section is not rendered

#### Scenario: Branding validation errors are localized

- **GIVEN** an authenticated administrator opens `/settings`
- **WHEN** the administrator submits a blank product name
- **THEN** the form shows a localized validation error and does not call the update API
- **WHEN** the update API returns `VERSION_CONFLICT`
- **THEN** the form shows a localized conflict error and refetches current branding
