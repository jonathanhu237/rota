## 1. Database migration

- [x] 1.1 Add `migrations/00013_create_sessions_table.sql` (next sequential number) per design D-7. Up creates the `sessions` table with `id TEXT PRIMARY KEY`, `user_id BIGINT REFERENCES users(id) ON DELETE CASCADE`, `expires_at TIMESTAMPTZ NOT NULL`, `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, plus indexes on `user_id` and `expires_at`. Down drops the indexes and table. Verify: `make migrate-down && make migrate-up && make migrate-down` clean; CI's `migrations-roundtrip` exercises both directions.

## 2. Backend repository — new Postgres session store

- [x] 2.1 Add `backend/internal/repository/session.go` with `SessionRepository` and the five methods per design D-3:
    - `NewSessionRepository(db *sql.DB, expires time.Duration) *SessionRepository`
    - `Create(ctx, userID) (sessionID string, expiresInSeconds int64, error)` — generates 32-byte hex via `crypto/rand`, INSERTs row, returns id and remaining seconds.
    - `Get(ctx, sessionID) (userID int64, error)` — `SELECT user_id FROM sessions WHERE id = $1 AND expires_at > NOW()`; not-found / expired → `ErrSessionNotFound`.
    - `Refresh(ctx, sessionID) (expiresInSeconds int64, error)` — `UPDATE sessions SET expires_at = NOW() + $1 WHERE id = $2 AND expires_at > NOW() RETURNING (extract(epoch from expires_at) - extract(epoch from NOW()))::bigint`; not-found → `ErrSessionNotFound`.
    - `Delete(ctx, sessionID) error` — `DELETE FROM sessions WHERE id = $1`.
    - `DeleteUserSessions(ctx, userID) error` — `DELETE FROM sessions WHERE user_id = $1`.
  Define `var ErrSessionNotFound = errors.New("session not found")` in this file (exported, replacing today's `session.ErrSessionNotFound`). Verify: `cd backend && go build ./...`.

- [x] 2.2 Add `backend/internal/repository/session_db_test.go` with `//go:build integration` covering: Create / Get round-trip, Refresh extends expiry, Delete removes row, DeleteUserSessions removes all of one user's rows but leaves others, lazy filtering excludes already-expired rows from Get/Refresh, FK CASCADE removes sessions when a user row is deleted. Verify: `cd backend && go test -tags=integration ./internal/repository -run Session -count=1`.

## 3. Backend wiring + dead-code removal

- [x] 3.1 Update `backend/internal/handler/auth.go`: imports of `session` package → `repository` package; the `session.ErrSessionNotFound` reference becomes `repository.ErrSessionNotFound`. Verify: `go build ./...`.

- [x] 3.2 Update `backend/internal/service/auth.go` and `backend/internal/service/user.go`: the type accepted by these services for "the session store" stays as the same five-method interface; only the concrete type at construction time changes. Update any test mocks accordingly. Verify: `go build ./...` and `go test ./internal/service -count=1`.

- [x] 3.3 Update `backend/cmd/server/main.go`:
    - Remove the Redis client construction (`redis.NewClient(...)`).
    - Remove the `session.NewStore(client, ...)` call.
    - Add `repository.NewSessionRepository(db, sessionExpires)` construction; pass it to `AuthService` and `UserService` in place of the old store.
    - Add the periodic cleanup goroutine per D-4: 6-hour ticker running `DELETE FROM sessions WHERE expires_at < NOW() - INTERVAL '1 day'`. Use the existing `ctx` for cancellation on shutdown.
  Verify: `go build ./...`; `make run-backend` starts cleanly without a Redis container running.

- [x] 3.4 Delete `backend/internal/session/store.go` and the `backend/internal/session/` package. The error sentinel has moved to `repository` (per task 2.1); no other code remains in the package. Verify: no references to `internal/session` survive in any Go file (`grep -rln "internal/session" backend --include="*.go"` returns nothing).

- [x] 3.5 Run `cd backend && go mod tidy` to drop `github.com/redis/go-redis/v9` from `go.mod` and update `go.sum`. Verify: `git diff backend/go.mod` shows the redis line removed; `go build ./...` clean.

## 4. Config cleanup

