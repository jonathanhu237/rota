# Authentication

## Overview

Rota is an internal tool. There is no public signup: every account is created by an administrator, who triggers an emailed invitation link. The invitee follows the link to set a password, which activates the account. Thereafter the user authenticates with email and password against a Redis-backed session. Sessions are conveyed to the browser through an `HttpOnly` cookie. This spec covers identity, sessions, invitation, password reset, rate limiting, and the authorization middleware shared by the rest of the API.

Scheduling, publications, assignments, and shift changes are described in `@openspec/specs/scheduling.md`. Audit log mechanics and retention are described in `@openspec/specs/audit.md`; this spec only lists which actions it emits.

## Data model

### `users`

| Column          | Type          | Notes                                                      |
| --------------- | ------------- | ---------------------------------------------------------- |
| `id`            | `BIGSERIAL`   | Primary key.                                               |
| `email`         | `TEXT`        | Unique, not null. Normalized by trimming whitespace.       |
| `password_hash` | `TEXT`        | Nullable. Null while the user is `pending`.                |
| `name`          | `TEXT`        | Not null.                                                  |
| `is_admin`      | `BOOLEAN`     | Not null, default `FALSE`.                                 |
| `status`        | `TEXT`        | Not null, default `active`. `CHECK IN ('active','disabled','pending')`. |
| `version`       | `INTEGER`     | Not null, default `1`. Optimistic concurrency token.       |

Password hashes are produced with `bcrypt.DefaultCost`. The password policy is a minimum length of 8 characters; there are no composition requirements.

### `user_setup_tokens`

| Column       | Type          | Notes                                                        |
| ------------ | ------------- | ------------------------------------------------------------ |
| `id`         | `BIGSERIAL`   | Primary key.                                                 |
| `user_id`    | `BIGINT`      | FK to `users(id)` with `ON DELETE CASCADE`.                  |
| `token_hash` | `TEXT`        | Unique. SHA-256 hex of the raw token. Raw tokens are never stored. |
| `purpose`    | `TEXT`        | `CHECK IN ('invitation','password_reset')`.                  |
| `expires_at` | `TIMESTAMPTZ` | Not null.                                                    |
| `used_at`    | `TIMESTAMPTZ` | Null until the token is spent; set once on successful use.   |
| `created_at` | `TIMESTAMPTZ` | Not null, default `NOW()`.                                   |

Indexes: unique on `token_hash`, secondary on `(user_id, purpose)`.

### User status transitions

| From       | To         | Trigger                                           |
| ---------- | ---------- | ------------------------------------------------- |
| *(none)*   | `pending`  | Admin creates a user with no password.            |
| *(none)*   | `active`   | Bootstrap admin creation (see below).             |
| `pending`  | `active`   | Successful `SetupPassword` using an invitation token. |
| `active`   | `disabled` | Admin disables the user.                          |
| `disabled` | `active`   | Admin re-activates the user.                      |

## Bootstrap admin

On startup the server reads `BOOTSTRAP_ADMIN_EMAIL`, `BOOTSTRAP_ADMIN_PASSWORD`, and `BOOTSTRAP_ADMIN_NAME` from the environment and calls `EnsureBootstrapAdmin`:

- If any admin already exists (`COUNT(*)` of users where `is_admin = TRUE` is greater than zero), the function returns without changes. The operation is idempotent; restarts do not create duplicate admins.
- Otherwise all three env vars are required. A missing value or a password that fails `ValidatePassword` yields `ErrConfigInvalid` and the server exits.
- On success the admin is inserted directly as `status = 'active'` with `is_admin = TRUE` and a bcrypt-hashed password. The bootstrap path never emits an invitation token.

Operators SHOULD rotate `BOOTSTRAP_ADMIN_PASSWORD` after the first successful startup; after that initial insert the env var is unused.

## Session model

Sessions live in Redis under keys of the form `session:<session_id>`, where `session_id` is 32 bytes of `crypto/rand` rendered as lowercase hex. The value is the user's `id` as a decimal string.

