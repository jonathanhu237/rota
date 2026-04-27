## 1. Schema migration

- [x] 1.1 Add `migrations/00018_extend_setup_tokens_for_email_change.sql` per design D-1: drop the existing `purpose` CHECK and re-add it with `IN ('invitation','password_reset','email_change')`; add nullable `new_email TEXT` column; add the table-level CHECK that ties `new_email IS NOT NULL ⇔ purpose = 'email_change'`. Down reverses these steps. Verify by `make migrate-up && make migrate-down 1 && make migrate-up`.
- [x] 1.2 Re-seed dev DB after migration: `make seed SCENARIO=basic`. Verify by `make migrate-status`.

## 2. Backend model + repository

- [x] 2.1 Update `backend/internal/model/setup_token.go` (or wherever `UserSetupToken` lives): extend `Purpose` enum string set to include `"email_change"`; add nullable `NewEmail *string` field. Verify by `cd backend && go build ./...`.
- [x] 2.2 Update `backend/internal/repository/setup_token.go` (or equivalent): SELECT/INSERT include `new_email` column. New helper `IssueEmailChangeToken(ctx, tx, userID, newEmail, ttl) (raw string, err error)` per design D-2 step 7-8. Verify by `go vet ./...`.
- [x] 2.3 Update `backend/internal/repository/user.go`: new method `UpdateEmail(ctx, tx, userID, newEmail) error` that bumps `version` in the same UPDATE. Verify by integration test in 5.4.

## 3. Backend service layer

- [x] 3.1 Add `service/auth.go` (or `service/user.go`) method `RequestEmailChange(ctx, viewerID, newEmail, currentPassword) error` per design D-2. Validation: bcrypt compare current; reject `ErrInvalidCurrentPassword`; `mail.ParseAddress` for shape; reject `ErrInvalidRequest` if same as current; check uniqueness across `users` (any status) → `ErrEmailAlreadyExists`; invalidate prior `email_change` tokens; insert new token with `expires_at = now + 24h`; enqueue confirmation email to new address + heads-up email to old address (both via `OutboxRepository.EnqueueTx`); emit `user.email_change.request` audit. Verify by unit tests in 5.1.
- [x] 3.2 Add `service/auth.go` method `ConfirmEmailChange(ctx, rawToken) error` per design D-3. Validate token shape → `ErrInvalidToken`; resolve via `purpose = 'email_change'` → `ErrTokenNotFound`; `ErrTokenUsed` / `ErrTokenExpired` checks; CAS mark used; re-check email uniqueness → `ErrEmailAlreadyExists`; `UpdateEmail`; `DeleteAllSessions(userID)`; invalidate sibling unused tokens across all purposes; emit `user.email_change.confirm` audit with `revoked_session_count`. Verify by unit tests in 5.1.
- [x] 3.3 Update `service/auth.go` setup-password lookup to filter by `purpose IN ('invitation','password_reset')` so `email_change` tokens cannot be redeemed as setup-password (per design D-3 / spec scenario "email_change token cannot be redeemed via setup-password"). Verify by 5.5.

## 4. Backend handler layer

- [x] 4.1 Add `backend/internal/handler/user.go` route `POST /users/me/email-change-request` per design D-2. DTO: `{ new_email, current_password }` with `DisallowUnknownFields`. Map service errors: `ErrInvalidCurrentPassword` → 401 / `INVALID_CURRENT_PASSWORD`; `ErrInvalidRequest` (bad shape, same-as-current) → 400 / `INVALID_REQUEST`; `ErrEmailAlreadyExists` → 409 / `EMAIL_ALREADY_EXISTS`. On success return 202. Verify by `go build ./...`.
- [x] 4.2 Add `backend/internal/handler/auth.go` route `POST /auth/confirm-email-change` per design D-3. DTO: `{ token }`. No middleware. Map errors: token-rejection set per the existing setup-token vocabulary; `ErrEmailAlreadyExists` → 409 / `EMAIL_ALREADY_EXISTS`. On success return 204. Verify by `go build ./...`.
- [x] 4.3 Wire both routes in `backend/cmd/server/main.go`. Verify by route table grep + `go build ./...`.

## 5. Backend tests

