## Why

Email is the login credential, but right now the only way for a user's email to change is for an admin to overwrite it via `PUT /users/{id}` — which is fine for typo fixes but isn't self-service, doesn't verify the new address belongs to the user, and gives no audit trail of "who initiated this." The `user-settings-page` change deliberately deferred email change because it needs a verification round-trip: anyone with a session cookie shouldn't be able to silently re-route account access by punching in a new email.

This change adds the standard two-step flow: **request → confirm via emailed link**. The user proves they control the new mailbox before the swap; the old mailbox gets a heads-up so a compromised session can't quietly steal the account; tokens are stored in the existing `user_setup_tokens` table with a new `purpose = 'email_change'` value plus a nullable `new_email` column so we don't fork the token machinery.

## What Changes

### Backend

- **Schema** — `user_setup_tokens` grows two changes via `migrations/00018_extend_setup_tokens_for_email_change.sql`:
  - `purpose` CHECK constraint widens to `IN ('invitation','password_reset','email_change')`.
  - New nullable column `new_email TEXT` carrying the requested target address. NULL for invitation / password_reset rows; required for email_change rows (enforced by partial CHECK or by the service layer).
- **New endpoint `POST /users/me/email-change-request`** — `RequireAuth`. Body `{ new_email, current_password }`. Inside one transaction:
  1. Re-verify `current_password` against `users.password_hash` (defense against pure-cookie session theft).
  2. Validate `new_email` shape via `mail.ParseAddress`; trim/normalize.
  3. Reject with HTTP 400 / `INVALID_REQUEST` if `new_email == users.email` (no-op).
  4. Reject with HTTP 409 / `EMAIL_ALREADY_EXISTS` if another user already has this email.
  5. Invalidate any prior unused `email_change` tokens for this user (per the existing `Token issuance invalidates prior same-purpose tokens` requirement).
  6. Generate a fresh raw token + SHA-256 hash; insert a new `user_setup_tokens` row with `purpose='email_change'`, `new_email=<new>`, `expires_at = now + 24h`.
  7. Enqueue (via outbox) a confirmation email **to the new address** containing a link `<APP_BASE_URL>/auth/confirm-email-change?token=<raw>`.
  8. Enqueue (via outbox) a heads-up email **to the old address** along the lines of "an email-change request was issued; if this wasn't you, change your password."
  9. Emit `user.email_change.request` audit event (metadata: `user_id`, `new_email_normalized`).
  10. Return 202 Accepted (response body empty; the user's next move is to click the link in their inbox).
  - Wrong `current_password` → HTTP 401 / `INVALID_CURRENT_PASSWORD` (already exists from `user-settings-page`).
- **New endpoint `POST /auth/confirm-email-change`** — **anonymous** (no middleware), mirrors the public `POST /auth/setup-password` shape. Body `{ token }`. Inside one transaction:
  1. Hash the raw token; resolve the row; reject with `INVALID_TOKEN` / `TOKEN_NOT_FOUND` / `TOKEN_USED` / `TOKEN_EXPIRED` per the existing setup-token rejection vocabulary.
  2. Mark the token used (single-use CAS — same pattern as setup-password).
  3. Re-check that `new_email` is still unique among `users` (in case another row claimed it between request and confirm). On collision, return HTTP 409 / `EMAIL_ALREADY_EXISTS`.
  4. `UPDATE users SET email = $new_email, version = version + 1 WHERE id = $userID`.
  5. **Revoke ALL sessions for this user** — the link is anonymous, so the user is forced to log in again with the new email. This includes any session that was sitting open in the requester's tab.
  6. Invalidate every other unused token for that user (across all purposes) — same belt-and-braces clause as `setup-password` consumption.
  7. Emit `user.email_change.confirm` audit event (metadata: `user_id`, `new_email_normalized`).
  8. Return 204 No Content.
- **Outbox** — two new email templates: `email_change_confirm` (to new address) and `email_change_notice` (to old address). Both bilingual (zh / en) following the existing template structure.

### Frontend

- **Settings page** — new section "Email" between Profile and Password, showing the current email read-only with a "更改邮箱 / Change email" button that opens a dialog with `new_email` + `current_password` inputs. On submit: call the request endpoint, show a success toast/state "确认邮件已发送至 <new_email>", and close the dialog. The current email on the page does NOT update yet — the swap happens only after the user clicks the email link.
- **New public route `/auth/confirm-email-change`** — reads `?token=` from the URL, calls the confirm endpoint, shows success state ("邮箱已更新，请用新邮箱重新登录") with a button to `/login`, or an error state for the four token-rejection cases. No auth required.
- **i18n** — strings for the new section, dialog, success/error states, and the two email templates.

### Capabilities

#### New Capabilities

None.

#### Modified Capabilities

- `auth`:
  - *Setup token table schema* — `purpose` enum widens to include `'email_change'`; new nullable `new_email TEXT` column.
  - *Auth and user API routing* — adds `POST /users/me/email-change-request` and `POST /auth/confirm-email-change`.
  - *Canonical error codes* — no new codes (we reuse existing `INVALID_REQUEST`, `EMAIL_ALREADY_EXISTS`, `INVALID_TOKEN`, `TOKEN_NOT_FOUND`, `TOKEN_USED`, `TOKEN_EXPIRED`, `INVALID_CURRENT_PASSWORD`).
  - *Emitted audit actions* — adds `user.email_change.request` and `user.email_change.confirm`.
  - One new requirement: *Authenticated user requests email change*.
  - One new requirement: *Email change confirmation*.

## Non-goals

- **Self-service email change without password re-verification.** Always require the current password. Yes, the user is already logged in; that's not enough for a credential change.
- **Allowing the new email to belong to a `disabled` or `pending` user.** The collision check is against ALL users regardless of status — a disabled email is still a unique identifier we don't want to recycle.
- **Rolling back an email change after confirmation.** Once confirmed, reverting requires another full request → confirm cycle. No "undo" button.
- **Bulk email migration tools.** This is a per-user flow, not an admin migration utility.
- **Verifying the new email address by retrying SMTP delivery.** The outbox worker already handles delivery retries; if the email doesn't arrive, the user clicks "Change email" again and gets a fresh token.
- **Custom token TTLs per user or per request.** 24h for everyone. Period.
- **Localized email subject lines beyond the existing zh/en pair.** No new locales.

## Impact

- **Backend code:**
  - `migrations/00018_extend_setup_tokens_for_email_change.sql` (new).
  - `backend/internal/repository/setup_token.go` (or wherever) — extend insert/select to handle `new_email` column.
  - `backend/internal/repository/user.go` — new method to update email + bump version.
  - `backend/internal/service/auth.go` — new methods `RequestEmailChange` and `ConfirmEmailChange`.
  - `backend/internal/handler/auth.go` and `user.go` — two new handlers.
  - `backend/internal/email/templates.go` (or equivalent) — two new template renderers.
  - `backend/cmd/server/main.go` — wire the new routes.
  - Tests: unit + integration for both flows + audit + outbox enqueue.
- **Frontend code:**
  - `frontend/src/components/settings/email-form.tsx` (new, dialog-style).
  - `frontend/src/routes/auth/confirm-email-change.tsx` (new public route).
  - `frontend/src/components/settings/settings-api.ts` — add `requestEmailChangeMutation`, `confirmEmailChangeMutation`.
  - i18n strings.
- **Spec:** four `auth` requirements modified + two added.
- **No new dependencies.** No infra / config changes.
- **No new audit data sensitivity.** Audit metadata carries `user_id` + `new_email_normalized` — no token, no password.

## Risks / safeguards

- **Risk:** the heads-up email to the old address fails to deliver (stale mailbox, bounce). → Mitigation: outbox worker retries per its existing budget; if exhausted, a `user.invitation.email_failed`-style audit event surfaces. The change-confirm flow doesn't depend on the heads-up landing — it's purely a notification.
- **Risk:** between request and confirm, another user signs up with the same email. → Mitigation: the confirm endpoint re-checks uniqueness inside its transaction. Loser gets HTTP 409.
- **Risk:** an attacker with session-cookie access spams email-change requests to harvest tokens or noise the user's mailboxes. → Mitigation: `Token issuance invalidates prior same-purpose tokens` already wipes prior unused tokens on each new request; the heads-up email gives the user visibility; current-password gate stops pure-cookie pivots.
- **Risk:** rate-limit absent on the new request endpoint. → Mitigation: not added in this change; the current-password check is the practical bottleneck for an attacker. If we see abuse, add a per-user rate limit later.
- **Risk:** old email column carries trailing whitespace or different case than new email. → Mitigation: normalize both via the same trim path used for `POST /users` admin creation; lowercase comparison for the no-op check.
