## Context

Email is the login credential, but currently only an admin can change a user's email (via `PUT /users/{id}`). There's no self-service path, and even if there were, naively letting any logged-in session swap the email would let a stolen session pivot account access without ownership proof of the new mailbox. The user-settings-page change explicitly punted this — it called for a verification flow rather than a one-shot endpoint.

This change adds the standard request → confirm pattern: the user enters the new email and re-enters their current password; the server emails a one-time link to the **new** address; clicking the link swaps the email and revokes all sessions, forcing a fresh login. The old address gets a heads-up email so a compromised session can't quietly steal the account. Tokens reuse the existing `user_setup_tokens` table with a new `purpose = 'email_change'` value plus a nullable `new_email` column.

## Goals / Non-Goals

**Goals:**

- Authenticated user can request an email change from `/settings`. Request requires re-entering the current password.
- Confirmation email goes to the new address; old address receives a notification (not a link, just a heads-up).
- Token lives 24 hours, single-use, hashed at rest. Re-requesting invalidates the prior token.
- Confirmation endpoint is anonymous (clicked from email); on success it swaps the email and revokes ALL sessions.
- Audit trail: `user.email_change.request` on request, `user.email_change.confirm` on confirm.
- No new error codes; reuse the existing setup-token rejection vocabulary plus `EMAIL_ALREADY_EXISTS` and `INVALID_CURRENT_PASSWORD`.

**Non-Goals:**

- Rate limiting on the new endpoints. The current-password gate is the practical bottleneck for the request side; the token-rejection branch on the confirm side is constant-time relative to attacker input.
- Auth on the confirm endpoint. The token IS the credential.
- "Undo email change" semantics. After confirmation, reverting requires another full request → confirm cycle.
- Custom token TTLs.
- Multi-locale email subjects beyond zh / en (the existing template structure).
- Allowing the new email to belong to a `disabled` or `pending` user. Collision check is across all users regardless of status.

## Decisions

### D-1. Schema — extend `user_setup_tokens`

Migration `00018_extend_setup_tokens_for_email_change.sql`:

```sql
-- +goose Up
ALTER TABLE user_setup_tokens
    DROP CONSTRAINT user_setup_tokens_purpose_check;
ALTER TABLE user_setup_tokens
    ADD CONSTRAINT user_setup_tokens_purpose_check
    CHECK (purpose IN ('invitation', 'password_reset', 'email_change'));
ALTER TABLE user_setup_tokens
    ADD COLUMN new_email TEXT NULL;
ALTER TABLE user_setup_tokens
    ADD CONSTRAINT user_setup_tokens_email_change_has_new_email
    CHECK (
        (purpose = 'email_change' AND new_email IS NOT NULL)
        OR (purpose <> 'email_change' AND new_email IS NULL)
    );

-- +goose Down
ALTER TABLE user_setup_tokens
    DROP CONSTRAINT user_setup_tokens_email_change_has_new_email;
ALTER TABLE user_setup_tokens
    DROP COLUMN new_email;
ALTER TABLE user_setup_tokens
    DROP CONSTRAINT user_setup_tokens_purpose_check;
ALTER TABLE user_setup_tokens
    ADD CONSTRAINT user_setup_tokens_purpose_check
    CHECK (purpose IN ('invitation', 'password_reset'));
```

The two-pronged CHECK ("email_change requires new_email; everything else forbids it") guarantees the column is correctly populated and never accidentally leaks into invitation / reset rows. Existing rows are unaffected — they have `new_email IS NULL` and `purpose IN ('invitation', 'password_reset')`, both branches accept them.

**Rejected — separate `email_change_tokens` table.** Forks token consumption / invalidation logic for marginal isolation gain. The token machinery is identical (hash, single-use, TTL).

**Rejected — JSON column for purpose-specific extras.** Tiny use case; explicit column is simpler and queryable.

### D-2. Request endpoint — `POST /users/me/email-change-request`

`RequireAuth`. Body:

```json
{ "new_email": "alice@example.com", "current_password": "..." }
```

Service flow (transactional):

