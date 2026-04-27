## Why

Right now the only way for an employee to change anything about their own account is to ask an admin (admin uses `PUT /users/{id}`) or the password-reset flow via email. There's no place for an authenticated user to change their password from inside the app, no place to set a language preference that persists across devices, and no theme preference at all (the UI is light-mode only). The avatar dropdown in the sidebar has a "toggle language" entry that flips the i18n locale in localStorage but doesn't actually persist anywhere, and clicking the rest of the chrome leads to a `MenuGroupRootContext` crash that we just fixed.

This change adds a self-service `/settings` page with the four most-needed knobs (password, name, language, theme), plumbs language + theme as user-level columns so they're stable across devices, and tidies up the avatar dropdown so admins and employees alike can navigate to settings + log out from one place.

Out of scope (deliberately) and queued as later changes: email change (needs a verification flow), avatar upload (needs blob storage), and the broader navigation restructure (sidebar consolidation, breadcrumb, per-feature top tab bar).

## What Changes

### Backend

- **Schema** — `users` table grows two nullable columns:
  - `language_preference TEXT NULL` with `CHECK (language_preference IN ('zh','en'))`. NULL means "follow request `Accept-Language` / app default".
  - `theme_preference TEXT NULL` with `CHECK (theme_preference IN ('light','dark','system'))`. NULL means "follow OS — i.e., `system`."
  - Migration: `00017_add_user_preferences.sql`. Backfill is left as NULL; existing users see the defaults.
- **New endpoint `POST /auth/change-password`** — `RequireAuth`. Body `{current_password, new_password}`. Verifies `current_password` against `users.password_hash`; rejects with HTTP 401 + `INVALID_CURRENT_PASSWORD` on mismatch. Validates new password per the existing 8-rune minimum. On success bcrypt-hashes the new password, updates `users.password_hash`, **revokes all OTHER sessions for that user** (current session preserved), and emits `user.password_change` audit event. Returns 204.
- **New endpoint `PUT /users/me`** — `RequireAuth`. Body accepts any subset of `{name, language_preference, theme_preference}`. Validation: `name` trimmed, 1..100 code points; `language_preference ∈ {'zh', 'en'} | null`; `theme_preference ∈ {'light', 'dark', 'system'} | null`. Updates only the supplied fields. Returns the updated user. Emits `user.update` audit event.
- **Extend `GET /auth/me`** — response now carries `language_preference` and `theme_preference` so the frontend can hydrate UI on first paint.
- **Audit** — new action `user.password_change`. Existing `user.update` action gets reused for self-service profile updates (audit row carries the actor + target user — they happen to be the same user for self-service).

### Frontend

- **New route `/settings`** under the authenticated layout. Page split into three sections:
  1. **Profile** — name input, save button.
  2. **Password** — current password + new password + confirm new password fields, save button.
  3. **Preferences** — language radio (中文 / English), theme radio (跟随系统 / 浅色 / 深色), save button.
- **Theme system** — a `ThemeProvider` component sets the `dark` / `light` class on `<html>` based on `users.theme_preference`. `system` mode listens to `prefers-color-scheme` and reapplies on change. Initial load uses localStorage cache for instant paint, server value overrides on `/auth/me` resolution. Tailwind's `dark:` variants drive the actual styles.
- **Language system** — i18next switches locale on user preference. Persisted to `users.language_preference` server-side; localStorage cache for fast initial paint mirrors theme handling.
- **Avatar dropdown rework** — replace the existing "toggle language" entry with three new items:
  - 切换主题 (toggles light↔dark; "system" stays unless the admin explicitly sets it via settings)
  - 前往设置 (navigates to `/settings`)
  - (existing) 登出 / Log out

### Capabilities

#### New Capabilities

None.

#### Modified Capabilities

- `auth`:
  - `Users table schema` — adds `language_preference` and `theme_preference` columns.
  - `Auth and user API routing` — adds `POST /auth/change-password` and `PUT /users/me`.
  - `Canonical error codes` — adds `INVALID_CURRENT_PASSWORD`.
  - `Emitted audit actions` — adds `user.password_change`.
  - One new requirement: *Authenticated user changes own password*.
  - One new requirement: *Authenticated user updates own profile*.

## Non-goals

- **Email change.** Login depends on email; changing it needs a verification flow (token to new email + click-to-confirm) plus outbox plumbing. Separate change.
- **Avatar upload.** No blob storage in the project today; adding it is a much larger surface (storage backend, image processing, CDN considerations). Initials chip stays for now.
- **Two-factor auth.** Already ruled out elsewhere.
- **Per-device preferences.** Both language and theme are user-level. Logging in on a second device shows the same locale + theme.
- **Sidebar restructure / top tab bar / breadcrumb.** Separate, scope-creepy navigation rework.
- **Theme palette tweaks beyond dark / light.** No high-contrast, no custom accent colors. The `dark:` variants Tailwind ships with are it.

## Impact

- **Backend code:**
  - `migrations/00017_add_user_preferences.sql` (new).
  - `backend/internal/model/user.go` — fields added.
  - `backend/internal/repository/user.go` — read/write the new columns; new method to revoke other sessions on password change.
  - `backend/internal/service/user.go` (or `auth.go`) — new methods for `ChangeOwnPassword` and `UpdateOwnProfile`.
  - `backend/internal/handler/auth.go` — new endpoints.
  - `backend/internal/handler/user.go` — new `PUT /users/me` handler.
  - Tests: unit + integration coverage for both new endpoints.
- **Frontend code:**
  - `frontend/src/routes/_authenticated/settings.tsx` (new).
  - `frontend/src/components/settings/profile-form.tsx`, `password-form.tsx`, `preferences-form.tsx` (new).
  - `frontend/src/components/theme-provider.tsx` (new) wrapped around the app at the route layout.
  - `frontend/src/components/app-sidebar.tsx` — dropdown items rewired.
  - `frontend/src/lib/types.ts` — extend `User` shape.
  - i18n strings under `frontend/src/i18n/locales/{en,zh}.json`.
- **Spec:** `auth` capability gets two new requirements + edits to four existing ones (users table, routing, error codes, audit actions).
- **No new third-party dependencies.** Tailwind already supports `dark:` variants.
- **No infra / config changes.**

## Risks / safeguards

- **Risk:** revoking other sessions on password change might surprise a user with a still-logged-in tab on another device. **Mitigation:** standard security expectation; documented in the success message ("已退出其他设备" / "Signed out other devices").
- **Risk:** theme = `system` race during initial paint causes flash-of-unstyled-content. **Mitigation:** localStorage cache primes the `<html>` class before React hydrates; server value reconciles on `/auth/me` resolve.
- **Risk:** `PUT /users/me` overlaps semantically with the admin's `PUT /users/{id}` and could drift apart. **Mitigation:** both endpoints share the same service-layer validation function; the handler-level difference is just "who's the target user."
- **Risk:** the schema migration is mid-cycle (the project already has 16 migrations) but the columns are nullable with no backfill, so it's near-zero risk.
