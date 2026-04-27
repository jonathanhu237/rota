## ADDED Requirements

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

## MODIFIED Requirements

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