1. Resolve viewer's user_id from session.
2. Fetch viewer's row with `FOR UPDATE`.
3. `bcrypt.CompareHashAndPassword(viewer.password_hash, current_password)` — on mismatch, return `ErrInvalidCurrentPassword` → HTTP 401 / `INVALID_CURRENT_PASSWORD`.
4. Trim + normalize `new_email`; validate via `mail.ParseAddress`. On invalid shape: HTTP 400 / `INVALID_REQUEST`.
5. If `LowerCase(new_email) == LowerCase(viewer.email)`: HTTP 400 / `INVALID_REQUEST`. (No-op; same address.)
6. `SELECT id FROM users WHERE email = $new_email` — if any row exists, HTTP 409 / `EMAIL_ALREADY_EXISTS`.
7. Invalidate prior unused email_change tokens for this user via the existing `InvalidateUnusedTokens(user_id, 'email_change', now)` helper.
8. Generate a 32-byte random raw token; SHA-256 hash; insert `user_setup_tokens` row with `purpose='email_change'`, `new_email=<normalized>`, `expires_at = now + 24h`.
9. Enqueue (via outbox in the same transaction) two emails:
   - **Confirmation** to `new_email`, body containing `<APP_BASE_URL>/auth/confirm-email-change?token=<raw>` (raw token in URL).
   - **Heads-up** to the user's current `users.email`, body explaining a change request was made and to act if it wasn't them (no link to act, just a notification — they can change password if they suspect compromise).
10. Emit `user.email_change.request` audit event with metadata `{ user_id, new_email_normalized }`. The audit metadata SHALL NOT include the raw token, the token hash, or the current password.
11. Return 202 Accepted (empty body).

The transaction guarantees that two concurrent requests can't both win — only one's `InvalidateUnusedTokens` + insert sequence completes; the other sees a stale view. (In practice the second request supersedes the first by invalidating it.)

**Rejected — return the new_email in the response so the UI can display "sent to <X>".** The frontend already has `new_email` from the form input; no need to round-trip.

**Rejected — also accept `confirm_password` field for "type your new email twice."** The confirmation email IS the second-confirmation step; double-typing the email is friction without security gain.

### D-3. Confirm endpoint — `POST /auth/confirm-email-change`

**Anonymous** (no middleware), parallel to `POST /auth/setup-password`. Body:

```json
{ "token": "..." }
```

Service flow (transactional):

1. SHA-256-hash the raw token; `SELECT * FROM user_setup_tokens WHERE token_hash = $hash AND purpose = 'email_change'`.
2. Standard rejection branches in order:
   - Row not found → `ErrTokenNotFound` → HTTP 404 / `TOKEN_NOT_FOUND`.
   - `used_at IS NOT NULL` → `ErrTokenUsed` → HTTP 410 / `TOKEN_USED`.
   - `expires_at <= now` → `ErrTokenExpired` → HTTP 410 / `TOKEN_EXPIRED`.
   - Bad token shape (raw not 32 bytes hex etc.) → `ErrInvalidToken` → HTTP 400 / `INVALID_TOKEN` (validated before the DB lookup).
