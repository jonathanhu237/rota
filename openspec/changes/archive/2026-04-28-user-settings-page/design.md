## Context

Users currently have no in-app way to change anything about their own account: password changes go through the email-token reset flow, name/email/etc are admin-only, and there's no UI affordance for either language (the sidebar's "toggle language" only flips localStorage and is one-step, no scope) or theme (the app is light-mode only). This change adds a `/settings` page covering the four most-needed knobs (password, name, language, theme), persists language + theme as user-level columns so they follow the user across devices, and untangles the avatar dropdown.

The change is intentionally narrow — email change, avatar upload, and the broader navigation restructure are queued as separate changes.

## Goals / Non-Goals

**Goals:**

- Authenticated users can change their own password from `/settings` without the email reset flow. Other sessions are revoked on success; current session stays.
- Authenticated users can change their own display name from `/settings`.
- Language preference (zh / en) is a user-level field stored in `users.language_preference`. Stable across devices.
- Theme preference (light / dark / system) is a user-level field stored in `users.theme_preference`. Stable across devices. `system` follows `prefers-color-scheme` at runtime.
- The avatar dropdown ferries the user to settings, toggles theme as a one-tap shortcut, and signs out.

**Non-Goals:**

- Email change (separate change with verification flow).
- Avatar upload (separate change with blob storage).
- Per-device preferences.
- Quick-toggle buttons for language or theme in a global top-right zone.
- Sidebar restructure, breadcrumb, top tab bar — all queued as a separate larger change.
- Theme variants beyond dark / light (no high-contrast, no custom accent).
- Password strength meter or composition rules — the existing 8-rune minimum stays.

## Decisions

### D-1. Schema

Migration `00017_add_user_preferences.sql`:

```sql
-- +goose Up
ALTER TABLE users
    ADD COLUMN language_preference TEXT NULL
        CHECK (language_preference IS NULL OR language_preference IN ('zh', 'en')),
    ADD COLUMN theme_preference TEXT NULL
        CHECK (theme_preference IS NULL OR theme_preference IN ('light', 'dark', 'system'));

-- +goose Down
ALTER TABLE users
    DROP COLUMN theme_preference,
    DROP COLUMN language_preference;
```

Both columns nullable. NULL means "no preference set" — the frontend falls back to the i18n / theme defaults.

**Rejected — non-null with defaults:** would force a value on every user including the bootstrap admin and seed-created users. NULL semantics ("user hasn't expressed an opinion") is more honest.

**Rejected — separate `user_preferences` table:** at two columns it's overkill. If preferences grow to a dozen we revisit.

### D-2. Self-service password endpoint

`POST /auth/change-password`. RequireAuth.

Body:
```json
{ "current_password": "...", "new_password": "..." }
```

Service flow:

1. Resolve viewer's user_id from session.
2. Re-read `users.password_hash` for that user_id with `FOR UPDATE` inside a transaction.
3. `bcrypt.CompareHashAndPassword(password_hash, current_password)` — on mismatch, return `ErrInvalidCurrentPassword` mapped by handler to HTTP 401 / `INVALID_CURRENT_PASSWORD`.
4. Validate new password: `utf8.RuneCountInString(new_password) >= 8` (existing rule). Reject with HTTP 400 / `PASSWORD_TOO_SHORT`.
5. `bcrypt.GenerateFromPassword` the new password.
6. `UPDATE users SET password_hash = $1, version = version + 1 WHERE id = $userID`.
7. **Revoke other sessions:** `DELETE FROM sessions WHERE user_id = $userID AND id != $currentSessionID`. The current session stays so the user isn't immediately logged out.
8. Emit audit event `user.password_change` with metadata `{ revoked_session_count: <int> }`.
9. Commit. Return 204.

The transaction guarantees that two concurrent change-password requests can't both win (only one's `current_password` check succeeds against the post-update hash, the loser is rejected).

**Rejected — invalidate ALL sessions including current:** forces the user to log back in immediately after changing password. Bad UX.

**Rejected — leave other sessions alive:** standard security expectation is that "I changed my password" implies "any old logged-in tab on another device should stop working."

### D-3. Self-service profile endpoint

`PUT /users/me`. RequireAuth.

