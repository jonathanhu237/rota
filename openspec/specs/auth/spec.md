# Auth Capability

## Purpose

Rota is an internal tool with no public signup. Administrators create accounts, invitees activate them by setting a password, and thereafter users authenticate with email and password against a Postgres-backed session conveyed through an `HttpOnly` cookie. This capability covers identity, sessions, invitation, password reset, rate limiting, and the authorization middleware shared by the rest of the API. Scheduling is covered by the scheduling capability; audit log mechanics and retention are covered by the audit capability.
## Requirements
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

### Requirement: Password policy

The system SHALL require passwords of at least 8 code points (Unicode runes, not bytes), enforced at the backend on every code path that sets a `password_hash`. The frontend MAY enforce the same bound for UX, but backend enforcement is the security boundary. No composition requirements SHALL be enforced.

#### Scenario: Password under minimum length rejected

- **WHEN** `/auth/setup-password` is called with a password shorter than 8 code points
- **THEN** the request is rejected with `PASSWORD_TOO_SHORT` (400)

#### Scenario: Direct API call bypassing frontend is still rejected

- **GIVEN** a client (e.g., `curl`) that does not run the frontend zod validation
- **WHEN** the client posts `/auth/setup-password` with `password = "1"` (1 code point)
- **THEN** the response is HTTP 400 with code `PASSWORD_TOO_SHORT`
- **AND** no token is consumed and no user's `password_hash` is changed

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

### Requirement: Setup token table schema

The system SHALL persist setup tokens in a `user_setup_tokens` table with columns `id` (BIGSERIAL primary key), `user_id` (BIGINT, FK to `users(id)` with `ON DELETE CASCADE`), `token_hash` (TEXT, unique, the SHA-256 hex of the raw token), `purpose` (TEXT, constrained by `CHECK IN ('invitation','password_reset','email_change')`), `new_email` (TEXT, nullable), `expires_at` (TIMESTAMPTZ, not null), `used_at` (TIMESTAMPTZ, null until spent; set once on successful use), and `created_at` (TIMESTAMPTZ, not null, default `NOW()`). Indexes SHALL include a unique index on `token_hash` and a secondary index on `(user_id, purpose)`. Raw tokens SHALL NOT be stored.

A table-level CHECK SHALL enforce that `new_email IS NOT NULL` if and only if `purpose = 'email_change'`. For `purpose IN ('invitation','password_reset')`, `new_email` SHALL be NULL; for `purpose = 'email_change'`, `new_email` SHALL be the normalized target address.

Raw tokens SHALL NOT appear in any persistent log produced by the deployment, including the Caddy reverse-proxy access log. To enforce this, the access-log format SHALL omit URL query parameters (or actively redact `?token=` style values), so the substring `?token=` does not appear in `access.log`.

#### Scenario: Raw token is hashed before persistence

- **WHEN** a setup token is issued
- **THEN** only the SHA-256 hex digest of the raw token is stored in `token_hash`
- **AND** the raw token is never written to the database or logs

#### Scenario: Setup-token URL query is absent from the access log

- **GIVEN** the deployment's running Caddy access-log file
- **WHEN** a user clicks an invitation link `GET /api/auth/setup-token?token=<raw>` and `<raw>` is forwarded to the backend
- **THEN** the access-log entry for that request records the request path (e.g., `/api/auth/setup-token`)
- **AND** the access-log entry does NOT contain the substring `?token=` or the raw token bytes

#### Scenario: email_change row requires new_email

- **WHEN** an insert or update sets `purpose = 'email_change'` with `new_email IS NULL`
- **THEN** the database CHECK rejects the row

#### Scenario: invitation / password_reset rows forbid new_email

- **WHEN** an insert or update sets `purpose = 'invitation'` or `purpose = 'password_reset'` with `new_email IS NOT NULL`
- **THEN** the database CHECK rejects the row

#### Scenario: Unknown purpose value is rejected

- **WHEN** an insert sets `purpose` to a value outside `{invitation, password_reset, email_change}`
- **THEN** the database CHECK rejects the row

### Requirement: User status transitions

The system SHALL allow only the following `status` transitions: admin user creation without password produces `pending`; bootstrap admin creation produces `active`; successful `SetupPassword` using an invitation token transitions `pending` to `active`; admin disable transitions `active` to `disabled`; admin re-activation transitions `disabled` to `active`.

#### Scenario: Successful invitation setup activates pending user

- **GIVEN** a user with status `pending` and a valid invitation token
- **WHEN** the user POSTs a valid password to `/auth/setup-password`
- **THEN** the user's `status` is updated to `active`
- **AND** `password_hash` is set in the same SQL statement

#### Scenario: Admin disable transitions active to disabled

- **GIVEN** an `active` user
- **WHEN** an admin disables the user via `PATCH /users/{id}/status`
- **THEN** the user's `status` becomes `disabled`

### Requirement: Idempotent bootstrap admin

On startup the server SHALL invoke `EnsureBootstrapAdmin`, which MUST return without changes when any admin already exists (count of users with `is_admin = TRUE` is greater than zero). When no admin exists, the function SHALL require `BOOTSTRAP_ADMIN_EMAIL`, `BOOTSTRAP_ADMIN_PASSWORD`, and `BOOTSTRAP_ADMIN_NAME` environment variables; any missing value or a password that fails `ValidatePassword` SHALL yield `ErrConfigInvalid` and the server SHALL exit. On success the admin SHALL be inserted directly with `status = 'active'`, `is_admin = TRUE`, and a bcrypt-hashed password. The bootstrap path SHALL NOT emit an invitation token.

#### Scenario: Bootstrap is a no-op when admin already exists

- **GIVEN** at least one user with `is_admin = TRUE` already in the database
- **WHEN** the server starts and runs `EnsureBootstrapAdmin`
- **THEN** no new user is inserted and no error is raised

#### Scenario: Missing bootstrap env var aborts startup

- **GIVEN** no admin exists in the database
- **AND** `BOOTSTRAP_ADMIN_PASSWORD` is unset
- **WHEN** the server starts
- **THEN** `EnsureBootstrapAdmin` returns `ErrConfigInvalid`
- **AND** the server exits

#### Scenario: First-time bootstrap inserts active admin

- **GIVEN** no admin exists and all three bootstrap env vars are valid
- **WHEN** the server starts
- **THEN** a single user is inserted with `is_admin = TRUE`, `status = 'active'`, and a bcrypt password hash
- **AND** no invitation token is issued

### Requirement: Session storage and cookie

