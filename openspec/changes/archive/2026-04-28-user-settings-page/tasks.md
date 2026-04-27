## 1. Schema migration

- [x] 1.1 Add `migrations/00017_add_user_preferences.sql` with `+goose Up` adding `language_preference TEXT NULL CHECK (... IN ('zh','en'))` and `theme_preference TEXT NULL CHECK (... IN ('light','dark','system'))` to `users`, and `+goose Down` dropping both columns. Verify by `make migrate-up && make migrate-down 1 && make migrate-up`.
- [x] 1.2 Re-seed dev DB after migration: `make seed SCENARIO=basic`. Verify by `make migrate-status`.

## 2. Backend model + repository

- [x] 2.1 Update `backend/internal/model/user.go`: add `LanguagePreference *string` and `ThemePreference *string` (nullable) to the `User` struct. Verify by `cd backend && go build ./...`.
- [x] 2.2 Update `backend/internal/repository/user.go`: SELECT and UPDATE statements include the two new columns; new method `UpdatePreferencesAndName(ctx, userID, fields)` for the self-service update path. Verify by `go vet ./...`.
- [x] 2.3 Add `backend/internal/repository/session.go` (or wherever sessions repo lives) method `DeleteOtherSessions(ctx, userID, currentSessionID) (int, error)` returning the deleted count. Verify by integration test in 5.4.

## 3. Backend service layer

- [x] 3.1 Add `service/auth.go` (or `service/user.go`) method `ChangeOwnPassword(ctx, viewerID, currentSessionID, currentPassword, newPassword) (revokedCount int, err error)` per design D-2. Validation: bcrypt compare current; reject `ErrInvalidCurrentPassword`; rune-count new password ≥ 8; bcrypt-hash; transactional update + session revocation; emit `user.password_change` audit. Sentinel errors: `ErrInvalidCurrentPassword`, `ErrPasswordTooShort` (existing). Verify by unit tests in 5.1.
- [x] 3.2 Add `service/user.go` method `UpdateOwnProfile(ctx, viewerID, fields)` per design D-3. Validation: trimmed name 1..100 runes; language enum; theme enum; reject unknown fields at the handler layer (DTO is enforced). Emits `user.update` audit with `fields` metadata. Verify by unit tests in 5.1.

## 4. Backend handler layer

- [x] 4.1 Add `backend/internal/handler/auth.go` route `POST /auth/change-password` per design D-2. DTO: `{ current_password, new_password }`. Map service errors to HTTP per the Canonical error codes spec: `ErrInvalidCurrentPassword` → 401 / `INVALID_CURRENT_PASSWORD`; `ErrPasswordTooShort` → 400 / `PASSWORD_TOO_SHORT`. On success return 204. Verify by `go build ./...`.
- [x] 4.2 Add `backend/internal/handler/user.go` route `PUT /users/me` per design D-3. DTO has explicit fields; unknown fields rejected via Go's `json.Decoder.DisallowUnknownFields()`. Returns the updated user. Verify by `go build ./...`.
- [x] 4.3 Update `backend/internal/handler/auth.go` `GET /auth/me` response to include `language_preference` and `theme_preference`. Verify by 5.2.
- [x] 4.4 Wire the routes in `backend/cmd/server/main.go` (or wherever the chi router is configured). Verify by route table grep.

## 5. Backend tests

- [x] 5.1 Service-layer unit tests:
  - `ChangeOwnPassword`: wrong current → `ErrInvalidCurrentPassword`; new < 8 runes → `ErrPasswordTooShort`; success path → password hash updated, sessions deleted (count returned), audit emitted with `revoked_session_count`.
  - `UpdateOwnProfile`: empty trimmed name → error; out-of-enum language → error; out-of-enum theme → error; partial update only touches supplied fields; audit emitted with the changed-field set.
  Verify by `go test ./internal/service/...`.
- [x] 5.2 Handler-layer test for `GET /auth/me`: response carries `language_preference` and `theme_preference`. Verify by `go test ./internal/handler/...`.
- [x] 5.3 Handler-layer test for `PUT /users/me`: DTO rejects unknown fields with HTTP 400 / `INVALID_REQUEST`; valid request returns updated user; audit row created. Verify by `go test ./internal/handler/...`.
- [x] 5.4 Integration test (with DB): `POST /auth/change-password` end-to-end — login twice (two sessions), call change-password from session A, verify session B is gone (queried by sessions table count by user_id), session A still present (call `GET /auth/me` and get 200), audit row carries `revoked_session_count = 1`. Verify by `go test -count=1 -tags=integration ./internal/handler/...`.

