## MODIFIED Requirements

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

#### Scenario: Pending user sees USER_PENDING before bcrypt

- **GIVEN** a user with `status = 'pending'`
- **WHEN** `/auth/login` is called with that email
- **THEN** the response is 403 with code `USER_PENDING`
- **AND** `auth.login.failure` is emitted with `reason = user_pending`

#### Scenario: Wrong password returns INVALID_CREDENTIALS

- **GIVEN** an active user
- **WHEN** `/auth/login` is called with a wrong password
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

## ADDED Requirements

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