Sessions SHALL be stored in a Postgres `sessions` table with columns `id TEXT PRIMARY KEY` (the 32-byte `crypto/rand` random rendered as lowercase hex), `user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE`, `expires_at TIMESTAMPTZ NOT NULL`, and `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`. The table SHALL have indexes on `user_id` (to support `DeleteUserSessions(userID)`) and on `expires_at` (to support periodic cleanup of expired rows).

`expires_at` SHALL be set to `NOW() + SESSION_EXPIRES_HOURS` on row insert (login) and SHALL be refreshed to `NOW() + SESSION_EXPIRES_HOURS` on every `Authenticate` call (sliding-window semantics). The session cookie SHALL be named `session_id` with `Path=/`, `HttpOnly`, `SameSite=Lax`, and `Secure` whenever the request was received over TLS (`r.TLS != nil`). The cookie `MaxAge` and `Expires` SHALL match the remaining seconds until `expires_at`.

#### Scenario: Session TTL refreshed on authenticated request

- **GIVEN** an authenticated request carrying a valid `session_id` cookie
- **WHEN** the middleware calls `Authenticate`
- **THEN** the row's `expires_at` is set to `NOW() + SESSION_EXPIRES_HOURS`
- **AND** the response rewrites the cookie with a matching `MaxAge` / `Expires`

#### Scenario: Secure flag tracks request TLS

- **WHEN** a session cookie is emitted in response to a request with `r.TLS != nil`
- **THEN** the cookie has the `Secure` attribute set
- **WHEN** the request was not over TLS
- **THEN** the cookie is emitted without `Secure`

### Requirement: Session creation only on login

The system SHALL create sessions exclusively on successful `Login`. Each login SHALL produce a freshly generated `session_id` unrelated to any previous cookie. The system SHALL NOT upgrade an anonymous cookie to an authenticated session.

#### Scenario: Login replaces any pre-existing session id

- **GIVEN** a client sends `POST /auth/login` while presenting a pre-set `session_id` cookie
- **WHEN** the login succeeds
- **THEN** the response sets a freshly generated `session_id` unrelated to the request cookie

### Requirement: Session invalidation

`Logout` SHALL `DELETE FROM sessions WHERE id = $1` and clear the cookie. Admin disable of a user SHALL trigger `DeleteUserSessions(userID)`, which executes `DELETE FROM sessions WHERE user_id = $1` and removes every active session for that user. Successful `SetupPassword` whose token `purpose = 'password_reset'` SHALL likewise trigger `DeleteUserSessions(userID)` after the password update commits, so any session active at the moment of reset is terminated; `SetupPassword` for `purpose = 'invitation'` SHALL NOT, because pending users have no live sessions to invalidate.

#### Scenario: Admin disable terminates live sessions

- **GIVEN** a user with multiple `sessions` rows
- **WHEN** an admin disables the user
- **THEN** `DeleteUserSessions(userID)` deletes every `sessions` row whose `user_id` equals that user's id

#### Scenario: Password reset terminates live sessions

- **GIVEN** an `active` user with one or more `sessions` rows
- **WHEN** the user successfully calls `SetupPassword` using a `password_reset` token
- **THEN** the password change commits
- **AND** `DeleteUserSessions(userID)` deletes every `sessions` row whose `user_id` equals that user's id
- **AND** `auth.password.set` is emitted with metadata `purpose = "password_reset"`

#### Scenario: Invitation activation does not call DeleteUserSessions

- **GIVEN** a `pending` user with no live sessions
- **WHEN** the user successfully calls `SetupPassword` using an `invitation` token
- **THEN** the password change commits
- **AND** `DeleteUserSessions` is NOT called

#### Scenario: Authenticate does not revive an expired session

- **GIVEN** a browser still holds a `session_id` cookie whose `sessions` row has been deleted or whose `expires_at` is now in the past
- **WHEN** the next authenticated request is made
- **THEN** `Authenticate` returns an error
- **AND** the middleware clears the cookie and responds `UNAUTHORIZED` (401)

### Requirement: Periodic cleanup of expired sessions

The backend process SHALL run a background goroutine that periodically deletes long-expired session rows. The goroutine SHALL execute `DELETE FROM sessions WHERE expires_at < NOW() - INTERVAL '1 day'` every six hours. Errors from the cleanup SHALL be logged but SHALL NOT cause the goroutine to exit; the goroutine continues until the process shuts down. The cleanup is best-effort hygiene: lazy filtering on read (the `Authenticate` queries always filter `expires_at > NOW()`) is the correctness guarantee. No background cron, no external scheduler.

#### Scenario: Sweep removes long-expired rows

- **GIVEN** a `sessions` row whose `expires_at` is more than one day in the past
- **WHEN** the periodic sweep runs
- **THEN** the row is deleted

#### Scenario: Sweep does not remove still-valid sessions

- **GIVEN** a `sessions` row whose `expires_at` is in the future
- **WHEN** the periodic sweep runs
- **THEN** the row remains

#### Scenario: Sweep error does not exit the goroutine

- **WHEN** the sweep `DELETE` returns an error (e.g., transient DB issue)
- **THEN** the error is logged
- **AND** the goroutine continues, attempting the next sweep on the next tick

### Requirement: Login resolution order

`POST /auth/login` SHALL accept `{email, password}` and resolve in this order: (1) `GetByEmail` — return `INVALID_CREDENTIALS` (401) if not found; (2) return `USER_DISABLED` (403) if `status = 'disabled'`; (3) return `USER_PENDING` (403) if `status = 'pending'`; (4) `bcrypt.CompareHashAndPassword` — return `INVALID_CREDENTIALS` (401) on mismatch; (5) on success, insert a new `sessions` row, set the cookie, and return `{user: {...}}`. Unknown emails SHALL always resolve to `INVALID_CREDENTIALS`. Each failure branch SHALL emit `auth.login.failure` with the corresponding `reason` (`invalid_credentials`, `user_disabled`, `user_pending`); success SHALL emit `auth.login.success`.

#### Scenario: Unknown email returns generic INVALID_CREDENTIALS

- **WHEN** `/auth/login` is called with an email that is not in `users`
- **THEN** the response is 401 with code `INVALID_CREDENTIALS`
- **AND** `auth.login.failure` is emitted with `reason = invalid_credentials`

#### Scenario: Disabled user sees USER_DISABLED before bcrypt

