## Context

The registration / setup flow is `admin POST /users` → user clicks email link → `GET /auth/setup-token` (preview) → `POST /auth/setup-password` (consume + activate) → `POST /auth/login`. The `auth` spec describes how every step should behave. The code mostly does, but five edges drift:

1. Caddy access logs persist raw setup tokens as URL query parameters — violates the "raw tokens not in logs" spirit of the schema-level "raw tokens SHALL NOT be stored" rule.
2. `Referrer-Policy` may not be set; the spec delegates security headers to Caddy but does not name this header.
3. `MarkUsed` discards `RowsAffected`; concurrent token consumption silently succeeds, contradicting the existing `TOKEN_USED` (410) error in the catalog.
4. `SetupPassword` does not enforce the spec's `≥ 8 character` floor at the backend — frontend zod prevents it but a direct API call bypasses.
5. `sendInvitation` / `sendPasswordReset` are fire-and-forget; SMTP failures are silent. Spec's audit-action list does not currently cover this.

This change is mostly **spec compliance**: making the code do what the spec already says. Items 1, 2 are deployment-config compliance (Caddyfile). Items 3, 4 are pure backend work — surfacing existing sentinels to existing handler mappings. Item 5 is the only net-new spec rule (one new audit action).

## Goals / Non-Goals

**Goals:**

- Make `MarkUsed` return `ErrTokenUsed` strictly when `RowsAffected == 0`, so concurrent consumers observe `TOKEN_USED` 410 instead of silent success.
- Pin a backend-side `≥ 8` rune-count floor on `SetupPassword`, returning the existing `ErrPasswordTooShort` (`PASSWORD_TOO_SHORT` 400).
- Emit a new audit event `user.invitation.email_failed` whenever the post-tx invitation email send fails; same treatment for `sendPasswordReset`. Caller (admin) still gets the success response.
- Update Caddy access-log format to omit query parameters, so raw tokens don't enter `/data/access.log`.
- Confirm or add `Referrer-Policy: no-referrer` in the Caddy security-header block.

**Non-Goals:**

- New error codes — we use the existing `TOKEN_USED` and `PASSWORD_TOO_SHORT`.
- New schema migration.
- OOB verification codes; richer password rules; bootstrap admin rotation; login enumeration hardening; session invalidation on disable.
- An admin UI surface for "users whose invitation email failed."

## Decisions

### `MarkUsed` captures `RowsAffected`, returns `ErrTokenUsed` on zero

Today:

```go
_, err := r.db.ExecContext(ctx, query, userID, purpose, usedAt)
return err
```

After:

```go
result, err := r.db.ExecContext(ctx, query, ...)
if err != nil {
    return err
}
affected, err := result.RowsAffected()
if err != nil {
    return err
}
if affected == 0 {
    return ErrTokenUsed
}
return nil
```

The handler already maps `ErrTokenUsed` → `TOKEN_USED` (410). The gap is purely repo→service→handler error propagation. No new sentinel, no new code path in the handler.

The same change applies to any other `setup_tokens` `UPDATE ... WHERE used_at IS NULL` helper that exists today — they all share the same pattern of relying on the SQL predicate but discarding the affected count.

**Rejected**: introducing a new sentinel `ErrSetupTokenAlreadyUsed`. Rejected because the existing `ErrTokenUsed` already names this case, and the existing 410 mapping is correct. Adding a parallel name would just create coding-conventions drift.

### Backend password floor, using rune count

In `authService.SetupPassword`, before the bcrypt step:

```go
if utf8.RuneCountInString(password) < 8 {
    return ErrPasswordTooShort
}
```

Why `RuneCountInString` not `len`: the existing spec says "8 characters." A user who picks a Chinese passphrase like `你好世界你好` is 6 runes / 18 bytes — we want the rune count (= visible characters) for a meaningful length floor.

`ErrPasswordTooShort` is the existing sentinel; `PASSWORD_TOO_SHORT` (400) is in the existing error-code catalog. Frontend zod has `.min(8)` already, so this only catches direct-API-call bypass.

**Rejected**: composition rules (digit + letter + symbol). Rejected because (a) it pushes users to predictable patterns like `Password1!`, (b) NIST SP 800-63B explicitly recommends against composition rules.

### Email-failure audit + WARN log

`sendInvitation` is currently fire-and-forget after the tx commits. We change the call sites in `userService.CreateUser` and `userService.ResendInvitation` to capture the error:

```go
if err := s.setupFlows.sendInvitation(ctx, user, rawToken); err != nil {
    targetID := user.ID
    audit.Record(ctx, audit.Event{
        Action:     audit.ActionUserInvitationEmailFailed,
        TargetType: audit.TargetTypeUser,
        TargetID:   &targetID,
        Metadata: map[string]any{
            "email": user.Email,
            "error": err.Error(),
        },
    })
    s.logger.Warn("invitation email failed",
        "user_id", user.ID, "email", user.Email, "error", err)
}
```