3. CAS the token used: `UPDATE user_setup_tokens SET used_at = now WHERE id = $tokenID AND used_at IS NULL`. `RowsAffected == 0` → `ErrTokenUsed` (concurrency loser).
4. Re-check email uniqueness inside the transaction: `SELECT id FROM users WHERE email = $new_email AND id <> $token.user_id`. If any: `ErrEmailAlreadyExists` → HTTP 409 / `EMAIL_ALREADY_EXISTS`.
5. `UPDATE users SET email = $new_email, version = version + 1 WHERE id = $token.user_id`.
6. **Revoke ALL sessions for the user** — `DELETE FROM sessions WHERE user_id = $token.user_id`. The link is anonymous, so the requester isn't necessarily the caller; force everyone (including the original tab) to re-authenticate with the new email.
7. Invalidate every other unused token for this user across all purposes (mirrors `setup-password`'s belt-and-braces clause): `UPDATE user_setup_tokens SET expires_at = now WHERE user_id = $token.user_id AND id <> $tokenID AND used_at IS NULL`.
8. Emit `user.email_change.confirm` audit event with metadata `{ user_id, new_email_normalized, revoked_session_count }`. No raw token, no hash, no password.
9. Return 204 No Content.

The endpoint accepts the token alone — no email or user_id in the body — because the token row carries the user_id.

**Rejected — require the user to log in first, then confirm.** The whole point of the email round-trip is that the link works regardless of session state. Forcing pre-login defeats the verification.

**Rejected — preserve the requester's session and only revoke OTHER sessions.** With anonymous confirmation, "current session" is meaningful only if the user clicks from the same browser they requested from. The semantics get murky; "revoke all" is unambiguous and matches typical security expectation for credential changes.

### D-4. Outbox email templates

Two new templates added to the existing email module (alongside `invitation` and `password_reset`):

- **`email_change_confirm`** — sent to the new address. Subject (zh): "确认邮箱变更"; (en): "Confirm your email change". Body has a single CTA link `{{ confirm_url }}` (the `?token=` URL) and a 24-hour expiry note.
- **`email_change_notice`** — sent to the old address. Subject (zh): "邮箱变更请求"; (en): "Email change requested". Body explains: "An email-change request was issued for your account. The new address is `{{ new_email_partial }}`. If this wasn't you, change your password immediately." `new_email_partial` is the new email with the local-part partially masked (`a***@example.com`) so it doesn't fully leak the new address to the old inbox if the old account is compromised.

Both templates render zh / en per the existing i18n template path.

The two emails are enqueued in the same transaction as the token insert via `OutboxRepository.EnqueueTx`, identical to invitation-email enqueue.

**Rejected — single template combining both messages.** Different recipients, different content; combining adds branching logic to the renderer for no win.

**Rejected — partial-masking the new email in the heads-up.** Wait — I just argued for it above. Sticking with masking. A few characters help if the old mailbox is compromised by the same actor doing the change.

### D-5. Frontend — Settings dialog + public confirmation route

**Settings page** gets a new section between Profile and Password:

```
┌─ 邮箱 ─────────────────────────────────┐
│  当前邮箱: alice@example.com             │
│  [更改邮箱]                               │
└──────────────────────────────────────────┘
```

Clicking "更改邮箱" opens a dialog:

```
┌─ 更改邮箱 ──────────────────────────────┐
│  新邮箱:    [______________________]    │
│  当前密码:  [______________________]    │
│                                          │
│            [取消]    [发送确认邮件]       │
└──────────────────────────────────────────┘
```

On submit: call the request endpoint, show a success state inside the dialog ("确认邮件已发送至 alice@example.com，请查收并点击链接完成更改"), close after a few seconds. The current-email display on the settings page does NOT update — it updates after the user clicks the email link and re-logs in.

**New public route `/auth/confirm-email-change`**:

- Reads `?token=<raw>` from the URL.
- On mount: call `POST /auth/confirm-email-change`.
- Renders one of:
  - Success: "邮箱已更新，请使用新邮箱重新登录" + button to `/login`.
  - Error (invalid / not_found): "链接无效或已失效".
  - Error (used): "此链接已使用过".
  - Error (expired): "链接已过期，请重新发起邮箱变更请求".
  - Error (email_already_exists): "该邮箱已被其他账号使用，请联系管理员".

The route does NOT require auth — it works whether the user is logged in or not.

**Rejected — confirm in-page within the settings dialog.** The dialog requires an authenticated session for the request endpoint; the confirm endpoint is anonymous and clicked from email. Different flows, different routes.

### D-6. Audit + error code surface

Two new audit actions:

- `user.email_change.request` — emitted on successful request. Metadata: `{ user_id, new_email_normalized }`.
- `user.email_change.confirm` — emitted on successful confirmation. Metadata: `{ user_id, new_email_normalized, revoked_session_count }`.

Audit metadata SHALL NOT carry: raw tokens, token hashes, password values, or `Set-Cookie` headers.

No new error codes. The endpoints reuse the existing catalog:

- `INVALID_CURRENT_PASSWORD` (401) — wrong password on request.
- `INVALID_REQUEST` (400) — malformed body, bad email shape, no-op same-as-current.
- `EMAIL_ALREADY_EXISTS` (409) — collision at request OR at confirm time.
- `INVALID_TOKEN` (400) — bad token shape on confirm.
- `TOKEN_NOT_FOUND` (404) — unknown hash.
- `TOKEN_USED` (410) — already consumed.
- `TOKEN_EXPIRED` (410) — past TTL.

### D-7. Token TTL

24 hours from issuance. Falls between `Invitation token TTL` (7 days) and `password_reset` TTL (1 hour). Email change is more sensitive than invitation (account access pivot), less sensitive than password reset (which is anonymous-initiated). 24 hours respects the user's ability to confirm later in the day or next morning without expiring the token mid-coffee.

### D-8. Tests

Backend:

- Unit `service.RequestEmailChange`:
  - wrong current password → `ErrInvalidCurrentPassword`.
  - invalid email shape → `ErrInvalidRequest` (or whichever sentinel; mapped to `INVALID_REQUEST` at the handler).
  - same-as-current email → `ErrInvalidRequest`.
  - new email collides with another user → `ErrEmailAlreadyExists`.
  - happy path → token row with `purpose='email_change'`, `new_email` set, `expires_at` ≈ now+24h; both outbox rows enqueued; audit event emitted.
  - re-request invalidates prior unused email_change tokens.
- Unit `service.ConfirmEmailChange`:
  - bad token shape → `ErrInvalidToken`.
  - unknown hash → `ErrTokenNotFound`.
  - used row → `ErrTokenUsed`.
  - expired row → `ErrTokenExpired`.
  - happy path → `users.email` updated, `version` incremented, all sessions revoked, sibling tokens invalidated, audit event emitted.
  - email-uniqueness race: another row gets the same email between request and confirm → `ErrEmailAlreadyExists` at confirm time.
  - concurrent CAS: two simultaneous confirms with the same token → one wins (204), other gets `TOKEN_USED`.
- Integration: end-to-end request → outbox content (verify confirmation URL contains the token) → confirm → re-login with new email succeeds, old email rejected.

Frontend:

- Component `<EmailForm>` (in settings): new_email shape validation; current_password required; success state shown after request mutation succeeds.
- Component `<ConfirmEmailChangePage>`: the five render branches (success + 4 error variants).
- Schema: zod validates the request shape.

## Risks / Trade-offs

- **Risk:** confirmation email lands in spam, user never clicks → token expires → user re-requests, gets a fresh one. → Acceptable: the failure mode is just "user retries"; no data corruption.
- **Risk:** old-email heads-up has a partial-masked new email; an attacker scraping a compromised old inbox can still infer the domain and first letter. → Acceptable: complete redaction would defeat the point of the notice ("an email-change went out, no info"). Partial masking is a deliberate compromise.
- **Risk:** the CHECK constraint `email_change ⇒ new_email IS NOT NULL` blocks any existing data path that inserts email_change rows without `new_email`. → No risk: there's no such existing data; the column is brand new.
- **Trade-off:** revoking ALL sessions on confirm logs the requester out of their current tab — they'll see "session expired" if they refresh. Mild UX hit; acceptable security floor.
- **Trade-off:** requesting an email change requires the current password even though the user is already logged in. → Acceptable: consistent with `change-password`; standard re-authentication for credential mutation.

## Migration Plan

Single shipping unit:

1. Apply migration `00018_extend_setup_tokens_for_email_change.sql` via `make migrate-up`.
2. Backend rebuilt + tested.
3. Frontend rebuilt + tested.
4. Manual smoke: log in as Alice, navigate to `/settings`, click 更改邮箱, enter `alice2@example.com` + correct password, observe success toast. Inspect outbox table — two rows enqueued. Tail outbox worker logs / use a local SMTP catcher to capture the confirmation URL. Open `<app>/auth/confirm-email-change?token=<raw>` in a fresh incognito window — observe success page. Try logging in with old email → 401. Log in with new email → 200. Original tab refresh → session expired.

Rollback = `make migrate-down 1` (drops the constraint changes + new column) + revert the change. Any in-flight email_change rows are wiped by the migration's CHECK rollback.

## Open Questions

None.