- **GIVEN** a user with `status = 'disabled'`
- **WHEN** `/auth/login` is called with that email (regardless of password correctness)
- **THEN** the response is 403 with code `USER_DISABLED`
- **AND** `auth.login.failure` is emitted with `reason = user_disabled`
- **AND** `bcrypt.CompareHashAndPassword` is not invoked

#### Scenario: Pending user sees USER_PENDING before bcrypt

- **GIVEN** a user with `status = 'pending'`
- **WHEN** `/auth/login` is called with that email
- **THEN** the response is 403 with code `USER_PENDING`
- **AND** `auth.login.failure` is emitted with `reason = user_pending`

#### Scenario: Wrong password for active user

- **GIVEN** an active user
- **WHEN** `/auth/login` is called with the correct email but wrong password
- **THEN** the response is 401 with code `INVALID_CREDENTIALS`
- **AND** `auth.login.failure` is emitted with `reason = invalid_credentials`

#### Scenario: Successful login

- **GIVEN** an active user with a matching password
- **WHEN** `/auth/login` is called with correct credentials
- **THEN** a new row is inserted into `sessions`
- **AND** the `session_id` cookie is set
- **AND** the response body is `{user: {...}}`
- **AND** `auth.login.success` is emitted

### Requirement: Idempotent logout

`POST /auth/logout` SHALL read the `session_id` cookie, execute `DELETE FROM sessions WHERE id = $1` when present, and always write a cleared cookie (`MaxAge=-1`, empty value). The endpoint SHALL return `204 No Content` regardless of whether a session was present. It SHALL emit `auth.logout`, attaching the target user id when the logger had an authenticated actor on the request context.

#### Scenario: Logout without a session still succeeds

- **WHEN** `/auth/logout` is called with no `session_id` cookie
- **THEN** the response is `204 No Content`
- **AND** a cleared cookie is written

#### Scenario: Logout with a live session deletes the row

- **GIVEN** a `sessions` row whose `id` matches the request cookie
- **WHEN** `/auth/logout` is called
- **THEN** the row is deleted
- **AND** the response is `204 No Content`
- **AND** `auth.logout` is emitted

### Requirement: Admin-driven user creation

`POST /users` SHALL be an admin-only endpoint that (1) normalizes email and name, validates the email shape via `mail.ParseAddress`, and checks uniqueness — duplicates SHALL return `EMAIL_ALREADY_EXISTS`; (2) within a single transaction, inserts the user with `password_hash = NULL` and `status = 'pending'`, issues an invitation setup token, and enqueues an invitation email via `OutboxRepository.EnqueueTx` whose body contains a link of the form `<APP_BASE_URL>/setup-password?token=<raw-token>`; (3) emits `user.create` to the audit log. The raw token SHALL NOT be logged or persisted. The HTTP response is returned as soon as the transaction commits; SMTP delivery happens asynchronously via the outbox worker.

#### Scenario: Duplicate email rejected

- **WHEN** an admin posts to `/users` with an email that already exists (after trimming)
- **THEN** the response is an error with code `EMAIL_ALREADY_EXISTS`
- **AND** no user is inserted

#### Scenario: Invitation issued and enqueued on create

- **GIVEN** a valid, unique email
- **WHEN** an admin posts to `/users`
- **THEN** a user is inserted with `status = 'pending'` and `password_hash = NULL`
- **AND** an invitation setup token is issued in the same transaction
- **AND** an `email_outbox` row is enqueued in the same transaction containing the invitation email body with `<APP_BASE_URL>/setup-password?token=<raw-token>`
- **AND** `user.create` is emitted

### Requirement: Invitation token TTL

Invitation tokens SHALL be valid for `INVITATION_TOKEN_TTL` (default `72h`).

#### Scenario: Token past TTL rejected

- **GIVEN** an invitation token whose `expires_at` is in the past
- **WHEN** the token is presented to `/auth/setup-token` or `/auth/setup-password`
- **THEN** the handler responds with `TOKEN_EXPIRED` (410)

### Requirement: Resend invitation

`POST /users/{id}/resend-invitation` SHALL be admin-only. It SHALL refuse for non-pending users with `USER_NOT_PENDING`. It SHALL issue a fresh invitation token (implicitly invalidating any prior unused invitation tokens for the same user via `InvalidateUnusedTokens`), enqueue the invitation email via `OutboxRepository.EnqueueTx` in the same transaction, and emit `user.invitation.resend`.

#### Scenario: Resend refused for non-pending user

- **GIVEN** a user with `status != 'pending'`
- **WHEN** an admin calls `POST /users/{id}/resend-invitation`
- **THEN** the response is an error with code `USER_NOT_PENDING`

#### Scenario: Resend invalidates prior invitation

- **GIVEN** a pending user with an existing unused invitation token T1
- **WHEN** an admin calls `POST /users/{id}/resend-invitation`
- **THEN** T1's `expires_at` is set to `now` (invalidated)
- **AND** a new invitation token T2 is issued
- **AND** an `email_outbox` row is enqueued in the same transaction with the resent invitation body
- **AND** `user.invitation.resend` is emitted

### Requirement: Anti-enumeration password reset request

`POST /auth/password-reset-request` SHALL accept `{email}` and SHALL always return the same generic 200 response body `{"message": "If an account exists, a reset link has been sent"}`, regardless of whether the email exists or is eligible. The email SHALL be trimmed; an empty email SHALL be accepted as a no-op. A reset token SHALL be issued only when the user exists and `status = 'active'`; for not-found users, pending users, and disabled users, no token SHALL be issued. Active users SHALL receive a freshly issued `password_reset` setup token valid for `PASSWORD_RESET_TOKEN_TTL` (default `1h`) and an outbox-enqueued reset email; the email is delivered asynchronously by the outbox worker. The handler SHALL emit `auth.password_reset.request` with metadata including the email and a server-only `user_found` boolean.

#### Scenario: Unknown email returns generic 200

- **WHEN** `/auth/password-reset-request` is called with an email not present in `users`
- **THEN** the response is 200 with body `{"message": "If an account exists, a reset link has been sent"}`
- **AND** no token is issued
- **AND** no `email_outbox` row is enqueued
- **AND** `auth.password_reset.request` is emitted with `user_found = false`

#### Scenario: Pending or disabled user returns generic 200 without token

- **GIVEN** a user with `status` in `{pending, disabled}`
- **WHEN** `/auth/password-reset-request` is called with that email
- **THEN** the response is the same generic 200 body
- **AND** no `password_reset` token is issued
- **AND** no `email_outbox` row is enqueued

#### Scenario: Active user has reset email enqueued

