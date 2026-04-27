## ADDED Requirements

### Requirement: Authenticated user changes own password

The system SHALL expose `POST /auth/change-password` requiring `RequireAuth`. The body SHALL be `{ current_password, new_password }`.

The handler SHALL, inside a single transaction:

1. Resolve the viewer's `user_id` from the session.
2. Read the current `password_hash` for that user with row-level locking.
3. Compare `current_password` against `password_hash` via `bcrypt.CompareHashAndPassword`. On mismatch, the request SHALL be rejected with HTTP 401 and error code `INVALID_CURRENT_PASSWORD`.
4. Validate the new password against the existing minimum (`utf8.RuneCountInString(new_password) >= 8`). On violation, the request SHALL be rejected with HTTP 400 and error code `PASSWORD_TOO_SHORT`.
5. Hash `new_password` with `bcrypt.DefaultCost` and write it via `UPDATE users SET password_hash = $1, version = version + 1 WHERE id = $userID`.
6. **Revoke every other session for this user** by `DELETE FROM sessions WHERE user_id = $userID AND id != $currentSessionID`. The current session SHALL be preserved so the caller is not immediately logged out.
7. Emit a `user.password_change` audit event with metadata `{ user_id, revoked_session_count }`. The audit metadata SHALL NOT include either password (current or new) or any password hash.

On success the handler SHALL return HTTP 204 No Content.

#### Scenario: Wrong current password is rejected

- **GIVEN** an authenticated user
- **WHEN** the user calls `POST /auth/change-password` with a `current_password` that does not match their stored hash
- **THEN** the response is HTTP 401 with error code `INVALID_CURRENT_PASSWORD`
- **AND** no row in `users` is modified
- **AND** no rows in `sessions` are deleted

#### Scenario: Too-short new password is rejected

- **WHEN** an authenticated user calls `POST /auth/change-password` with a `new_password` shorter than 8 runes
- **THEN** the response is HTTP 400 with error code `PASSWORD_TOO_SHORT`
- **AND** no row in `users` or `sessions` is modified

#### Scenario: Successful change preserves current session and revokes others

- **GIVEN** an authenticated user with two active sessions (the current call and one stale tab on another device)
- **WHEN** the user calls `POST /auth/change-password` with a correct `current_password` and a valid `new_password`
- **THEN** the response is HTTP 204
- **AND** `users.password_hash` is the bcrypt of `new_password`
- **AND** the current session row is still present
- **AND** the other session row is gone
- **AND** an audit event of action `user.password_change` is recorded with metadata containing `revoked_session_count = 1`

### Requirement: Authenticated user updates own profile

The system SHALL expose `PUT /users/me` requiring `RequireAuth`. The body SHALL accept any subset of `{ name, language_preference, theme_preference }`. Any unknown field SHALL be rejected with HTTP 400 and error code `INVALID_REQUEST`.

Validation:

- `name` (when present): trimmed of whitespace; the trimmed result SHALL satisfy `1 <= utf8.RuneCountInString(name) <= 100`. Violation SHALL return HTTP 400 / `INVALID_REQUEST`.
- `language_preference` (when present): SHALL be `null` or one of `'zh'`, `'en'`. Violation SHALL return HTTP 400 / `INVALID_REQUEST`.
- `theme_preference` (when present): SHALL be `null` or one of `'light'`, `'dark'`, `'system'`. Violation SHALL return HTTP 400 / `INVALID_REQUEST`.

On success the handler SHALL update only the fields supplied (absent fields are unchanged), bump `users.version`, return the updated user, and emit a `user.update` audit event with metadata `{ user_id, fields }` where `fields` is the array of changed-field names.

The endpoint SHALL NOT allow changing `email`, `is_admin`, `status`, `password_hash`, or `version`. These fields SHALL never be present in the accepted DTO.

#### Scenario: Self profile update changes name and theme

- **GIVEN** an authenticated user
- **WHEN** the user calls `PUT /users/me` with `{ "name": "Alice", "theme_preference": "dark" }`
- **THEN** the response is HTTP 200 with the updated user
- **AND** `users.name = 'Alice'` and `users.theme_preference = 'dark'`
- **AND** `users.version` is incremented by 1
- **AND** an audit event of action `user.update` is recorded with metadata `fields = ["name", "theme_preference"]`

