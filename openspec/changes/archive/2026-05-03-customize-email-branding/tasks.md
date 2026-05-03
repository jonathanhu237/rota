## 1. Branding Persistence and API

- [x] 1.1 Add a goose migration for singleton `app_branding` with defaults, versioning, timestamps, validation checks, and Down migration. Verify: `make migrate-up && make migrate-status`.
- [x] 1.2 Add backend model and repository support for reading, upserting, and optimistic-concurrency updating branding. Verify: `cd backend && go test ./internal/repository -run 'Branding'`.
- [x] 1.3 Add branding service validation for required product name, length limits, trimming, default normalization, and version conflict mapping. Verify: `cd backend && go test ./internal/service -run 'Branding'`.
- [x] 1.4 Add public `GET /branding` and admin-only `PUT /branding` handlers with error mapping for `INVALID_REQUEST`, `UNAUTHORIZED`, `FORBIDDEN`, and `VERSION_CONFLICT`. Verify: `cd backend && go test ./internal/handler -run 'Branding'`.
- [x] 1.5 Register branding repository/service/handler routes in server startup. Verify: `cd backend && go test ./cmd/server`.

## 2. Email Branding Injection

- [x] 2.1 Extend email template data with normalized branding context and update subject builders to use `[<product_name>]`. Verify: `cd backend && go test ./internal/email -run 'Build.*Message|Template'`.
- [x] 2.2 Update account email templates in both languages to use product name, organization-aware invitation copy, and organization-aware footers. Verify: `cd backend && go test ./internal/email -run 'Account.*Branding|Template'`.
- [x] 2.3 Update shift-change email templates in both languages to use product name in subject/header/footer and organization name only in footers. Verify: `cd backend && go test ./internal/email -run 'ShiftChange.*Branding|Template'`.
- [x] 2.4 Wire branding lookup into invitation, resend invitation, password reset, email-change, shift-change request, and shift-change resolution producers before outbox enqueue. Verify: `cd backend && go test ./internal/service -run 'Invitation|PasswordReset|EmailChange|ShiftChange|Publication'`.
- [x] 2.5 Ensure branding lookup failure prevents enqueue, already-rendered outbox rows remain worker-sendable without branding metadata, and branding does not alter `SMTP_FROM`. Verify: `cd backend && go test ./internal/service ./cmd/server ./internal/email -run 'Branding|OutboxWorker|SMTPEmailer'`.

## 3. Frontend Branding UI

- [x] 3.1 Add frontend branding types, query, mutation, and fallback values. Verify: `cd frontend && pnpm test -- --run branding`.
- [x] 3.2 Use branding query on public login, forgot-password, setup-password, and authenticated shell/sidebar product identity surfaces without rendering organization name in shell/sidebar. Verify: `cd frontend && pnpm test -- --run 'login|setup|sidebar|branding'`.
- [x] 3.3 Add an admin-only Branding card to `/settings` using shadcn-style inputs and existing form patterns. Verify: `cd frontend && pnpm test -- --run settings`.
- [x] 3.4 Add localized validation and API-error handling for blank product name, length limits, and `VERSION_CONFLICT`. Verify: `cd frontend && pnpm test -- --run branding`.

## 4. Documentation and Specs

- [x] 4.1 Confirm branding introduces no environment variable fallback/override and update README only if operational notes are needed. Verify: `git diff -- .env.example README.md`.
- [x] 4.2 Run OpenSpec validation after implementation. Verify: `openspec validate --all --strict`.

## 5. End-to-End Verification

- [x] 5.1 Run backend unit and compile checks. Verify: `cd backend && go test ./... && go vet ./... && go build ./...`.
- [x] 5.2 Run SQL integration coverage for the branding migration/repository changes when Postgres is available. Verify: `cd backend && go test -tags=integration ./...`.
- [x] 5.3 Run frontend checks. Verify: `cd frontend && pnpm lint && pnpm test && pnpm build`.
- [x] 5.4 Confirm all task checkboxes are completed before archive. Verify: `rg -n '^- \\[ \\]' openspec/changes/customize-email-branding/tasks.md` returns no rows.