- **GIVEN** a user with `status = 'active'`
- **WHEN** `/auth/password-reset-request` is called with that email
- **THEN** a new `password_reset` token is issued with TTL `PASSWORD_RESET_TOKEN_TTL`
- **AND** an `email_outbox` row is enqueued in the same transaction with the reset email body
- **AND** the response is the same generic 200 body
- **AND** `auth.password_reset.request` is emitted with `user_found = true`

### Requirement: Setup token shape

The raw setup token SHALL be a URL-safe base64 encoding of 32 random bytes generated by `crypto/rand`. The database SHALL store only the SHA-256 hex digest of the raw token.

#### Scenario: Token hash uniqueness enforced

- **WHEN** a new setup token is inserted with a `token_hash` that already exists
- **THEN** the unique index on `token_hash` rejects the insert

### Requirement: Setup token preview

`GET /auth/setup-token?token=...` SHALL decode and hash the supplied token, look it up, and on success return `{email, name, purpose}`. It SHALL reject malformed tokens with `INVALID_TOKEN` (400), unknown tokens with `TOKEN_NOT_FOUND` (404), already-used tokens with `TOKEN_USED` (410), and expired tokens with `TOKEN_EXPIRED` (410).

#### Scenario: Malformed token rejected

- **WHEN** `/auth/setup-token` is called with a token that is not valid URL-safe base64 or wrong length
- **THEN** the response is 400 with code `INVALID_TOKEN`

#### Scenario: Unknown token rejected

- **WHEN** `/auth/setup-token` is called with a well-formed token whose hash has no row
- **THEN** the response is 404 with code `TOKEN_NOT_FOUND`

#### Scenario: Already-used token rejected

- **GIVEN** a setup token whose `used_at` is not null
- **WHEN** `/auth/setup-token` is called with that token
- **THEN** the response is 410 with code `TOKEN_USED`

#### Scenario: Valid token returns account context

- **GIVEN** a valid, unused, unexpired setup token
- **WHEN** `/auth/setup-token` is called
- **THEN** the response body is `{email, name, purpose}`

### Requirement: Setup password consumption

`POST /auth/setup-password` SHALL accept `{token, password}` and within a single transaction SHALL: (1) validate the password (`utf8.RuneCountInString(password) >= 8`); (2) resolve the token using the same four rejection branches as the preview (`INVALID_TOKEN`, `TOKEN_NOT_FOUND`, `TOKEN_USED`, `TOKEN_EXPIRED`); the lookup SHALL filter by `purpose IN ('invitation', 'password_reset')` so that an `email_change` token cannot be redeemed via this endpoint; (3) bcrypt-hash the new password; (4) mark the token `used_at = now` using a conditional `UPDATE ... WHERE used_at IS NULL` whose `RowsAffected` is checked: `RowsAffected == 0` SHALL be translated to `ErrTokenUsed` and surfaced as `TOKEN_USED` (410) — guaranteeing strict single-use under concurrency; (5) invalidate every other unused token for that user — across all purposes — by setting `expires_at` to `now`; (6) update `password_hash` and set `status = 'active'` in the same `SET`. On success the handler SHALL return `204 No Content` and emit `auth.password.set` with metadata `purpose ∈ {invitation, password_reset}` reflecting the token consumed.

#### Scenario: Short password rejected

- **WHEN** `/auth/setup-password` is called with a password shorter than 8 characters
- **THEN** the response is 400 with code `PASSWORD_TOO_SHORT`
- **AND** no token is consumed and no user is modified

#### Scenario: Successful invitation setup activates user and invalidates siblings

- **GIVEN** a pending user with an unused invitation token T1 and a separate unused reset token T2
- **WHEN** `/auth/setup-password` is called with T1 and a valid password
- **THEN** the password is bcrypt-hashed and written to `password_hash`
- **AND** `status` is set to `active` in the same SQL statement
- **AND** T1's `used_at` is set to now
- **AND** T2's `expires_at` is set to now (invalidated)
- **AND** the response is `204 No Content`
- **AND** `auth.password.set` is emitted with `purpose = invitation`

#### Scenario: Successful password reset

- **GIVEN** an active user with an unused password_reset token
- **WHEN** `/auth/setup-password` is called with that token and a valid password
- **THEN** `password_hash` is updated
- **AND** the token's `used_at` is set
- **AND** the response is `204 No Content`
- **AND** `auth.password.set` is emitted with `purpose = password_reset`

#### Scenario: email_change token cannot be redeemed via setup-password

- **GIVEN** an unused `email_change` token T
- **WHEN** `/auth/setup-password` is called with T's raw token
- **THEN** the response is HTTP 404 with code `TOKEN_NOT_FOUND` (the lookup excludes `email_change` purpose)
- **AND** T is unchanged

#### Scenario: Concurrent token consumption — second consumer rejected

- **GIVEN** an unused setup token `T` and two concurrent `POST /auth/setup-password` calls both carrying `T` and a valid password
- **WHEN** both calls reach `MarkUsed` and one transaction commits before the other
- **THEN** the first transaction's `MarkUsed` UPDATE returns `RowsAffected = 1` and the call returns 204
- **AND** the second transaction's `MarkUsed` UPDATE returns `RowsAffected = 0` because `used_at` is no longer NULL
- **AND** the second transaction translates `RowsAffected = 0` into `ErrTokenUsed`
- **AND** the second call returns HTTP 410 with code `TOKEN_USED`
- **AND** the user's `password_hash` reflects exactly the first transaction's password (not silently overwritten by the second)

### Requirement: Email change confirmation

The system SHALL expose `POST /auth/confirm-email-change` with **no middleware** (anonymous, parallel to `POST /auth/setup-password`). The body SHALL be `{ token }` where `token` is the raw setup-token string from the confirmation email's URL.

The handler SHALL, inside a single transaction:

