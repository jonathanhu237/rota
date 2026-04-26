## MODIFIED Requirements

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

### Requirement: Emitted audit actions

The auth and user surfaces SHALL emit the following audit actions: `auth.login.success`; `auth.login.failure` with metadata `reason ∈ {invalid_credentials, user_pending, user_disabled}`; `auth.logout`; `auth.password_reset.request` with metadata including a server-only `user_found` flag; `auth.password.set` with metadata `purpose ∈ {invitation, password_reset}`; `user.create`; `user.update`; `user.invitation.resend`; `user.invitation.email_failed` with metadata `{ email, error }` whenever an invitation email's outbox row transitions to `failed` (i.e., the worker has exhausted its retry budget); `user.status.activate`; `user.status.disable`. Audit records SHALL NOT carry passwords, raw tokens, token hashes, or session ids.

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