The user row and the setup token row remain in the database. The caller (admin) still gets the success response — the user is created — they discover the email failure via:

1. The audit log: `SELECT ... WHERE action='user.invitation.email_failed' ORDER BY created_at DESC`
2. WARN log lines in stderr / docker logs

**Rejected**: failing the entire `POST /users` call when email fails. Rejected because email delivery is asynchronous and unreliable by design; a 30-second SMTP timeout would block the admin's UI, and the user is still validly created — they just need a re-send. Surfacing in audit + log is the right path.

**Rejected**: storing email status as a column on `users`. Rejected as scope creep; the audit trail is enough.

The same treatment applies to `sendPasswordReset` for symmetry, though for password-reset the audit is less critical (the user already has an active account; they can request again). We extend the same audit action's metadata or create a parallel one — for symmetry we add only `user.invitation.email_failed` for the invitation path; password-reset's failure is logged at WARN but not audited as a distinct action (the existing `auth.password_reset.request` already records the request; absence of follow-through can be inferred). This keeps the spec's audit taxonomy minimal.

### Caddy access log: omit query parameters

Two reasonable approaches:

```
(a) Switch the Caddy log format to use `request>uri_path` instead of
    `request>uri`. The path field excludes the query string by design.

(b) Keep `request>uri` but add a runtime token-redaction step (e.g.,
    a `transform` directive matching `?token=*`).
```

We pick **(a)** because:
- One-line change in Caddyfile.
- Path-only is a strictly safer default — no risk of regex misses.
- The query string is not normally useful for ops debugging (the path + status + duration tell us what we need).

**Rejected**: dropping access logging entirely. Rejected because the rest of the access log is genuinely useful for ops debugging.

### `Referrer-Policy: no-referrer` at Caddy

The spec already says "Security headers delegated to proxy" — set the header globally at Caddy, not per-route in Go. Concretely:

```
header {
    ...
    Referrer-Policy "no-referrer"
}
```

We pick `no-referrer` over `strict-origin-when-cross-origin` because:
- Strictly stronger: zero outbound `Referer` ever.
- The app has no analytics, no inbound-referer logic — zero practical downside.
- The browser default (`strict-origin-when-cross-origin`) would also strip query params before sending Referer cross-origin and would suffice for the leak we worry about. Choosing `no-referrer` is defense-in-depth.

If the Caddyfile already has `Referrer-Policy` set (for example in an existing `header` block we may have overlooked), this task is a no-op verification.

### Verify, do not re-spec, the `ResendInvitation` prior-token invalidation

The `auth` spec's "Token issuance invalidates prior same-purpose tokens" requirement already mandates that `issueToken` calls `InvalidateUnusedTokens(userID, purpose, now)` before creating the new token. During implementation:

1. Read `setupFlows.issueToken` and confirm it calls `InvalidateUnusedTokens` first.
2. If yes — covered by tests, no change needed.
3. If no — that is pre-existing spec drift, fix the call sequence and add the missing test. No new spec content needed.

This is purely a verification step; either way, there is no new requirement.

## Risks / Trade-offs

- **Risk**: Caddy log change leaves us blind to which token request hit which user during incident response.
  **Mitigation**: the `audit_logs` table already records who-did-what for every meaningful action; access log is defense-in-depth, not the primary forensic source.
- **Risk**: setting `Referrer-Policy: no-referrer` globally could break a future feature that relies on inbound `Referer`.
  **Mitigation**: the app has no analytics, no inbound-referer logic; this is a safe default for our scope.
- **Risk**: emails that intermittently fail and then succeed (transient SMTP errors with retry at the emailer) might emit a spurious audit event even though the user does receive the email eventually.
  **Mitigation**: the audit event records "send-call returned an error" — admins reading the audit can correlate with WARN logs and the user's eventual status to disambiguate. Acceptable for low-frequency events.
- **Trade-off**: backend password length check duplicates the frontend's zod check. Accepted because backend is the security boundary; frontend convenience does not weaken backend correctness.

## Migration Plan

No data migration. Deployment:

1. Deploy backend (`MarkUsed` `RowsAffected`, `SetupPassword` length check, email-failure audit). Existing tokens unaffected. Existing users unaffected.
2. Update Caddyfile (`Referrer-Policy` header + access-log format change). Restart Caddy. Existing in-flight requests unaffected.
3. Frontend: no change.
4. Rollback: revert the deployment; no data needs to roll back.

## Open Questions

None.