1. Validate the raw token shape (length / hex / etc.). On invalid shape the response SHALL be HTTP 400 with error code `INVALID_TOKEN`.
2. Hash the raw token; resolve the row in `user_setup_tokens` filtered by `token_hash` AND `purpose = 'email_change'`. If no row matches, the response SHALL be HTTP 404 with error code `TOKEN_NOT_FOUND`.
3. If `used_at IS NOT NULL`, the response SHALL be HTTP 410 with error code `TOKEN_USED`.
4. If `expires_at <= now`, the response SHALL be HTTP 410 with error code `TOKEN_EXPIRED`.
5. Mark the token consumed via `UPDATE user_setup_tokens SET used_at = now WHERE id = $tokenID AND used_at IS NULL`. `RowsAffected = 0` SHALL be translated to `ErrTokenUsed` and surfaced as HTTP 410 / `TOKEN_USED` (concurrency-safe single-use).
6. Re-check email uniqueness inside the transaction: `SELECT id FROM users WHERE email = $token.new_email AND id <> $token.user_id`. If any row exists, the response SHALL be HTTP 409 with error code `EMAIL_ALREADY_EXISTS`.
7. Update the user: `UPDATE users SET email = $token.new_email, version = version + 1 WHERE id = $token.user_id`.
8. Revoke ALL sessions for the user: `DELETE FROM sessions WHERE user_id = $token.user_id`. The link is anonymous, so any pre-confirmation session is forced to re-authenticate with the new email.
9. Invalidate every other unused token for this user across all purposes by setting `expires_at` to `now`.
10. Emit a `user.email_change.confirm` audit event with metadata `{ user_id, new_email_normalized, revoked_session_count }`. The audit metadata SHALL NOT include the raw token, the token hash, or any password.

On success the handler SHALL return HTTP 204 No Content.

#### Scenario: Successful confirmation swaps email and revokes sessions

- **GIVEN** a user with two active sessions and an unused `email_change` token T pointing at `new_email = 'alice2@example.com'`
- **WHEN** an unauthenticated client calls `POST /auth/confirm-email-change` with T's raw token
- **THEN** the response is HTTP 204
- **AND** `users.email = 'alice2@example.com'`
- **AND** `users.version` is incremented by 1
- **AND** the two session rows for that user are deleted
- **AND** T's `used_at` is set
- **AND** an audit event of action `user.email_change.confirm` is recorded with metadata `revoked_session_count = 2`

#### Scenario: Token rejection vocabulary mirrors setup-password

- **GIVEN** a setup-token attempt against `/auth/confirm-email-change`
- **WHEN** the token shape is invalid
- **THEN** the response is HTTP 400 with error code `INVALID_TOKEN`
- **WHEN** the token hash is unknown
- **THEN** the response is HTTP 404 with error code `TOKEN_NOT_FOUND`
- **WHEN** the token has already been used
- **THEN** the response is HTTP 410 with error code `TOKEN_USED`
- **WHEN** the token is past `expires_at`
- **THEN** the response is HTTP 410 with error code `TOKEN_EXPIRED`

#### Scenario: Email-uniqueness race at confirm time

- **GIVEN** an unused `email_change` token T pointing at `new_email = 'alice2@example.com'`
- **AND** between the request and the confirmation, another user is created with `email = 'alice2@example.com'`
- **WHEN** the original requester calls `POST /auth/confirm-email-change` with T's raw token
- **THEN** the response is HTTP 409 with error code `EMAIL_ALREADY_EXISTS`
- **AND** T's `used_at` remains the just-set value (the CAS marks it used before the uniqueness re-check; once consumed the token is dead either way)
- **AND** `users.email` for the original requester is unchanged

#### Scenario: Concurrent confirm — second consumer rejected

- **GIVEN** an unused `email_change` token T and two concurrent calls to `POST /auth/confirm-email-change` both carrying T's raw token
- **WHEN** both calls reach the CAS UPDATE and one transaction commits before the other
- **THEN** the first transaction's CAS returns `RowsAffected = 1` and the call returns 204
- **AND** the second transaction's CAS returns `RowsAffected = 0`
- **AND** the second call returns HTTP 410 with code `TOKEN_USED`
- **AND** `users.email` reflects exactly the first transaction's swap

### Requirement: Token issuance invalidates prior same-purpose tokens

`issueToken` SHALL call `InvalidateUnusedTokens(userID, purpose, now)` before creating a new token, ensuring any still-valid token of the same purpose for that user is invalidated before the new one is created.

#### Scenario: Re-requesting reset invalidates prior reset token

- **GIVEN** an active user with an unused password_reset token T1
- **WHEN** the user requests another reset and the server issues T2
- **THEN** T1's `expires_at` is set to `now` before T2 is inserted
- **AND** only T2 remains usable

### Requirement: Login rate limiting

`POST /auth/login` SHALL be chained with two in-process `golang.org/x/time/rate` limiters; either one triggering SHALL reject the request. The per-IP limiter SHALL key on the first hop of `X-Forwarded-For`, falling back to `RemoteAddr`, and enforce 5 requests per minute (one token per 12s) with burst 5. The per-email limiter SHALL key on the lowercased email extracted from the JSON body and enforce 10 requests per 15 minutes (one token per 90s) with burst 10. On rejection the handler SHALL return `429` with code `TOO_MANY_REQUESTS` and a `Retry-After` header in seconds. Empty keys SHALL skip rate limiting.

#### Scenario: Per-IP burst exhausted returns 429

- **GIVEN** five consecutive `/auth/login` requests from the same IP within the burst window
- **WHEN** a sixth request arrives before a token refills
- **THEN** the response is 429 with code `TOO_MANY_REQUESTS`
- **AND** a `Retry-After` header in seconds is present

#### Scenario: Per-email limiter throttles credential stuffing

- **GIVEN** ten `/auth/login` requests against the same email from differing IPs within 15 minutes
- **WHEN** an eleventh request arrives for that email before an email-token refills
- **THEN** the response is 429 with code `TOO_MANY_REQUESTS`

#### Scenario: Unparseable body skips email limiter

- **WHEN** `/auth/login` receives a body from which the email key function cannot extract an email (empty key)
- **THEN** the email limiter does not evaluate that request (only the IP limiter applies)

### Requirement: Password reset request rate limiting

`POST /auth/password-reset-request` SHALL be protected by a per-IP rate limiter enforcing 3 requests per hour (one token per 20 minutes) with burst 3. On rejection the handler SHALL return `429` with code `TOO_MANY_REQUESTS` and a `Retry-After` header.

#### Scenario: Fourth reset request from same IP rejected

- **GIVEN** three `/auth/password-reset-request` calls from the same IP within the burst
- **WHEN** a fourth arrives before a token refills
- **THEN** the response is 429 with code `TOO_MANY_REQUESTS`

### Requirement: Rate limiter store bounds

The rate limiter store SHALL keep up to 4096 live keys in an LRU with 30-minute idle eviction, cleaned every 5 minutes.

#### Scenario: Idle key evicted after 30 minutes

- **GIVEN** a rate-limit key that has seen no traffic for 30 minutes
- **WHEN** the periodic cleanup (every 5 minutes) runs
- **THEN** the key is evicted from the store

### Requirement: RequireAuth middleware