Body:
```json
{
  "name": "...",                     // optional, 1..100 trimmed
  "language_preference": "zh",       // optional, ∈ {'zh','en'}|null
  "theme_preference": "dark"         // optional, ∈ {'light','dark','system'}|null
}
```

Validation:

- `name`: trim → `utf8.RuneCountInString` between 1 and 100 inclusive. Reject with HTTP 400 / `INVALID_REQUEST`.
- `language_preference`: explicit `null` allowed (clears the preference). Strings outside the enum rejected with HTTP 400 / `INVALID_REQUEST`.
- `theme_preference`: same.
- Unknown body fields → HTTP 400 / `INVALID_REQUEST`. The handler uses an explicit DTO (no JSON pass-through) so unknown fields naturally fail.

`UPDATE users SET ... WHERE id = $viewerUserID`. The handler emits `user.update` audit event with the changed-field set in metadata.

The endpoint **does not** allow changing `email`, `is_admin`, `status`, `password_hash`, or `version` — those fields are never in the DTO.

**Rejected — extend `PUT /users/{id}` to accept self-edit:** would mean threading a "is this self-edit or admin-edit" branch through the existing handler. Cleaner to keep `PUT /users/me` as a separate handler that delegates to a shared service method `UpdateProfile(viewerID, fields)`.

### D-4. `GET /auth/me` extension

Existing endpoint. Add to the response shape:

```json
{
  "user": {
    ...,
    "language_preference": "zh" | null,
    "theme_preference": "dark" | "light" | "system" | null
  }
}
```

The frontend uses this on app-load to hydrate the i18n locale + theme class. Cached in localStorage for instant first-paint on subsequent loads.

### D-5. Theme system on the frontend

```
┌─ index.html ────────────────────────────┐
│ <script>                                │
│   const cached = localStorage           │
│     .getItem("rota:theme") ?? "system"  │
│   const dark = cached === "dark" || (   │
│     cached === "system" &&              │
│     matchMedia("(prefers-color-scheme:  │
│                  dark)").matches)       │
│   document.documentElement.classList    │
│     .toggle("dark", dark)               │
│ </script>                               │
└─────────────────────────────────────────┘
```

Inline script in `index.html` runs before React mounts → no flash of unstyled content. Cached value is "last value applied"; the React `ThemeProvider` reads `users.theme_preference` from `/auth/me` once available and:

- Updates localStorage cache.
- Re-applies the `dark` class.
- For `system` mode, subscribes to the `(prefers-color-scheme: dark)` media query and toggles `dark` class on change.

Tailwind's `darkMode: 'class'` config (already standard) makes `dark:` variants drive the styles.

**Rejected — CSS-only theme via `:root` variables:** more flexible long-term but more refactoring of existing components. Tailwind `dark:` variants are sufficient and per-component opt-in.

**Rejected — server-rendered theme class:** we're not SSR; static HTML + the tiny boot script is enough.

### D-6. Language system on the frontend

i18next is already in place. Wire-up:

- On app boot: read `localStorage["rota:lang"]` (default `"zh"`); call `i18n.changeLanguage(...)`.
- After `/auth/me` resolves: if `users.language_preference !== null && users.language_preference !== currentLanguage`, call `i18n.changeLanguage(users.language_preference)` and update localStorage.
- On Settings save: `PUT /users/me { language_preference }`, then `i18n.changeLanguage` + localStorage update.

**Rejected — server-side `Accept-Language` content negotiation:** i18next on the client already handles per-string interpolation; server-side language switching adds complexity for no win.

### D-7. Avatar dropdown items

Current items (after the recent fix):
- 切换语言 → toggleLanguage
- (separator)
- 登出 → logout

New items:
- 切换主题 (Toggle theme) — toggles between `light` and `dark`. If the user is in `system` mode, the toggle moves them to the explicit opposite of whatever the current effective theme is (e.g., system-resolves-to-dark → click → `light`). The body of this handler calls `PUT /users/me { theme_preference: ... }` then updates the local theme state.
- 前往设置 (Go to settings) — `<Link to="/settings">`.
- (separator)
- 登出 (Log out) — unchanged.

