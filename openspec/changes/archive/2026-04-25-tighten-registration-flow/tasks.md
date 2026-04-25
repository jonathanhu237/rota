## 1. Setup token strict single-use

- [x] 1.1 In `backend/internal/repository/setup_token.go`, modify `MarkUsed` (and any other helper that does `UPDATE user_setup_tokens SET used_at = $... WHERE used_at IS NULL`) to capture `result.RowsAffected()` and return the existing `ErrTokenUsed` sentinel when `affected == 0`. Verify: `cd backend && go build ./...`.
- [x] 1.2 Confirm the service layer (`backend/internal/service/auth.go` `SetupPassword`) propagates `ErrTokenUsed` to the handler unchanged. The handler already maps `ErrTokenUsed` → `TOKEN_USED` (410); verify by reading `handler/auth.go` — no change expected. Verify: `cd backend && go vet ./...`.
- [x] 1.3 Add a service unit test in `backend/internal/service/auth_test.go` that wires a stub repo whose `MarkUsed` returns `ErrTokenUsed` and asserts `SetupPassword` returns the same sentinel up to the caller. Verify: `cd backend && go test ./internal/service -run SetupPassword -count=1`.
- [x] 1.4 Add a repository integration test in `backend/internal/repository/setup_token_db_test.go`: insert a token, call `MarkUsed` twice, assert first call returns `nil` and second returns `ErrTokenUsed`. Verify: `POSTGRES_HOST=localhost POSTGRES_PORT=${POSTGRES_PORT:-5433} POSTGRES_USER=rota POSTGRES_PASSWORD=pa55word POSTGRES_DB=rota go test -tags=integration ./internal/repository -run SetupTokenMarkUsed -count=1`.
- [x] 1.5 Add a handler/integration test confirming HTTP 410 with body `{"error":{"code":"TOKEN_USED",...}}` for a second consumer. Either a service-level test stubbing `ErrTokenUsed`, or an end-to-end test driving two concurrent calls if practical. Verify: `cd backend && go test ./internal/handler -run TokenUsed -count=1`.

## 2. Backend password length floor

- [x] 2.1 In `backend/internal/service/auth.go` `SetupPassword`, before the bcrypt step, add `if utf8.RuneCountInString(password) < 8 { return ErrPasswordTooShort }`. Add the import. Confirm `ErrPasswordTooShort` already exists; if not, add it as a sentinel in `model/auth.go`. Verify: `cd backend && go build ./... && go vet ./...`.
- [x] 2.2 Confirm handler maps `ErrPasswordTooShort` → `PASSWORD_TOO_SHORT` (400). Read `handler/auth.go`; no change expected per existing spec. Verify: `cd backend && go test ./internal/handler -count=1`.
- [x] 2.3 Add service unit tests covering the four bullet conditions: `len = 0` rejected, `len = 7` rejected, `len = 8` accepted, multi-byte (e.g., `你好世界你好` = 6 runes) rejected. Use Chinese / emoji input to prove rune-count semantics, not byte-count. Verify: `cd backend && go test ./internal/service -run SetupPasswordLength -count=1`.

## 3. Email-failure audit + WARN log

- [x] 3.1 In `backend/internal/audit/audit.go`, add `ActionUserInvitationEmailFailed = "user.invitation.email_failed"`. Verify: `cd backend && go build ./...`.
- [x] 3.2 In `backend/internal/service/user.go` `CreateUser`, change the post-tx `s.setupFlows.sendInvitation(...)` call to capture the returned error. On error, emit `audit.Record` with the new action and metadata `{ email, error: err.Error() }`, plus `s.logger.Warn(...)`. Do NOT alter the function's success/failure return — admin still gets 201. Verify: `cd backend && go vet ./... && go build ./...`.
- [x] 3.3 Apply the same pattern in `backend/internal/service/user.go` `ResendInvitation`. Verify: same.
- [x] 3.4 Add a service test for `CreateUser` with an `emailerStub` whose `SendInvitation` returns a configurable error. Use `audittest.New()` to capture audit events; assert exactly one `user.invitation.email_failed` event is recorded with the expected metadata, AND that the user row was created (admin still got success). Verify: `cd backend && go test ./internal/service -run CreateUserEmailFailure -count=1`.
- [x] 3.5 Add a parallel test for `ResendInvitation`. Same verify path.

