## Why

Transactional emails and the authenticated shell still expose `Rota` as a hard-coded product identity. Before delivery, administrators need to present the system as their own scheduling product and organization, and emails should clearly show who the message is from without forking templates.

## What Changes

- Add an application branding/settings capability with persisted `product_name` and `organization_name` as the single source of truth.
- Expose public read and admin-only update APIs for branding settings, with validation and optimistic concurrency.
- Expose read access to the current branding so unauthenticated entry pages and the authenticated shell can render the configured product identity.
- Add an admin branding form to Settings using existing shadcn-style controls.
- Replace hard-coded `Rota` in transactional email subjects, header, body copy, and footer with the configured product name.
- Inject organization name into invitation copy and email footers when configured, so recipients know which organization invited or contacted them.
- Update frontend product identity surfaces to use product name, while keeping organization name out of the shell/sidebar for now.
- Keep safe defaults: `product_name = "Rota"` and empty `organization_name` when no administrator has configured branding.
- Preserve rendered-email persistence: outbox rows continue storing fully rendered subject/text/html at enqueue time.

## Non-goals

- No multi-tenancy. Branding is a single application-wide setting, not per department, team, or customer.
- No logo upload, remote image hosting, arbitrary HTML, custom CSS, or theme-color editing in this change.
- No environment-variable fallback or override for product/organization names.
- No SMTP sender/from-address customization beyond existing `SMTP_FROM`.
- No per-language product or organization name fields.
- No new email delivery provider, push, SMS, or WebSocket notifications.
- No additional supported email languages beyond the existing `en` and `zh`.
- No historical outbox re-rendering; already enqueued emails keep their original rendered content.

## Capabilities

### New Capabilities

- `branding`: persisted application branding settings, public read API, admin update API, and frontend settings UI.

### Modified Capabilities

- `outbox`: transactional email rendering uses branding settings when generating persisted subject, text, and HTML content.
- `auth`: account transactional email requirements use the configured product and organization names instead of hard-coded `Rota`.
- `scheduling`: shift-change transactional email requirements use the configured product name instead of hard-coded `Rota`.
- `frontend-shell`: public entry pages and authenticated shell render product identity, and Settings exposes admin-only branding edits in the established UI style.

## Impact

- Database: add a singleton branding/settings table with defaults and an optimistic concurrency version.
- Backend model/repository/service/handler: add branding settings read/update flow and admin-only update authorization.
- Backend email package and producers: pass branding context into all transactional email builders.
- Frontend queries/types/i18n/settings UI: add branding query/mutation and an admin-only Settings section.
- Tests: add backend unit/integration coverage for validation, persistence, authorization, email injection, and frontend form/query coverage.
