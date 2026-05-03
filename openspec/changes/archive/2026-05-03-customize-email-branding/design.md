## Context

The previous email-template change made transactional emails presentable and localized, but the rendered content still hard-codes `Rota` in subjects, headers, copy, footers, tests, and a few frontend entry points. There is no organization/application settings model today; the closest persisted preferences are per-user language and theme fields.

This change introduces a single application-wide branding record. It intentionally does not create multi-tenancy or per-team branding. Emails remain rendered before enqueue and stored in the outbox, so branding changes affect future emails only.

## Goals / Non-Goals

**Goals:**

- Persist a configurable product name and organization name with safe defaults.
- Let administrators update branding from the existing Settings page.
- Let public entry pages and authenticated shell UI read branding without coupling to a user session.
- Render future transactional emails with the configured product name in subject prefix, layout header, body copy, and footer.
- Render organization name in invitation copy and email footers when configured.
- Render product name, but not organization name, in frontend shell identity surfaces.
- Keep already enqueued outbox rows stable and unchanged.

**Non-Goals:**

- No multi-tenant or per-publication branding.
- No logo upload, image hosting, arbitrary HTML, custom CSS, or accent-color editing.
- No SMTP sender/from-address customization beyond existing `SMTP_FROM`.
- No environment-variable fallback or override for branding values.
- No per-language branding fields.
- No backfill or re-render of existing outbox rows.
- No additional languages.

## Decisions

### Store branding in a singleton table

Add an `app_branding` table with a singleton primary key, `product_name`, `organization_name`, `version`, `created_at`, and `updated_at`. The migration inserts the default row with `product_name = 'Rota'` and `organization_name = ''`.

Validation:

- `product_name`: trimmed, required, max 60 runes, not truncated.
- `organization_name`: trimmed, optional, max 100 runes, not truncated.
- characters: any Unicode text is allowed; safety comes from HTML/React escaping, not a whitelist.
- `version`: required on update for optimistic concurrency.

Rejected alternative: environment variables only. They would work for deployment branding but would not satisfy administrator customization from the UI and would require restarts for changes.

Rejected alternative: environment variables as fallback/override. Mixing DB and env would create unclear precedence when an administrator updates branding from the UI, so the DB row is the only source of truth after migration.

Rejected alternative: storing branding on users. Product and organization identity is application-wide, not personal preference.

Rejected alternative: per-language product and organization names. Product/organization names are treated as proper names for this scope; multilingual fields can be added later if there is a concrete localization requirement.

### Public read, admin-only update

Expose:

- `GET /branding`: public, returns safe branding fields and version.
- `PUT /branding`: admin-only, updates both names with optimistic concurrency.

The read endpoint is public because login, setup-password, and forgot-password pages need product identity before a session exists. The exposed values are intended to be visible in emails and UI, so they are not secret.

Error codes:

- `INVALID_REQUEST` (400): malformed JSON, unknown fields, blank/too-long product name, too-long organization name, or invalid version.
- `UNAUTHORIZED` (401): unauthenticated update.
- `FORBIDDEN` (403): authenticated non-admin update.
- `VERSION_CONFLICT` (409): update version is stale.

### Email builders receive branding context

Extend email template data with a branding value. The normalized branding always has a non-empty product name and an optional organization name. Existing service email producers fetch branding before building each email and pass it into invitation, password reset, email-change, and shift-change builders.

Rendered output rules:

- Subject prefix becomes `[<product_name>]`.
- Layout header uses `<product_name>`.
- Account references use `<product_name> account` instead of `Rota account`.
- Invitation copy includes organization name when configured: `<organization_name> invited you to <product_name>`.
- Footer uses product name, and includes organization name when configured.
- Non-invitation account emails and shift-change emails use organization name only in the footer, not in the main business copy.
- SMTP `From` display name remains controlled only by `SMTP_FROM`; branding does not rewrite it.

Rejected alternative: render branding in the worker. That would make queued emails depend on settings at send time, contradicting the existing outbox contract that stores rendered subject/text/html.

### Frontend caches branding as application metadata

Add a branding query in `frontend/src/lib/queries.ts`. Public auth pages and authenticated layout read it with a fallback to `{ product_name: "Rota", organization_name: "" }` so UI remains usable if the query is loading or fails.

Settings gains an admin-only Branding card using the same shadcn form style as profile/preferences. Non-admin users do not see the card. The sidebar header and entry/auth pages use the configured product name instead of literal `Rota`. Organization name is editable in Settings and used in emails, but it is not rendered in the sidebar or shell header in this change.

Rejected alternative: pushing branding through `/auth/me`. That would not cover unauthenticated pages and would make public application metadata depend on auth state.

## Risks / Trade-offs

- Branding row missing after partial migration -> repository `Get` returns in-code defaults and update can upsert the singleton row.
- Public branding endpoint reveals organization name -> this is intended public presentation data already sent in emails; secrets remain excluded.
- Admin changes product name while emails are queued -> queued rows keep prior rendered content by design; only future rows change.
- Product name too long can break email subjects/sidebar layout -> validation caps names before save; rendering does not truncate the stored value.
- More dynamic email copy increases template-test surface -> tests must cover default branding, custom product name, organization-name injection, and blank organization fallback in both languages.

## Migration Plan

1. Add goose migration creating `app_branding` and inserting the singleton default row.
2. Add model/repository/service/handler layers and route registration.
3. Wire branding provider into account and scheduling email producers.
4. Update email templates and frontend strings to consume branding.
5. Add tests and run backend/frontend checks.

Rollback drops the branding table. Code rollback restores hard-coded `Rota`. Already queued emails remain sendable because outbox rows contain rendered content.

## Open Questions

None for this scope. Logo/color/theming customization remains intentionally deferred.