## 4. Verify ResendInvitation invalidates prior tokens (spec compliance)

- [x] 4.1 Read `backend/internal/service/setup.go` `issueToken`. Confirm it calls `InvalidateUnusedTokens(userID, purpose, now)` before `Create`. If yes, add a service test in `setup_test.go` that proves the order (only if missing). Verify: `cd backend && go test ./internal/service -run TokenIssuanceInvalidatesPrior -count=1`.
- [x] 4.2 If the previous step shows `issueToken` does NOT call `InvalidateUnusedTokens` first, fix it. Add the call inside `issueToken`'s tx scope. Add the missing test. Verify: same as 4.1.

## 5. Caddy access log scrubbing

- [x] 5.1 Inspect `frontend/Caddyfile` (or wherever Caddy config lives) for the access-log block. Identify the current log format: does it use `request>uri` (includes query string) or `request>uri_path` (excludes)? Verify: `docker compose -f docker-compose.prod.yml up -d caddy && curl -s http://localhost/api/auth/setup-token?token=AAA > /dev/null && docker compose -f docker-compose.prod.yml exec -T caddy cat /data/access.log | tail -1` — observe whether `?token=AAA` appears.
- [x] 5.2 If the log includes the query string, change the format to use `request>uri_path` (or equivalent). Restart caddy. Re-run the same curl + tail check; assert the substring `?token=` does NOT appear in the new log line. Verify: same command shows the change took effect.
- [x] 5.3 Update `frontend/Caddyfile` accordingly and add a comment line `# uri_path keeps query strings (which may carry setup tokens) out of access logs`.

## 6. Caddy Referrer-Policy header

- [x] 6.1 Inspect the `header` block in `frontend/Caddyfile`. Confirm whether `Referrer-Policy "no-referrer"` is present. Verify: `curl -sI http://localhost/api/health | grep -i referrer-policy`.
- [x] 6.2 If absent, add `Referrer-Policy "no-referrer"` to the existing header block. Restart caddy. Verify: same `curl -sI` command shows the header in the response. Document with a comment.

## 7. Frontend i18n (touch-up only)

- [x] 7.1 Spot-check `frontend/src/i18n/locales/en.json` and `zh.json` for existing copy under `auth.errors.TOKEN_USED` and `auth.errors.PASSWORD_TOO_SHORT`. Both should already exist (the codes were specced). If either is missing, add suggested copy: en TOKEN_USED → "This setup link has already been used. Please contact an admin for a new invitation."; zh → "该设置链接已经使用过了，请联系管理员重新发送邀请。"; en PASSWORD_TOO_SHORT → "Password must be at least 8 characters."; zh → "密码至少需要 8 个字符。" Verify: `cd frontend && pnpm build && python3 -c "import json;a,b=[set(__import__('json').load(open(f'frontend/src/i18n/locales/{l}.json'))) for l in ('en','zh')];print(a^b)"` — empty set expected.

## 8. Final verification

- [x] 8.1 `cd backend && go build ./... && go vet ./... && go test ./... && govulncheck ./...` — all clean.
- [x] 8.2 `POSTGRES_HOST=localhost POSTGRES_PORT=${POSTGRES_PORT:-5433} POSTGRES_USER=rota POSTGRES_PASSWORD=pa55word POSTGRES_DB=rota go test -tags=integration ./... -count=1` — all clean.
- [x] 8.3 `cd frontend && pnpm lint && pnpm test && pnpm build` — all clean.
- [x] 8.4 Smoke test against `docker compose -f docker-compose.prod.yml up`: (a) admin creates user; manually break SMTP (point `EMAIL_HOST` at an unreachable host or disable the emailer); confirm admin still gets 201 and `audit_logs` has a row with action `user.invitation.email_failed`. (b) Use `curl` to `POST /auth/setup-password` with `password=1`; confirm 400 `PASSWORD_TOO_SHORT`. (c) Use two parallel `curl` calls with the same setup token; confirm one returns 204 and the other 410 `TOKEN_USED`. (d) `tail -1` Caddy access log after a `setup-token` request; confirm `?token=` is absent. (e) `curl -sI` any endpoint; confirm `Referrer-Policy: no-referrer` header is present.