- **TTL:** `SESSION_EXPIRES_HOURS` hours (336 in the shipped `.env.example`, i.e. 14 days). The TTL is applied on `SET` and refreshed on every `Authenticate` call via `EXPIRE`, so active users ride a sliding window.
- **Cookie:** name `session_id`, `Path=/`, `HttpOnly`, `SameSite=Lax`, `Secure` whenever the request was received over TLS (`r.TLS != nil`), `MaxAge` and `Expires` matching the Redis TTL.
- **Creation:** only `Login` creates sessions. There is no upgrade from anonymous to authenticated state — each login produces a freshly generated session id unrelated to any previous cookie, which removes any possibility of fixation through a pre-planted cookie value.
- **Invalidation:** `Logout` deletes the Redis key and clears the cookie. Admin-driven disable of a user triggers `DeleteUserSessions(userID)`, which scans the session keyspace and drops every session whose value equals that user id.

## Login flow

`POST /auth/login` accepts `{email, password}` and resolves in this order:

1. `GetByEmail` — if not found, return `INVALID_CREDENTIALS` (401). Emits `auth.login.failure` with reason `invalid_credentials`.
2. If `status = 'disabled'`, return `USER_DISABLED` (403). Emits `auth.login.failure` with reason `user_disabled`.
3. If `status = 'pending'`, return `USER_PENDING` (403). Emits `auth.login.failure` with reason `user_pending`.
4. `bcrypt.CompareHashAndPassword` — on mismatch, return `INVALID_CREDENTIALS` (401). Emits `auth.login.failure` with reason `invalid_credentials`.
5. Success: create a new session, set the cookie, and emit `auth.login.success`. The response body is `{user: {...}}`.

The order matters: the active-status checks run before the bcrypt comparison, so a pending or disabled user sees their specific status rather than a generic credentials error. This is acceptable because both branches require the requester to already know a valid email, and the distinction improves UX for legitimate users who are blocked. Unknown emails always resolve to `INVALID_CREDENTIALS`.

## Logout

`POST /auth/logout` reads the `session_id` cookie, deletes the Redis key when present, and always writes a cleared cookie (`MaxAge=-1`, empty value). The endpoint returns `204 No Content` regardless of whether a session was present. It emits `auth.logout`; the target user id is attached when the logger had an authenticated actor on the request context.

## Invitation flow

Admins create users through `POST /users`:

1. `CreateUser` normalizes email and name, validates the email shape via `mail.ParseAddress`, and checks for uniqueness. Duplicates return `EMAIL_ALREADY_EXISTS`.
2. Within a single transaction, the service inserts the user with `password_hash = NULL` and `status = 'pending'`, then issues an invitation setup token.
3. After the transaction commits, the emailer sends `BuildInvitationMessage` with a link of the form `<APP_BASE_URL>/setup-password?token=<raw-token>`. The raw token is never logged or persisted.
4. `user.create` is emitted to the audit log.

Invitation tokens are valid for `INVITATION_TOKEN_TTL` (default `72h`). Admins may regenerate one via `POST /users/{id}/resend-invitation` — that handler refuses for non-pending users (returns `USER_NOT_PENDING`), issues a fresh token (implicitly invalidating prior unused invitation tokens for the same user, see below), resends the email, and emits `user.invitation.resend`.

## Password reset flow

`POST /auth/password-reset-request` accepts `{email}` and always returns the same generic 200 response:

```
{"message": "If an account exists, a reset link has been sent"}
```

Internally:

1. The email is trimmed. An empty email is accepted and the request is a no-op.
2. `GetByEmail` is looked up. Not-found users cause the request to silently succeed.
3. If the user exists but `status != 'active'` (i.e. pending or disabled), no token is issued. Pending users must complete their invitation; disabled users cannot log in regardless.
4. Active users receive a freshly issued `password_reset` setup token valid for `PASSWORD_RESET_TOKEN_TTL` (default `1h`) and a templated email.
5. `auth.password_reset.request` is emitted. Metadata includes the email and a server-only `user_found` boolean so operators can distinguish probing from real requests without exposing that information to the client.

No information about account existence, status, or eligibility leaks to the caller.

## Setup-password endpoint (shared)

Both invitation and password-reset flows converge on the same token lifecycle. The raw token is a URL-safe base64 encoding of 32 random bytes. The database stores only its SHA-256 hex digest; a plain hash is adequate here because the token already carries 256 bits of entropy, so pre-image resistance is what matters and per-token salting would add no security.

### `GET /auth/setup-token?token=...`

Used by the frontend to render an account context screen (email, name, purpose) before asking for a new password. The handler decodes and hashes the token, looks it up, and returns `{email, name, purpose}`. It rejects tokens that are malformed (`INVALID_TOKEN`, 400), unknown (`TOKEN_NOT_FOUND`, 404), already used (`TOKEN_USED`, 410), or past `expires_at` (`TOKEN_EXPIRED`, 410).