The `RequireAuth` middleware SHALL read the `session_id` cookie, call `Authenticate`, and on success: (a) refresh the cookie expiry to match the new `expires_at`; (b) attach the resolved `*model.User` to the request context; (c) attach the actor id for audit. On `repository.ErrSessionNotFound`, on an unknown user id, or on a user whose status has since flipped to `disabled`, it SHALL clear the cookie and return `UNAUTHORIZED` (401).

#### Scenario: Missing session cookie rejected

- **WHEN** an authenticated endpoint is called without a `session_id` cookie
- **THEN** the response is 401 with code `UNAUTHORIZED`

#### Scenario: Cookie for disabled user rejected

- **GIVEN** a still-live session cookie whose referenced user has been disabled
- **WHEN** the next request is made to a `RequireAuth`-protected endpoint
- **THEN** the cookie is cleared
- **AND** the response is 401 with code `UNAUTHORIZED`

#### Scenario: Authenticated request carries user on context

- **GIVEN** a valid session whose user is active
- **WHEN** the request reaches a `RequireAuth`-protected handler
- **THEN** `*model.User` is attached to the request context
- **AND** the actor id is attached for audit logging
- **AND** the cookie expiry is refreshed to match the new `expires_at`

### Requirement: RequireAdmin middleware

`RequireAdmin` SHALL wrap `RequireAuth` and SHALL additionally return `FORBIDDEN` (403) when the resolved user's `IsAdmin` is false.

#### Scenario: Non-admin user forbidden from admin endpoint

- **GIVEN** an authenticated user with `is_admin = FALSE`
- **WHEN** the user calls an admin-only endpoint (e.g. `GET /users`)
- **THEN** the response is 403 with code `FORBIDDEN`

#### Scenario: Admin passes through

- **GIVEN** an authenticated user with `is_admin = TRUE`
- **WHEN** the user calls an admin-only endpoint
- **THEN** the request proceeds to the handler

### Requirement: Auth and user API routing

The server SHALL expose these HTTP routes: `POST /auth/login` (rate-limited per IP and per email); `POST /auth/logout` (no middleware); `GET /auth/me` (`RequireAuth`); `POST /auth/change-password` (`RequireAuth`); `POST /auth/confirm-email-change` (no middleware); `POST /auth/password-reset-request` (rate-limited per IP); `GET /auth/setup-token` (no middleware); `POST /auth/setup-password` (no middleware); `GET /users` (`RequireAdmin`); `POST /users` (`RequireAdmin`); `GET /users/{id}` (`RequireAdmin`); `PUT /users/me` (`RequireAuth`); `POST /users/me/email-change-request` (`RequireAuth`); `PUT /users/{id}` (`RequireAdmin`, versioned); `POST /users/{id}/resend-invitation` (`RequireAdmin`); `PATCH /users/{id}/status` (`RequireAdmin`, versioned).

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

#### Scenario: Confirm-email-change is anonymous

- **WHEN** a client without a session cookie calls `POST /auth/confirm-email-change` with a body containing a valid raw token
- **THEN** the request is processed by the handler (the route has no middleware gate)

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

### Requirement: Authenticated user requests email change

The system SHALL expose `POST /users/me/email-change-request` requiring `RequireAuth`. The body SHALL be `{ new_email, current_password }`.

The handler SHALL, inside a single transaction:

1. Resolve the viewer's `user_id` from the session.
2. Read the viewer's row with row-level locking.
3. Compare `current_password` against `users.password_hash` via `bcrypt.CompareHashAndPassword`. On mismatch the request SHALL be rejected with HTTP 401 and error code `INVALID_CURRENT_PASSWORD`.
4. Trim and normalize `new_email`; validate the shape via `mail.ParseAddress`. On invalid shape the request SHALL be rejected with HTTP 400 and error code `INVALID_REQUEST`.
5. If the lower-cased `new_email` equals the lower-cased `users.email`, the request SHALL be rejected with HTTP 400 and error code `INVALID_REQUEST`.
6. Check uniqueness across `users` (any status); on collision the request SHALL be rejected with HTTP 409 and error code `EMAIL_ALREADY_EXISTS`.
7. Invalidate every prior unused `email_change` token for this user via `InvalidateUnusedTokens(user_id, 'email_change', now)`.
8. Issue a fresh raw token (≥ 32 random bytes), persist its SHA-256 hex digest, set `purpose = 'email_change'`, set `new_email = <normalized>`, set `expires_at = now + 24 hours`.
9. Enqueue, in the same transaction via `OutboxRepository.EnqueueTx`, two emails: a confirmation email to the new address whose body contains the link `<APP_BASE_URL>/auth/confirm-email-change?token=<raw>`, and a heads-up email to the current address informing them that an email-change request was issued. The heads-up email SHALL include a partially-masked rendering of `new_email` (e.g., `a***@example.com`) and SHALL NOT include the raw token or any link to act on the change.
10. Emit a `user.email_change.request` audit event with metadata `{ user_id, new_email_normalized }`. The audit metadata SHALL NOT include the raw token, the token hash, or any password.

On success the handler SHALL return HTTP 202 Accepted with an empty body.

#### Scenario: Wrong current password is rejected

- **GIVEN** an authenticated user
- **WHEN** the user calls `POST /users/me/email-change-request` with a `current_password` that does not match their stored hash
- **THEN** the response is HTTP 401 with error code `INVALID_CURRENT_PASSWORD`
- **AND** no token row is inserted
- **AND** no outbox row is enqueued
- **AND** no audit event is emitted

#### Scenario: New email matches current email is rejected

- **GIVEN** an authenticated user with `users.email = 'alice@example.com'`
- **WHEN** the user calls `POST /users/me/email-change-request` with `new_email = 'alice@example.com'` and a valid current password
- **THEN** the response is HTTP 400 with error code `INVALID_REQUEST`
- **AND** no token row is inserted

#### Scenario: New email already used by another user is rejected

- **GIVEN** an authenticated user requesting `new_email = 'bob@example.com'`
- **AND** another user already has `email = 'bob@example.com'` (regardless of status)
- **WHEN** the request handler runs
- **THEN** the response is HTTP 409 with error code `EMAIL_ALREADY_EXISTS`
- **AND** no token row is inserted

#### Scenario: Successful request enqueues confirmation and notice emails