## 6. Frontend types + API

- [x] 6.1 Update `frontend/src/lib/types.ts`: extend `User` to include `language_preference: 'zh' | 'en' | null` and `theme_preference: 'light' | 'dark' | 'system' | null`. Verify by `pnpm tsc --noEmit`.
- [x] 6.2 Add Zod schemas + TanStack Query mutations for the new endpoints under `frontend/src/components/settings/` (or wherever appropriate): `changeOwnPasswordMutation`, `updateOwnProfileMutation`. Verify by `pnpm tsc --noEmit`.

## 7. Frontend theme + language plumbing

- [x] 7.1 Inline boot script in `frontend/index.html` that reads `localStorage["rota:theme"]` and applies `dark`/`light` class to `<html>` before React mounts (per design D-5). Verify by manual smoke (no FOUC).
- [x] 7.2 Add `frontend/src/components/theme-provider.tsx`: subscribes to `users.theme_preference` from `/auth/me`, applies `dark`/`light` class accordingly, listens to `prefers-color-scheme` when in `system` mode. Wraps the authenticated layout. Verify by component test in 9.4.
- [x] 7.3 Update `frontend/src/lib/i18n.ts` (or equivalent): on app boot read localStorage; on `/auth/me` resolve, override with server `language_preference` if non-null and different. Verify by manual smoke.

## 8. Frontend settings page + dropdown

- [x] 8.1 Add `frontend/src/routes/_authenticated/settings.tsx`: page-level component, three sections (profile / password / preferences) per design D-8. Verify by `pnpm tsc --noEmit`.
- [x] 8.2 Add `frontend/src/components/settings/profile-form.tsx`: React Hook Form + Zod (zod/v3) for name; save calls `updateOwnProfileMutation`. Verify by component test in 9.1.
- [x] 8.3 Add `frontend/src/components/settings/password-form.tsx`: form with current / new / confirm-new fields; client-side rule new == confirm; save calls `changeOwnPasswordMutation`; on success show toast "已退出其他设备" (revoke count > 0) and clear form. Verify by component test in 9.2.
- [x] 8.4 Add `frontend/src/components/settings/preferences-form.tsx`: language + theme radio groups; save calls `updateOwnProfileMutation` and updates `i18next` + theme provider locally. Verify by component test in 9.3.
- [x] 8.5 Update `frontend/src/components/app-sidebar.tsx`: avatar dropdown now has 切换主题 / 前往设置 / 登出 (per design D-7). Remove the existing "toggle language" entry. Verify by manual smoke + test in 9.5.

## 9. Frontend tests + i18n

- [x] 9.1 `profile-form.test.tsx`: name length validation (empty rejected, > 100 rejected, valid 50-char accepted).
- [x] 9.2 `password-form.test.tsx`: new < 8 chars rejected; mismatch new vs confirm rejected; success calls mutation.
- [x] 9.3 `preferences-form.test.tsx`: enum values render; save calls mutation; success updates local theme + locale.
- [x] 9.4 `theme-provider.test.tsx`: applies `dark` class for `theme_preference = 'dark'`; for `'system'`, follows `prefers-color-scheme` mock.
- [x] 9.5 Update `app-sidebar` tests: dropdown renders three items (toggle theme / settings / logout); clicking 前往设置 navigates to `/settings`.
- [x] 9.6 Add new i18n strings to `frontend/src/i18n/locales/{en,zh}.json`: settings page titles, section headers, field labels, error messages, save toasts, dropdown items. Verify by `pnpm lint && pnpm tsc --noEmit`.

## 10. Spec sync

- [x] 10.1 Confirm the change-folder spec delta at `openspec/changes/user-settings-page/specs/auth/spec.md` matches the implemented behavior (2 ADDED requirements + 4 MODIFIED). Do not edit `openspec/specs/auth/spec.md` directly — `/opsx:archive` syncs it.

## 11. Final gates

- [x] 11.1 `cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./... && govulncheck ./...`. All clean.
- [x] 11.2 `cd frontend && pnpm lint && pnpm test && pnpm build`. All clean.
- [x] 11.3 Manual smoke: log in → click avatar → 前往设置 → change name (verify reflected in sidebar avatar block on save) → change password (verify session B logged out by opening a second incognito window pre-change and refreshing post-change) → toggle theme via avatar dropdown (verify class change instant + persists across reload) → set language to English in settings (verify all UI strings translate).
- [x] 11.4 `openspec validate user-settings-page --strict`. Clean.