- [x] 5.1 Service-layer unit tests for `RequestEmailChange` and `ConfirmEmailChange` covering every spec scenario in the change-folder delta (wrong password, same-as-current, collision, success enqueues 2 emails, re-request invalidates prior, token rejection vocabulary, race on confirm, concurrent CAS). Verify by `go test ./internal/service/...`.
- [x] 5.2 Handler-layer test for `POST /users/me/email-change-request`: DTO rejects unknown fields; valid request returns 202; audit row exists. Verify by `go test ./internal/handler/...`.
- [x] 5.3 Handler-layer test for `POST /auth/confirm-email-change`: anonymous (no cookie) request succeeds with valid token; token-rejection HTTP/code mapping correct. Verify by `go test ./internal/handler/...`.
- [x] 5.4 Integration test (with DB): end-to-end. Login as Alice (one session); call request endpoint; inspect outbox table for two rows (confirmation to new + notice to old); extract raw token from confirmation outbox body or from the issued token (test helper); call confirm endpoint anonymously; verify `users.email` swapped, all sessions deleted, audit rows present. Verify by `go test -count=1 -tags=integration ./internal/handler/...`.
- [x] 5.5 Service-layer test that an `email_change` token cannot be redeemed via `/auth/setup-password` (returns `TOKEN_NOT_FOUND`). Verify by `go test ./internal/service/...`.

## 6. Backend email templates

- [x] 6.1 Add `email_change_confirm` to the existing in-code email renderer registry. Body contains the generated confirmation URL and a 24-hour expiry note. Verify by template-render unit test.
- [x] 6.2 Add `email_change_notice` to the existing in-code email renderer registry. Body explains a change-request was made, includes partial-masked `new_email` (e.g., `a***@example.com`), no actionable link. Helper `PartialMaskEmail(s string) string` lives next to the renderer. Verify by unit test for the helper.
- [x] 6.3 Wire the two new templates into the renderer registry. Verify by `go test ./internal/email/...`.

## 7. Frontend types + API

- [x] 7.1 Update `frontend/src/components/settings/settings-api.ts` to add `requestEmailChangeMutation` (POST `/users/me/email-change-request`). Verify by `pnpm tsc --noEmit`.
- [x] 7.2 Add `frontend/src/components/settings/settings-schemas.ts` zod schema for the request body. Verify by `pnpm tsc --noEmit`.
- [x] 7.3 Add a public-route API helper for the confirm endpoint (no auth). Verify by `pnpm tsc --noEmit`.

## 8. Frontend settings dialog

- [x] 8.1 Add `frontend/src/components/settings/email-form.tsx`: dialog component opened from the new "Email" section in the settings page. React Hook Form + zod (`zod/v3`) with `new_email` (email shape) and `current_password` fields; submit calls `requestEmailChangeMutation`; on success show in-dialog "确认邮件已发送至 <new_email>" message and auto-close after a moment; map server errors to inline form errors. Verify by component test.
- [x] 8.2 Update `frontend/src/routes/_authenticated/settings.tsx` to render the new "Email" section between Profile and Password. Show current `users.email` read-only and a button that opens the dialog. Verify by `pnpm tsc --noEmit` + component test.

## 9. Frontend public confirmation route

- [x] 9.1 Add `frontend/src/routes/auth/confirm-email-change.tsx` (public, anonymous). On mount: read `?token=` and call the confirm endpoint. Render branches per design D-5: success → "邮箱已更新，请使用新邮箱重新登录" + button to `/login`; the four token-rejection error variants; the `EMAIL_ALREADY_EXISTS` branch. Verify by component test.

## 10. Frontend tests + i18n

- [x] 10.1 `email-form.test.tsx`: invalid-email-shape rejected client-side; happy path calls mutation; server error mapped to form. Verify by `pnpm test settings/email-form`.
- [x] 10.2 `confirm-email-change.test.tsx`: success branch renders re-login CTA; each error code branch renders the matching message. Verify by `pnpm test confirm-email-change`.
- [x] 10.3 Add new i18n strings under `frontend/src/i18n/locales/{en,zh}.json`: settings Email section labels; dialog labels and placeholders; success/error toasts and messages; confirmation page labels. Verify by `pnpm lint && pnpm tsc --noEmit`.

## 11. Spec sync

- [x] 11.1 Confirm the change-folder spec delta at `openspec/changes/change-email-flow/specs/auth/spec.md` matches the implemented behavior (2 ADDED requirements + 5 MODIFIED). Do not edit `openspec/specs/auth/spec.md` directly — `/opsx:archive` syncs it.

## 12. Final gates

- [x] 12.1 `cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./... && govulncheck ./...`. All clean.
- [x] 12.2 `cd frontend && pnpm lint && pnpm test && pnpm build`. All clean.
- [x] 12.3 Manual smoke: log in as Alice → 设置 → 更改邮箱 → enter `alice2@example.com` + correct password → observe success state. Inspect outbox table (two rows). Extract token from the confirmation outbox body (or use the docker-postgres SELECT) → open `<app>/auth/confirm-email-change?token=<raw>` in a fresh incognito window → observe success state → click "重新登录" → log in with new email succeeds, old email returns `INVALID_CREDENTIALS`. Original tab refresh → session expired.
- [x] 12.4 `openspec validate change-email-flow --strict`. Clean.