- **GIVEN** an authenticated user with `users.email = 'alice@example.com'`
- **WHEN** the user calls `POST /users/me/email-change-request` with `new_email = 'alice2@example.com'` and a valid current password
- **THEN** the response is HTTP 202 Accepted with an empty body
- **AND** a `user_setup_tokens` row is inserted with `purpose = 'email_change'`, `new_email = 'alice2@example.com'`, `expires_at` ≈ now + 24h
- **AND** an outbox row is enqueued for `to = 'alice2@example.com'` whose body contains the confirmation URL with the raw token
- **AND** an outbox row is enqueued for `to = 'alice@example.com'` whose body informs the user that a change to a partially-masked address was requested and contains no actionable link
- **AND** an audit event of action `user.email_change.request` is recorded with metadata `{ user_id, new_email_normalized }`

#### Scenario: Re-request invalidates prior unused email_change token

- **GIVEN** an authenticated user with an unused `email_change` token T1
- **WHEN** the user calls `POST /users/me/email-change-request` again with valid inputs
- **THEN** T1's `expires_at` is set to `now` before the new token T2 is inserted
- **AND** only T2 is usable

### Requirement: Security headers delegated to proxy

The Go server SHALL NOT set security headers such as `Content-Security-Policy`, `Strict-Transport-Security`, `X-Content-Type-Options`, or `Referrer-Policy`. These headers SHALL be set by the Caddy reverse proxy at the edge of the deployment. The Caddy security-header block SHALL include `Referrer-Policy: no-referrer` to prevent any cross-origin sub-resource request from leaking URL query strings (which may include setup tokens) via the `Referer` header.

#### Scenario: Backend response omits CSP

- **WHEN** the backend returns a response
- **THEN** the response does not contain a `Content-Security-Policy` header set by the Go server

#### Scenario: Caddy sets Referrer-Policy: no-referrer

- **WHEN** any HTTP response is served by the deployment
- **THEN** the response carries the header `Referrer-Policy: no-referrer`

### Requirement: Canonical error codes

The system SHALL use the following error codes with the associated HTTP statuses: `INVALID_REQUEST` (400) for malformed JSON or missing required login fields; `INVALID_CREDENTIALS` (401) for unknown email or wrong password on `/auth/login`; `INVALID_CURRENT_PASSWORD` (401) for wrong current password on `/auth/change-password` or `/users/me/email-change-request`; `UNAUTHORIZED` (401) for missing/invalid session, expired session, or deleted user; `USER_PENDING` (403) for login against a pending account; `USER_DISABLED` (403) for login against a disabled account; `FORBIDDEN` (403) for authenticated non-admin on admin-only endpoint; `EMAIL_ALREADY_EXISTS` (409) for an email collision when creating a user via `POST /users` or requesting / confirming an email change; `INVALID_TOKEN` (400) for malformed/wrong-length setup token; `TOKEN_NOT_FOUND` (404) for unknown setup token hash; `TOKEN_EXPIRED` (410) for expired setup token; `TOKEN_USED` (410) for already-used setup token; `PASSWORD_TOO_SHORT` (400) for password under 8 characters on `/auth/setup-password` or `/auth/change-password`; `TOO_MANY_REQUESTS` (429) for rate-limit rejection; `INTERNAL_ERROR` (500) for unmapped internal error.

#### Scenario: Malformed JSON on login returns INVALID_REQUEST

- **WHEN** `/auth/login` receives a body that is not valid JSON
- **THEN** the response is 400 with code `INVALID_REQUEST`

#### Scenario: Unmapped internal error returns 500

- **WHEN** a handler encounters an error that is not mapped to a specific code
- **THEN** the response is 500 with code `INTERNAL_ERROR`

#### Scenario: Wrong current password on change-password returns INVALID_CURRENT_PASSWORD

- **WHEN** `/auth/change-password` receives a `current_password` that does not match the stored hash
- **THEN** the response is 401 with code `INVALID_CURRENT_PASSWORD` (distinct from `INVALID_CREDENTIALS`, which is reserved for login)

#### Scenario: Email collision on email-change request returns EMAIL_ALREADY_EXISTS

- **WHEN** `/users/me/email-change-request` is called with a `new_email` already used by another user
- **THEN** the response is 409 with code `EMAIL_ALREADY_EXISTS`

### Requirement: Emitted audit actions

The auth and user surfaces SHALL emit the following audit actions: `auth.login.success`; `auth.login.failure` with metadata `reason ∈ {invalid_credentials, user_pending, user_disabled}`; `auth.logout`; `auth.password_reset.request` with metadata including a server-only `user_found` flag; `auth.password.set` with metadata `purpose ∈ {invitation, password_reset}`; `user.create`; `user.update`; `user.password_change` with metadata `{ user_id, revoked_session_count }`; `user.email_change.request` with metadata `{ user_id, new_email_normalized }`; `user.email_change.confirm` with metadata `{ user_id, new_email_normalized, revoked_session_count }`; `user.invitation.resend`; `user.invitation.email_failed` with metadata `{ email, error }` whenever an invitation email's outbox row transitions to `failed` (i.e., the worker has exhausted its retry budget); `user.status.activate`; `user.status.disable`. Audit records SHALL NOT carry passwords, raw tokens, token hashes, or session ids.

The `user.update` action SHALL be emitted by both the admin `PUT /users/{id}` endpoint and the self-service `PUT /users/me` endpoint. The `user.password_change` action SHALL be emitted only by `POST /auth/change-password`. The `user.email_change.request` and `user.email_change.confirm` actions SHALL be emitted by their respective endpoints.

#### Scenario: Login failure metadata carries reason

- **WHEN** `/auth/login` fails due to a disabled user
- **THEN** an `auth.login.failure` audit record is written with metadata `reason = user_disabled`

#### Scenario: Password set metadata carries purpose

- **WHEN** `/auth/setup-password` succeeds with a `password_reset` token
- **THEN** an `auth.password.set` audit record is written with metadata `purpose = password_reset`

#### Scenario: Invitation email failure is audited after retry exhaustion

- **GIVEN** an admin call `POST /users` whose user-creation transaction commits successfully
- **AND** the outbox row enqueued for the invitation email
- **WHEN** the outbox worker repeatedly fails to send and ultimately marks the row `failed` after exhausting the retry budget
- **THEN** at the moment of `failed` transition, an audit event with action `user.invitation.email_failed` is recorded with `target_type = user`, `target_id = <new user id>`, and metadata `{ email, error }` (the most recent error message)
- **AND** the original admin's HTTP response was 201 with the new user (long since returned)
- **AND** the invitation token row remains valid (admin can call `ResendInvitation` later, which will create a fresh outbox row)
- **AND** during the in-flight retries (before the row reaches `failed`), no `user.invitation.email_failed` audit record is emitted