#### Scenario: Unknown field is rejected

- **WHEN** an authenticated user calls `PUT /users/me` with body `{ "is_admin": true }`
- **THEN** the response is HTTP 400 with error code `INVALID_REQUEST`
- **AND** no row in `users` is modified

#### Scenario: Out-of-enum language preference is rejected

- **WHEN** an authenticated user calls `PUT /users/me` with `{ "language_preference": "fr" }`
- **THEN** the response is HTTP 400 with error code `INVALID_REQUEST`
- **AND** no row is modified

#### Scenario: Empty name is rejected

- **WHEN** an authenticated user calls `PUT /users/me` with `{ "name": "   " }`
- **THEN** the response is HTTP 400 with error code `INVALID_REQUEST` (trimmed length 0 violates the 1..100 bound)
- **AND** no row is modified

## MODIFIED Requirements

### Requirement: Users table schema

The system SHALL persist users in a `users` table with columns `id` (BIGSERIAL primary key), `email` (TEXT, unique, not null, normalized by trimming whitespace), `password_hash` (TEXT, nullable while the user is `pending`), `name` (TEXT, not null), `is_admin` (BOOLEAN, not null, default `FALSE`), `status` (TEXT, not null, default `active`, constrained by `CHECK IN ('active','disabled','pending')`), `version` (INTEGER, not null, default `1`, used as an optimistic concurrency token), `language_preference` (TEXT, nullable, constrained by `CHECK (language_preference IS NULL OR language_preference IN ('zh','en'))`), and `theme_preference` (TEXT, nullable, constrained by `CHECK (theme_preference IS NULL OR theme_preference IN ('light','dark','system'))`). Password hashes SHALL be produced with `bcrypt.DefaultCost`. Both preference columns SHALL default to NULL, meaning "the user has not expressed a preference"; the frontend SHALL fall back to its own defaults (zh / system) when the column is NULL.

#### Scenario: Pending user has no password hash

- **GIVEN** a user in status `pending`
- **THEN** the `password_hash` column is null
- **AND** bcrypt comparison never succeeds against that row

#### Scenario: Status check constraint rejects unknown values

- **WHEN** an insert or update sets `status` to a value outside `{active, disabled, pending}`
- **THEN** the database rejects the statement due to the CHECK constraint

#### Scenario: Language preference check constraint rejects unknown values

- **WHEN** an insert or update sets `language_preference` to a value outside `{zh, en, NULL}`
- **THEN** the database rejects the statement due to the CHECK constraint

#### Scenario: Theme preference check constraint rejects unknown values

- **WHEN** an insert or update sets `theme_preference` to a value outside `{light, dark, system, NULL}`
- **THEN** the database rejects the statement due to the CHECK constraint

### Requirement: Auth and user API routing

The server SHALL expose these HTTP routes: `POST /auth/login` (rate-limited per IP and per email); `POST /auth/logout` (no middleware); `GET /auth/me` (`RequireAuth`); `POST /auth/change-password` (`RequireAuth`); `POST /auth/password-reset-request` (rate-limited per IP); `GET /auth/setup-token` (no middleware); `POST /auth/setup-password` (no middleware); `GET /users` (`RequireAdmin`); `POST /users` (`RequireAdmin`); `GET /users/{id}` (`RequireAdmin`); `PUT /users/me` (`RequireAuth`); `PUT /users/{id}` (`RequireAdmin`, versioned); `POST /users/{id}/resend-invitation` (`RequireAdmin`); `PATCH /users/{id}/status` (`RequireAdmin`, versioned).

`GET /auth/me` SHALL return the viewer's user including `language_preference` and `theme_preference` so the frontend can hydrate UI on first paint.

#### Scenario: User update uses optimistic concurrency

- **GIVEN** a user row with `version = N`
- **WHEN** `PUT /users/{id}` is called with a stale `version`
- **THEN** the update is rejected due to version mismatch

#### Scenario: Admin list is paginated

- **WHEN** an admin calls `GET /users`
- **THEN** the response is a paginated list of users

#### Scenario: GET /auth/me carries preference fields