### `POST /auth/setup-password`

Accepts `{token, password}`. Inside a single transaction the service:

1. Validates the password (`len >= 8`).
2. Resolves the token (same four rejection branches as the preview).
3. Bcrypt-hashes the new password.
4. Marks the token `used_at = now`.
5. Invalidates every other unused token for that user — across both purposes — by setting `expires_at` to `now`. This guarantees a setup token is single-use and that any stale invitation or reset link still in an inbox is neutralized the moment a password is successfully set.
6. Updates `password_hash` and sets `status = 'active'` in the same `SET` (so pending users become active atomically with their password landing).

On success the handler returns `204 No Content` and emits `auth.password.set` with metadata `purpose ∈ {invitation, password_reset}` reflecting the token that was consumed.

Token issuance itself is also defensive: `issueToken` first calls `InvalidateUnusedTokens(userID, purpose, now)` so that re-inviting or re-requesting a reset invalidates any still-valid token of the same purpose before creating the new one. Combined with step 5 above, only the most recently mailed token is ever usable.

## Rate limiting

The rate limiter is an in-process `golang.org/x/time/rate` limiter keyed per request. Keys that evaluate to the empty string skip rate limiting (this happens, for example, when the email key function cannot parse the JSON body). Exceeding a limit returns `429 TOO_MANY_REQUESTS` with a `Retry-After` header in seconds.

| Endpoint                           | Key                                    | Rate                           | Burst |
| ---------------------------------- | -------------------------------------- | ------------------------------ | ----- |
| `POST /auth/login`                 | client IP (from `X-Forwarded-For` first hop, else `RemoteAddr`) | 5 / minute (one token per 12 s) | 5     |
| `POST /auth/login`                 | lowercased email from the JSON body    | 10 / 15 minutes (one token per 90 s) | 10    |
| `POST /auth/password-reset-request`| client IP                              | 3 / hour (one token per 20 minutes) | 3     |

The login endpoint is chained with both limiters; either one triggering rejects the request. The per-email limiter throttles credential-stuffing attacks that rotate IPs, the per-IP limiter throttles attacks that rotate emails from a single origin. The password-reset limiter is deliberately the tightest: reset emails cost real money and attention, and 3/hour is well above any legitimate user's need while denying mass-mailing abuse.

The store keeps up to 4096 live keys in an LRU with 30-minute idle eviction, cleaned every 5 minutes.

## Access and authorization

The auth handler exposes two middleware wrappers used throughout the router in `cmd/server/main.go`:

- `RequireAuth` reads the `session_id` cookie, calls `Authenticate`, and on success: (a) refreshes the cookie expiry to match the new Redis TTL, (b) attaches the resolved `*model.User` to the request context, (c) attaches the actor id for audit. On `session.ErrSessionNotFound`, on an unknown user id, or on a user whose status has since flipped to `disabled`, it clears the cookie and returns `UNAUTHORIZED` (401).
- `RequireAdmin` wraps `RequireAuth` and additionally returns `FORBIDDEN` (403) when `user.IsAdmin` is false.

`Authenticate` does not revive sessions: if the Redis key is gone (expired, evicted, deleted by admin disable), the caller is logged out. The `disabled` re-check means even a still-live session cookie for a disabled user is rejected on the next request.

## API surface

| Method | Path                                | Middleware                                                | Purpose                                    |
| ------ | ----------------------------------- | --------------------------------------------------------- | ------------------------------------------ |
| POST   | `/auth/login`                       | rate-limit (IP) + rate-limit (email)                      | Log in; returns user, sets cookie.         |
| POST   | `/auth/logout`                      | —                                                         | Clear session and cookie.                  |
| GET    | `/auth/me`                          | `RequireAuth`                                             | Current user.                              |
| POST   | `/auth/password-reset-request`      | rate-limit (IP)                                           | Mail a reset link (always generic 200).    |
| GET    | `/auth/setup-token`                 | —                                                         | Preview invitation / reset context.        |
| POST   | `/auth/setup-password`              | —                                                         | Consume token, set password, activate.    |
| GET    | `/users`                            | `RequireAdmin`                                            | Paginated user list.                       |
| POST   | `/users`                            | `RequireAdmin`                                            | Create user; sends invitation email.       |
| GET    | `/users/{id}`                       | `RequireAdmin`                                            | Read user.                                 |
| PUT    | `/users/{id}`                       | `RequireAdmin`                                            | Update profile / admin flag (versioned).   |
| POST   | `/users/{id}/resend-invitation`     | `RequireAdmin`                                            | Reissue invitation to a pending user.      |
| PATCH  | `/users/{id}/status`                | `RequireAdmin`                                            | Activate / disable (versioned).            |