#### Scenario: Audit records exclude secrets

- **WHEN** any auth audit event is emitted
- **THEN** the event's metadata does not contain a password, raw token, token hash, or session id

#### Scenario: Self password change emits user.password_change with revocation count

- **GIVEN** an authenticated user with three active sessions
- **WHEN** the user calls `POST /auth/change-password` and it succeeds
- **THEN** an audit record of action `user.password_change` is written
- **AND** the metadata includes `revoked_session_count = 2` (current session preserved)
- **AND** no password hash or password value appears in the metadata

#### Scenario: Self profile update emits user.update

- **WHEN** an authenticated user calls `PUT /users/me` with `{ name: "Alice" }` and it succeeds
- **THEN** an audit record of action `user.update` is written with metadata `fields = ["name"]`

#### Scenario: Email change request emits audit event without leaking token

- **WHEN** `/users/me/email-change-request` succeeds
- **THEN** an audit record of action `user.email_change.request` is written
- **AND** the metadata contains `user_id` and `new_email_normalized`
- **AND** the metadata does NOT contain the raw token, the token hash, or the current password

#### Scenario: Email change confirmation emits audit event with revocation count

- **GIVEN** a user with two active sessions and an unused `email_change` token T
- **WHEN** `/auth/confirm-email-change` succeeds with T's raw token
- **THEN** an audit record of action `user.email_change.confirm` is written
- **AND** the metadata includes `user_id`, `new_email_normalized`, and `revoked_session_count = 2`
- **AND** the metadata does NOT contain the raw token or the token hash

### Requirement: Account emails render localized HTML and text bodies

Invitation, password-reset, email-change confirmation, and email-change notice emails SHALL be rendered from embedded external template files into both a text/plain body and an HTML body. Each account email subject SHALL be prefixed with `[Rota]`. Each actionable email SHALL include a primary CTA in HTML and the complete action URL in both HTML and text bodies. Email-change notice emails SHALL NOT include an action link or raw token.

The system SHALL support exactly the existing user preference languages for account email rendering: `en` and `zh`. Unsupported or missing language input SHALL resolve to `en`.

#### Scenario: Invitation email contains localized CTA and fallback URL

- **GIVEN** an admin creates or resends an invitation for a pending user
- **WHEN** the invitation email is enqueued
- **THEN** the outbox row has `kind = 'invitation'`
- **AND** the subject starts with `[Rota]`
- **AND** the text body contains the setup-password URL
- **AND** the HTML body contains a CTA for setting the password and the complete setup-password URL

#### Scenario: Password reset email contains localized CTA and fallback URL

- **GIVEN** an active user requests a password reset
- **WHEN** the password reset email is enqueued
- **THEN** the outbox row has `kind = 'password_reset'`
- **AND** the subject starts with `[Rota]`
- **AND** the text body contains the setup-password URL
- **AND** the HTML body contains a CTA for resetting the password and the complete setup-password URL

#### Scenario: Email-change confirmation contains localized CTA and fallback URL

- **GIVEN** an authenticated user requests an email change
- **WHEN** the confirmation email to the new address is enqueued
- **THEN** the outbox row has `kind = 'email_change_confirm'`
- **AND** the subject starts with `[Rota]`
- **AND** the text body contains the email-change confirmation URL
- **AND** the HTML body contains a CTA for confirming the email change and the complete confirmation URL

#### Scenario: Email-change notice omits action link

- **GIVEN** an authenticated user requests an email change
- **WHEN** the notice email to the current address is enqueued
- **THEN** the outbox row has `kind = 'email_change_notice'`
- **AND** the subject starts with `[Rota]`
- **AND** the text and HTML bodies include a partially masked rendering of the requested new address
- **AND** neither body contains the raw email-change token
- **AND** neither body contains a confirmation CTA

#### Scenario: Chinese account email is fully localized

- **GIVEN** an account email resolves to language `zh`
- **WHEN** the message is rendered
- **THEN** the subject after `[Rota]` is Chinese
- **AND** the CTA, duration text, security guidance, fallback-link label, and footer are Chinese
- **AND** the message does not mix English duration or action labels into the Chinese body

### Requirement: Account email language selection

The system SHALL choose account email language deterministically. Invitation email language SHALL resolve in this order: invited user's `language_preference`, triggering admin's `language_preference`, triggering admin request `Accept-Language`, then `en`. Password reset email language SHALL resolve in this order: recipient user's `language_preference`, request `Accept-Language`, then `en`. Email-change confirmation and notice email language SHALL resolve in this order: current user's `language_preference`, request `Accept-Language`, then `en`.

`Accept-Language` parsing SHALL only return `zh` or `en`. A header such as `zh-CN,zh;q=0.9,en;q=0.8` SHALL resolve to `zh`; a header such as `en-US,en;q=0.9` SHALL resolve to `en`; unsupported languages SHALL resolve to `en` unless an earlier persisted preference selected another language.

#### Scenario: Invitation uses admin language when invitee has none

- **GIVEN** a pending invitee with `language_preference IS NULL`
- **AND** the admin creating the invitation has `language_preference = 'zh'`
- **WHEN** the invitation email is enqueued
- **THEN** the rendered email language is `zh`

#### Scenario: Invitation uses Accept-Language after persisted preferences

- **GIVEN** a pending invitee with `language_preference IS NULL`
- **AND** the admin creating the invitation has `language_preference IS NULL`
- **AND** the request has `Accept-Language: zh-CN,zh;q=0.9,en;q=0.8`
- **WHEN** the invitation email is enqueued
- **THEN** the rendered email language is `zh`

#### Scenario: Password reset falls back to request language

- **GIVEN** an active user with `language_preference IS NULL`
- **AND** the password-reset request has `Accept-Language: zh-CN,zh;q=0.9`
- **WHEN** the password reset email is enqueued
- **THEN** the rendered email language is `zh`

#### Scenario: Persisted preference beats Accept-Language

- **GIVEN** an active user with `language_preference = 'en'`
- **AND** the triggering request has `Accept-Language: zh-CN,zh;q=0.9`
- **WHEN** an account email is enqueued for that user
- **THEN** the rendered email language is `en`

#### Scenario: Unsupported language falls back to English

- **GIVEN** an account email recipient with `language_preference IS NULL`
- **AND** the triggering request has `Accept-Language: fr-FR,fr;q=0.9`
- **WHEN** the account email is enqueued
- **THEN** the rendered email language is `en`