- [x] 4.1 Update `backend/internal/config/config.go`: remove the four `REDIS_*` fields from the `Config` struct (`RedisHost`, `RedisPort`, `RedisPassword`, `RedisDB`). Verify: `go build ./...`.

- [x] 4.2 Update `backend/internal/config/config_test.go`: remove all assertions referencing `REDIS_*` env vars. Verify: `go test ./internal/config -count=1`.

- [x] 4.3 Update `.env.example`: remove the lines `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_DB`. Verify: `make seed SCENARIO=basic` runs cleanly with the trimmed `.env`.

## 5. Infrastructure cleanup

- [x] 5.1 Update `docker-compose.yml`: remove the `redis` service block and the `redisdata` volume. Verify: `docker compose up -d` brings up only Postgres; `docker compose ps` shows one container.

- [x] 5.2 Update `docker-compose.prod.yml`: remove the `redis` service, the `redisdata` volume, every `REDIS_HOST: redis` env injection on the `migrate` and `backend` services, and the `redis:` entry in any `depends_on:` block. Verify: `docker compose -f docker-compose.prod.yml config` parses without errors and references no Redis.

## 6. Documentation cleanup

- [x] 6.1 Update `README.md`:
    - In the local-development setup steps (around lines 15–16): replace "Set local values for Postgres, Redis, bootstrap admin..." with "Set local values for Postgres, bootstrap admin..." and replace "Start Postgres and Redis:" with "Start Postgres:".
    - In the production stack description (around line 75): "The production stack includes Postgres, Redis, …" becomes "The production stack includes Postgres, …".
    - Remove the four `REDIS_*` rows from the env-var table.
  Verify: render the README locally; no stray Redis mentions.

- [x] 6.2 Update `openspec/config.yaml`: change "Go backend + PostgreSQL + Redis" to "Go backend + PostgreSQL". Verify: `openspec validate drop-redis-postgres-sessions` continues to pass; the change-context (which the propose flow loads from this file) reads sensibly without the Redis reference.

- [x] 6.3 Update the auth capability spec's Purpose paragraph (top of `openspec/specs/auth/spec.md` line 5) directly during apply: replace "against a Redis-backed session" with "against a Postgres-backed session". This is a direct edit to main spec rather than a delta, because the OpenSpec delta format does not address the Purpose section. Verify: `grep -i redis openspec/specs/auth/spec.md` returns zero hits.

## 7. Final verification

- [x] 7.1 Backend clean: `cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./... && govulncheck ./...` — every step exits 0.
- [x] 7.2 Frontend untouched: `cd frontend && pnpm lint && pnpm test && pnpm build` (regression check; this change touches no frontend code).
- [x] 7.3 Migrations roundtrip: `make migrate-down && make migrate-up && make migrate-down && make migrate-up`.
- [x] 7.4 Smoke test (manual):
    - (a) Stop any running Redis container: `docker rm -f rota-redis-1 || true`.
    - (b) `docker compose up -d` — only Postgres comes up; no Redis container exists.
    - (c) `make migrate-up && make seed SCENARIO=full && make run-backend` (in another terminal) and `make run-frontend`.
    - (d) Login as `admin@example.com` / `pa55word`; cookie set; navigate around; confirm sessions work.
    - (e) `psql -c "SELECT id, user_id, expires_at FROM sessions"` — the row exists.
    - (f) Logout; re-query — the row is gone.
    - (g) Disable a user as admin; re-query — that user's session row is gone.
    - (h) Stop the backend, restart it, confirm the existing session cookie still authenticates (TTL persisted in DB across restart).
- [x] 7.5 Confirm zero Redis references in source: `grep -RIn "redis\|Redis\|REDIS" backend openspec README.md docker-compose*.yml .env.example | grep -v "openspec/changes/archive" | grep -v "openspec/changes/drop-redis-postgres-sessions"` returns no hits. The active change artifacts are excluded because the proposal, design, and task rationale intentionally mention the dependency being removed.
- [x] 7.6 Confirm CI-equivalent checks are green locally before handoff: `backend-test`, `frontend-test`, `migrations-roundtrip`, `docker-build`, `govulncheck`. If the CI workflow file references a Redis service in the `backend-test` job, remove it as part of this task (otherwise the job will keep waiting on a non-existent dependency). Remote branch CI is confirmed after the branch is pushed.