The "切换语言" entry is **removed** from the dropdown. Language now lives only in the settings page (per the proposal's argument that it changes infrequently).

**Rejected — keep "切换语言" as a quick action:** noise; settings page is the right surface.

### D-8. Settings page layout

Single page at `/settings`, three stacked sections (each is its own `<Card>`):

```
┌─────────────────────────────────────────┐
│  设置                                    │
│  管理你的账号和应用偏好。                 │
├─────────────────────────────────────────┤
│  个人资料                                │
│    显示名称  [_________________]  保存   │
├─────────────────────────────────────────┤
│  密码                                    │
│    当前密码  [_________________]         │
│    新密码    [_________________]         │
│    确认新密码 [_________________]         │
│                                  保存    │
├─────────────────────────────────────────┤
│  偏好                                    │
│    语言     ( ) 中文   ( ) English       │
│    主题     ( ) 跟随系统  ( ) 浅色  ( ) 深色 │
│                                  保存    │
└─────────────────────────────────────────┘
```

Three independent forms with their own React Hook Form instances and submit buttons. Saving one section doesn't affect the others. After a successful save the section's mutation invalidates the relevant TanStack Query cache (`/auth/me`).

**Rejected — single big form with one Save:** mixes high-friction (password change) with low-friction (theme picker). Per-section saves match the user's mental model.

### D-9. Audit + error code surface

New error code: `INVALID_CURRENT_PASSWORD` (HTTP 401), used only by `POST /auth/change-password`.

New audit action: `user.password_change`. Emitted on the success branch of `POST /auth/change-password`. Metadata: `{ user_id, revoked_session_count }`. The actor is the same user; standard `actor_user_id` is filled.

Existing `user.update` audit action is reused for `PUT /users/me`. Metadata reflects the changed-field set: `{ user_id, fields: ["name","language_preference"] }` (or whichever subset was touched).

### D-10. Tests

Backend:

- Unit: `users.UpdateProfile` validates field bounds, rejects unknown enum values, ignores untouched fields.
- Unit: `users.ChangePassword` rejects wrong current password, rejects too-short new password, succeeds and revokes other sessions.
- Integration (with DB): `POST /auth/change-password` end-to-end: cookie-authenticated request changes hash, current session works post-change, other sessions are gone.
- Integration: `PUT /users/me` end-to-end with the four fields.
- Integration: audit log carries the new event type.

Frontend:

- Unit: `<ProfileForm>` validates name length (Zod schema test).
- Unit: `<PasswordForm>` validates the "new == confirm" rule.
- Unit: `<PreferencesForm>` validates enum values.
- Component: theme provider applies `dark` class on mount with cached value; reapplies on `/auth/me` returning a different value.
- Component: avatar dropdown renders the three new items.
- Schema: zod schema for the new endpoint shapes.

## Risks / Trade-offs

- **Risk:** the inline boot script in `index.html` is global JS that runs before React. → Mitigation: single 8-line script with no dependencies, no fetch, just localStorage + matchMedia + classList. If it errors it does no harm; React's `ThemeProvider` reasserts the class anyway.
- **Risk:** users on shared computers might leave the theme set to dark, surprising the next person. → Acceptable: that's the same threat model as any user-level preference. Logout clears the session cookie but not localStorage.
- **Risk:** `PUT /users/me` and the existing `PUT /users/{id}` could drift in validation rules. → Mitigation: extract `validateUserUpdate(fields)` into a shared helper; both handlers call it.
- **Trade-off:** language stored both in `users.language_preference` AND localStorage — two sources of truth. → Acceptable: localStorage is a pure cache for first-paint speed; server is canonical and overrides on `/auth/me` resolve.

## Migration Plan

Single shipping unit:

1. Apply migration `00017_add_user_preferences.sql` via `make migrate-up`. Existing users get NULL preferences (interpreted as "no preference set").
2. Backend rebuilt + tested.
3. Frontend rebuilt + tested.
4. Manual smoke: log in, navigate to `/settings`, change name, change password, log in on a second tab pre-change — confirm second tab is logged out post-change. Toggle theme via dropdown, confirm class change. Set language via settings, refresh, confirm persisted.

Rollback = `make migrate-down 1` (drops the two columns) + revert the change. The frontend gracefully handles missing fields (treat as null).

## Open Questions

None.