Security headers (`Content-Security-Policy`, `Strict-Transport-Security`, `X-Content-Type-Options`, `Referrer-Policy`, etc.) are not set by the Go server; they are the responsibility of the Caddy reverse proxy configured at the edge of the deployment.

## Error codes

| Code                   | HTTP | When                                                                  |
| ---------------------- | ---- | --------------------------------------------------------------------- |
| `INVALID_REQUEST`      | 400  | Malformed JSON body or missing required login fields.                 |
| `INVALID_CREDENTIALS`  | 401  | Unknown email or wrong password on `/auth/login`.                     |
| `UNAUTHORIZED`         | 401  | Missing / invalid session cookie, expired session, deleted user.      |
| `USER_PENDING`         | 403  | Login attempt against a `pending` account.                            |
| `USER_DISABLED`        | 403  | Login attempt against a `disabled` account.                           |
| `FORBIDDEN`            | 403  | Authenticated non-admin calling an admin-only endpoint.               |
| `INVALID_TOKEN`        | 400  | Setup token is malformed or wrong length.                             |
| `TOKEN_NOT_FOUND`      | 404  | Setup token hash has no row.                                          |
| `TOKEN_EXPIRED`        | 410  | Setup token past `expires_at`.                                        |
| `TOKEN_USED`           | 410  | Setup token already has `used_at`.                                    |
| `PASSWORD_TOO_SHORT`   | 400  | Supplied password under 8 characters on `/auth/setup-password`.       |
| `TOO_MANY_REQUESTS`    | 429  | Any of the three rate limiters rejected the request.                  |
| `INTERNAL_ERROR`       | 500  | Unmapped internal error.                                              |

## Audit events emitted

The following audit actions originate from the auth and user surfaces (see `@openspec/specs/audit.md` for schema and retention):

- `auth.login.success`
- `auth.login.failure` (metadata `reason ∈ {invalid_credentials, user_pending, user_disabled}`)
- `auth.logout`
- `auth.password_reset.request` (metadata includes a server-only `user_found` flag)
- `auth.password.set` (metadata `purpose ∈ {invitation, password_reset}`)
- `user.create`
- `user.update`
- `user.invitation.resend`
- `user.status.activate`
- `user.status.disable`

Audit records never carry passwords, raw tokens, token hashes, or session ids.

## Decisions log

- **`password_hash` nullable.** Invitations are a first-class state rather than a placeholder password. Making the column nullable and adding an explicit `pending` status avoids magic sentinel hashes, keeps bcrypt comparisons from ever succeeding against a pending user by accident, and lets the DB-level `CHECK` guard the state machine.
- **SHA-256 over bcrypt for setup tokens.** A setup token already carries 256 bits of entropy from `crypto/rand`, so resistance to offline brute force is not the threat model. We want constant-time database lookup by hash and we want raw tokens never stored — a plain SHA-256 digest satisfies both without the latency of bcrypt on every click of a mailed link.
- **Anti-enumeration on password reset.** The endpoint always returns the same generic message, regardless of whether the email exists or is eligible. Whether a reset was actually sent is recorded server-side in the audit log (`user_found`) so operators retain the ability to detect probing, but the external response surface exposes no signal.
- **Status check before bcrypt on login.** Pending and disabled branches precede password comparison so legitimate users receive actionable feedback. We accept the very small enumeration surface this creates — the attacker must already possess a valid email to observe it — in exchange for materially better UX on the two most common non-success login paths.
- **Rate-limit thresholds.** 5 logins/minute per IP covers real users who fat-finger a password a handful of times; 10 per 15 minutes per email caps credential-stuffing against a single account even when IPs rotate; 3 password resets per hour per IP is well above any human's need and hard-caps email-spam abuse. All three use token-bucket semantics, so the burst value equals the quota and bursts refill continuously.
- **Single-use tokens with cross-purpose invalidation.** A successful `SetupPassword` expires every other unused token for that user, not just the one being consumed. This eliminates the race where a stale invitation email and a fresh reset email coexist in a mailbox.
