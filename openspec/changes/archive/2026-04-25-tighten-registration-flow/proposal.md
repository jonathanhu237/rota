## Why

Walking through the registration flow against the existing `auth` spec uncovered five concrete code-vs-spec gaps. **Most of these aren't "new rules" — the spec already describes the correct behaviour, but the code does not enforce it.** A walk-through:

1. **Setup tokens leak into Caddy access logs as raw query parameters.** The `auth` spec already states "raw tokens SHALL NOT be stored" for the DB; we extend that intent — and verify the deployment honours it — for log files. Anyone with read access to `/data/access.log` (ops, log shippers, Docker volume backups) can replay an unused token within its 72h TTL and hijack the pending account.
2. **Setup-page responses do not pin a `Referrer-Policy`.** The `auth` spec already delegates security headers to Caddy; we verify that `Referrer-Policy: no-referrer` is in fact configured. If the page ever loads any cross-origin resource (logo, font, analytics), browsers may leak the URL — including the token — via the `Referer` header to that origin.
3. **`MarkUsed` discards `result.RowsAffected()`.** The `auth` spec's error catalog already defines `TOKEN_USED` (410) for the already-used case, and the SQL uses `WHERE used_at IS NULL` defensively. But the Go layer ignores `RowsAffected`, so two concurrent `POST /auth/setup-password` calls with the same token both return 204 — the second silently overwrites the first user's password. The spec's single-use guarantee is broken at the application layer.
4. **Backend has no minimum-length password check.** The `auth` spec's "Password policy" requirement says `≥ 8 characters` and the error catalog defines `PASSWORD_TOO_SHORT` (400). Frontend has zod `min(8)`, but a direct API call (`curl POST /auth/setup-password`) bypasses validation and the backend silently accepts a 1-character password. Spec drift, code is too permissive.
5. **Email send failures during invitation are fire-and-forget.** When the school SMTP gateway is down, admin sees `201 Created` for every user but some never receive an email. There is no audit trail and no log signal for the admin to diagnose. The `auth` spec's "Emitted audit actions" requirement does not currently include this case — this is the single new spec rule we add.

None of these are bugs in the strict sense (no crash, no data loss). All five are **edge-case correctness** problems — situations where the current code reaches an inconsistent state, or fails to enforce a guarantee the spec already promises.

## What Changes

- **Caddy access log scrubbing**: configure the Caddy access-log format to omit query parameters (use `request>uri_path` or equivalent), so raw setup tokens never enter `/data/access.log`. Update Caddy security-header block to set `Referrer-Policy: no-referrer` if not already present.
- **Strict single-use token consumption**: modify `MarkUsed` (and any analogous helpers) in `backend/internal/repository/setup_token.go` to capture `result.RowsAffected()` and return the existing `ErrTokenUsed` sentinel when `RowsAffected == 0`. The handler already maps `ErrTokenUsed` → `TOKEN_USED` 410; the gap is purely between repo and service.
- **Backend-side password length check**: in `authService.SetupPassword`, add `utf8.RuneCountInString(password) < 8 → ErrPasswordTooShort`. Use the existing `PASSWORD_TOO_SHORT` (400) error code (already in the spec's error catalog). Frontend zod is unchanged (already `min(8)`).
- **Email-failure audit**: in `userService.CreateUser` and `userService.ResendInvitation` (and `authService.RequestPasswordReset`), capture the result of `sendInvitation` / `sendPasswordReset`. On failure, write a new audit event with action `user.invitation.email_failed` and metadata `{ user_id, email, error }`; also log at WARN. The user row and token row remain (admin can ResendInvitation later); the caller still gets the success response (no need to fail the whole request because email is async).
- **Verify, don't add, the `ResendInvitation` token-invalidation behaviour**: the spec already requires `Token issuance invalidates prior same-purpose tokens`. We verify the existing code calls `InvalidateUnusedTokens` before `issueToken`; if not, fix it to comply. No new spec content.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `auth`:
  - `Setup token table schema` — clarify that raw tokens SHALL NOT enter access logs (extension of the existing "not stored" wording to cover log files).
  - `Setup password consumption` — add a scenario for concurrent token consumption (the second consumer must observe `TOKEN_USED` 410, not silent success).
  - `Password policy` — add a scenario making explicit that the backend enforces the minimum length even when called directly (frontend bypass).
  - `Security headers delegated to proxy` — add a scenario that explicitly names `Referrer-Policy: no-referrer`.
  - `Emitted audit actions` — add `user.invitation.email_failed` to the action taxonomy.

## Non-goals

The following are deliberately excluded:

- **OOB (out-of-band) verification codes for invitations / password resets** — token-in-URL via email is industry standard; the existing DB-level mitigations (hashed in DB, TTL, single-use) plus this change's log-scrubbing make this acceptable for our scope.
- **Stronger password complexity rules** (digit + symbol requirements, dictionary checks, zxcvbn) — minimum length is the floor; richer rules are a separate UX/policy decision.
- **Bootstrap admin password rotation enforcement** — known scope-1 gap, accepted.
- **Login response uniformity (account-enumeration hardening)** — known scope-1 gap, accepted.
- **Session invalidation on user disable** — separate concern, separate change.
- **Per-IP rate limit on `setup-password` / `setup-token`** — not in this scope; existing rate limits cover login and password-reset request endpoints.
- **Admin UI surface for failed email deliveries** — the audit log is queryable directly; UI is a future change.

## Impact

- **Backend code**:
  - `backend/internal/repository/setup_token.go`: `MarkUsed` (and any `MarkOldestPending`-style helpers) capture `RowsAffected`, return existing `ErrTokenUsed` on `0`.
  - `backend/internal/service/auth.go`: `SetupPassword` adds `utf8.RuneCountInString(password) < 8` check returning the existing `ErrPasswordTooShort`. Aliases `ErrTokenUsed` if not already.
  - `backend/internal/service/user.go`: `CreateUser` and `ResendInvitation` capture `sendInvitation` error → audit + WARN-log; do not fail the whole request.
  - `backend/internal/service/setup.go`: same audit + WARN treatment for `sendPasswordReset` flow.
  - `backend/internal/audit/audit.go`: new constant `ActionUserInvitationEmailFailed = "user.invitation.email_failed"`.
- **Backend tests**:
  - Repository integration test: prove `MarkUsed` returns `ErrTokenUsed` on the second call.
  - Service test: prove `SetupPassword` rejects password under 8 runes with `ErrPasswordTooShort`.
  - Service test: prove `CreateUser` writes the new audit event when `sendInvitation` returns an error (use a stub emailer that returns a configurable error).
  - Verify (no code change expected) that `issueToken` calls `InvalidateUnusedTokens` per the existing spec; if missing, add coverage and the call site.
- **Frontend**:
  - No change. Existing zod `min(8)` and existing handling of `TOKEN_USED` / `PASSWORD_TOO_SHORT` cover the new server behaviour.
- **Caddy / deployment**:
  - `frontend/Caddyfile`: add or verify `Referrer-Policy: no-referrer` in the security headers block; change the access-log format to omit query strings (or post-process to redact `?token=...`).
- **Specs**: `openspec/specs/auth/spec.md` — five MODIFIED requirements (see "Modified Capabilities").
- **No schema migration**: zero DB change.
- **No new dependency**: zero new Go module or npm package.
