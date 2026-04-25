## MODIFIED Requirements

### Requirement: Setup token table schema

The system SHALL persist setup tokens in a `user_setup_tokens` table with columns `id` (BIGSERIAL primary key), `user_id` (BIGINT, FK to `users(id)` with `ON DELETE CASCADE`), `token_hash` (TEXT, unique, the SHA-256 hex of the raw token), `purpose` (TEXT, constrained by `CHECK IN ('invitation','password_reset')`), `expires_at` (TIMESTAMPTZ, not null), `used_at` (TIMESTAMPTZ, null until spent; set once on successful use), and `created_at` (TIMESTAMPTZ, not null, default `NOW()`). Indexes SHALL include a unique index on `token_hash` and a secondary index on `(user_id, purpose)`. Raw tokens SHALL NOT be stored.

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

### Requirement: Setup password consumption

`POST /auth/setup-password` SHALL accept `{token, password}` and within a single transaction SHALL: (1) validate the password (`utf8.RuneCountInString(password) >= 8`); (2) resolve the token using the same four rejection branches as the preview (`INVALID_TOKEN`, `TOKEN_NOT_FOUND`, `TOKEN_USED`, `TOKEN_EXPIRED`); (3) bcrypt-hash the new password; (4) mark the token `used_at = now` using a conditional `UPDATE ... WHERE used_at IS NULL` whose `RowsAffected` is checked: `RowsAffected == 0` SHALL be translated to `ErrTokenUsed` and surfaced as `TOKEN_USED` (410) — guaranteeing strict single-use under concurrency; (5) invalidate every other unused token for that user — across both purposes — by setting `expires_at` to `now`; (6) update `password_hash` and set `status = 'active'` in the same `SET`. On success the handler SHALL return `204 No Content` and emit `auth.password.set` with metadata `purpose ∈ {invitation, password_reset}` reflecting the token consumed.

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

#### Scenario: Concurrent token consumption — second consumer rejected

- **GIVEN** an unused setup token `T` and two concurrent `POST /auth/setup-password` calls both carrying `T` and a valid password
- **WHEN** both calls reach `MarkUsed` and one transaction commits before the other
- **THEN** the first transaction's `MarkUsed` UPDATE returns `RowsAffected = 1` and the call returns 204
- **AND** the second transaction's `MarkUsed` UPDATE returns `RowsAffected = 0` because `used_at` is no longer NULL
- **AND** the second transaction translates `RowsAffected = 0` into `ErrTokenUsed`
- **AND** the second call returns HTTP 410 with code `TOKEN_USED`
- **AND** the user's `password_hash` reflects exactly the first transaction's password (not silently overwritten by the second)

### Requirement: Security headers delegated to proxy

The Go server SHALL NOT set security headers such as `Content-Security-Policy`, `Strict-Transport-Security`, `X-Content-Type-Options`, or `Referrer-Policy`. These headers SHALL be set by the Caddy reverse proxy at the edge of the deployment. The Caddy security-header block SHALL include `Referrer-Policy: no-referrer` to prevent any cross-origin sub-resource request from leaking URL query strings (which may include setup tokens) via the `Referer` header.

#### Scenario: Backend response omits CSP

- **WHEN** the backend returns a response
- **THEN** the response does not contain a `Content-Security-Policy` header set by the Go server

#### Scenario: Caddy sets Referrer-Policy: no-referrer

- **WHEN** any HTTP response is served by the deployment
- **THEN** the response carries the header `Referrer-Policy: no-referrer`

### Requirement: Emitted audit actions

The auth and user surfaces SHALL emit the following audit actions: `auth.login.success`; `auth.login.failure` with metadata `reason ∈ {invalid_credentials, user_pending, user_disabled}`; `auth.logout`; `auth.password_reset.request` with metadata including a server-only `user_found` flag; `auth.password.set` with metadata `purpose ∈ {invitation, password_reset}`; `user.create`; `user.update`; `user.invitation.resend`; `user.invitation.email_failed` with metadata `{ email, error }` whenever an invitation email send call (in `CreateUser` or `ResendInvitation`) returns an error; `user.status.activate`; `user.status.disable`. Audit records SHALL NOT carry passwords, raw tokens, token hashes, or session ids.

#### Scenario: Login failure metadata carries reason

- **WHEN** `/auth/login` fails due to a disabled user
- **THEN** an `auth.login.failure` audit record is written with metadata `reason = user_disabled`

#### Scenario: Password set metadata carries purpose

- **WHEN** `/auth/setup-password` succeeds with a `password_reset` token
- **THEN** an `auth.password.set` audit record is written with metadata `purpose = password_reset`

#### Scenario: Invitation email failure is audited

- **GIVEN** an admin call `POST /users` whose user-creation transaction commits successfully
- **WHEN** the post-commit `sendInvitation` returns an error (SMTP timeout, gateway rejection, etc.)
- **THEN** the admin's HTTP response is still 201 with the new user
- **AND** an audit event with action `user.invitation.email_failed` is recorded with `target_type = user`, `target_id = <new user id>`, and metadata `{ email, error }`
- **AND** a WARN-level log line is written carrying the same context
- **AND** the invitation token row remains valid (admin can call `ResendInvitation` later)

#### Scenario: Audit records exclude secrets

- **WHEN** any auth audit event is emitted
- **THEN** the event's metadata does not contain a password, raw token, token hash, or session id