- **GIVEN** an authenticated user with `language_preference = 'en'` and `theme_preference = 'dark'`
- **WHEN** the user calls `GET /auth/me`
- **THEN** the response includes `language_preference = 'en'` and `theme_preference = 'dark'`

### Requirement: Canonical error codes

The system SHALL use the following error codes with the associated HTTP statuses: `INVALID_REQUEST` (400) for malformed JSON or missing required login fields; `INVALID_CREDENTIALS` (401) for unknown email or wrong password on `/auth/login`; `INVALID_CURRENT_PASSWORD` (401) for wrong current password on `/auth/change-password`; `UNAUTHORIZED` (401) for missing/invalid session, expired session, or deleted user; `USER_PENDING` (403) for login against a pending account; `USER_DISABLED` (403) for login against a disabled account; `FORBIDDEN` (403) for authenticated non-admin on admin-only endpoint; `INVALID_TOKEN` (400) for malformed/wrong-length setup token; `TOKEN_NOT_FOUND` (404) for unknown setup token hash; `TOKEN_EXPIRED` (410) for expired setup token; `TOKEN_USED` (410) for already-used setup token; `PASSWORD_TOO_SHORT` (400) for password under 8 characters on `/auth/setup-password` or `/auth/change-password`; `TOO_MANY_REQUESTS` (429) for rate-limit rejection; `INTERNAL_ERROR` (500) for unmapped internal error.

#### Scenario: Malformed JSON on login returns INVALID_REQUEST

- **WHEN** `/auth/login` receives a body that is not valid JSON
- **THEN** the response is 400 with code `INVALID_REQUEST`

#### Scenario: Unmapped internal error returns 500

- **WHEN** a handler encounters an error that is not mapped to a specific code
- **THEN** the response is 500 with code `INTERNAL_ERROR`

#### Scenario: Wrong current password on change-password returns INVALID_CURRENT_PASSWORD

- **WHEN** `/auth/change-password` receives a `current_password` that does not match the stored hash
- **THEN** the response is 401 with code `INVALID_CURRENT_PASSWORD` (distinct from `INVALID_CREDENTIALS`, which is reserved for login)

### Requirement: Emitted audit actions

The auth and user surfaces SHALL emit the following audit actions: `auth.login.success`; `auth.login.failure` with metadata `reason ∈ {invalid_credentials, user_pending, user_disabled}`; `auth.logout`; `auth.password_reset.request` with metadata including a server-only `user_found` flag; `auth.password.set` with metadata `purpose ∈ {invitation, password_reset}`; `user.create`; `user.update`; `user.password_change` with metadata `{ user_id, revoked_session_count }`; `user.invitation.resend`; `user.invitation.email_failed` with metadata `{ email, error }` whenever an invitation email's outbox row transitions to `failed` (i.e., the worker has exhausted its retry budget); `user.status.activate`; `user.status.disable`. Audit records SHALL NOT carry passwords, raw tokens, token hashes, or session ids.

The `user.update` action SHALL be emitted by both the admin `PUT /users/{id}` endpoint and the self-service `PUT /users/me` endpoint. The `user.password_change` action SHALL be emitted only by `POST /auth/change-password`.

#### Scenario: Login failure metadata carries reason

- **WHEN** `/auth/login` fails due to a disabled user
- **THEN** an `auth.login.failure` audit record is written with metadata `reason = user_disabled`

#### Scenario: Password set metadata carries purpose

- **WHEN** `/auth/setup-password` succeeds with a `password_reset` token
- **THEN** an `auth.password.set` audit record is written with metadata `purpose = password_reset`

#### Scenario: Self password change emits user.password_change with revocation count

- **GIVEN** an authenticated user with three active sessions
- **WHEN** the user calls `POST /auth/change-password` and it succeeds
- **THEN** an audit record of action `user.password_change` is written
- **AND** the metadata includes `revoked_session_count = 2` (current session preserved)
- **AND** no password hash or password value appears in the metadata

#### Scenario: Self profile update emits user.update

- **WHEN** an authenticated user calls `PUT /users/me` with `{ name: "Alice" }` and it succeeds
- **THEN** an audit record of action `user.update` is written with metadata `fields = ["name"]`
