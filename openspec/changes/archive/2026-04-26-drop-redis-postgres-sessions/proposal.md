## Why

Redis exists in this project for exactly one purpose: storing the K-V `session:<id> → user_id` map for HTTP authentication. No caching, no queue, no pub/sub. For a single-monolith internal tool serving ~100 people, Redis is operational dead weight — it adds a second stateful component to back up, monitor, and recover, in exchange for sub-millisecond reads on a key set that maxes out around 100 active sessions. Postgres handles that workload trivially (indexed primary-key lookup is ~1ms, well under any UX-relevant threshold), and consolidating to a single stateful dependency simplifies dev setup, CI containers, prod compose stack, and disaster recovery.

This change moves the session store from Redis to Postgres. The session API surface (`Store.Create`, `Get`, `Refresh`, `Delete`, `DeleteUserSessions`) is preserved; only the storage backend changes. Auth behavior is identical from the user's perspective — same cookie shape, same TTL, same sliding refresh, same disable-clears-sessions semantics.

There is no production data; existing dev / CI sessions in Redis are abandoned at switchover and reissued by next login. Acceptable.

## What Changes

- **New DB schema:** `sessions` table with `id TEXT PRIMARY KEY` (the 32-byte hex random id used today), `user_id BIGINT REFERENCES users(id) ON DELETE CASCADE`, `expires_at TIMESTAMPTZ NOT NULL`, `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`. Indexes: PK on `id` (Authenticate lookups); `user_id` (DeleteUserSessions); `expires_at` (lazy cleanup sweep).
- **New repository:** `backend/internal/repository/session.go` with `SessionRepository` implementing the existing `session.Store` API surface (or a renamed equivalent — see design).
- **`backend/internal/session/store.go` is removed**; the package's only export — `ErrSessionNotFound` — moves to `backend/internal/model/session.go` (or stays in a thin `session/` package without the Redis impl). Service-layer interfaces that today reference `*session.Store` change to either an interface or the new repository type.
- **Wiring in `backend/cmd/server/main.go`:** drop the Redis client construction; pass the new `SessionRepository` to `AuthService` and `UserService` in place of the old `*session.Store`.
- **Config drops Redis fields:** `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_DB` are removed from `Config` struct, `config_test.go`, and `.env.example`.
- **Docker compose:** the `redis` service and `redisdata` volume are removed from both `docker-compose.yml` (dev) and `docker-compose.prod.yml` (prod). `depends_on: redis` references are removed; `REDIS_HOST: redis` envs are removed.
- **Go module:** `github.com/redis/go-redis/v9` is removed from `backend/go.mod`; `go mod tidy` runs.
- **README:** the "Redis" mentions in setup, prod stack description, and env-var table are removed.
- **`openspec/config.yaml`:** the project description "Go backend + PostgreSQL + Redis" becomes "Go backend + PostgreSQL".
- **Auth spec:** several requirements that today say "Redis-backed session" / "Redis key" are reworded to refer to the `sessions` table and SQL operations. The user-visible behavior (cookie, TTL, sliding refresh, disable-terminates) is unchanged; only the internal storage language changes.
- **Sliding TTL implementation:** preserved. Every `Authenticate` call first reads a still-valid session row and then updates `expires_at` via `UPDATE sessions SET expires_at = $1 WHERE id = $2 AND expires_at > NOW()`. This keeps the existing five-method session interface intact while moving storage to Postgres.
- **Expired-session cleanup:** lazy on read (the SELECT filters `expires_at > NOW()`) plus a periodic sweep — see design D-4. No background cron in scope; sweep runs on a goroutine timer inside the backend process.
- **Tests:** existing service-layer tests use a mock implementing the session-store interface; those keep working with a tiny shape change. Repository integration tests (new file) cover real Postgres reads/writes/expiry.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `auth`: replaces the Redis-backed session storage with a Postgres-backed `sessions` table. Updates several requirements that mention Redis in the wire / storage description; user-visible session behavior (cookie, TTL, sliding refresh, disable-terminates) is unchanged.

## Non-goals

- **Adding any new auth feature.** Pure storage swap.
- **Changing session ID format, cookie shape, or TTL semantics.** Same `session_id` 32-byte hex in cookie; same `SESSION_EXPIRES_HOURS`; same sliding refresh on Authenticate; same `HttpOnly / SameSite=Lax / Secure-when-TLS` cookie attributes.
- **Migrating live Redis sessions.** No production data; switchover discards in-flight Redis state. Dev-environment users log in again.
- **Revisiting rate limiting.** Rate limiting is currently a non-goal in `auth` (no requirement) and stays out of scope.
- **Adding caching for any other purpose.** If/when caching is wanted later, a future change can re-introduce Redis (or use Postgres `UNLOGGED` tables / in-process LRU). Not now.
- **Admin endpoints to view active sessions.** Could be done with the new schema (it's just a table) but not in scope.
- **Multi-region session replication.** Not relevant for this app.

## Impact

- **Backend code:**
  - New: `migrations/000NN_create_sessions_table.sql` (Up + Down).
  - New: `backend/internal/repository/session.go` with the Postgres-backed implementation.
  - Modified: `backend/internal/session/store.go` is deleted; the small `session/` package either disappears or is reduced to the `ErrSessionNotFound` sentinel (depending on how callers import it). Design.md picks an exact disposition.
  - Modified: `backend/cmd/server/main.go` wiring.
  - Modified: `backend/internal/config/config.go` + `config_test.go`.
  - Modified: `backend/internal/handler/auth.go` and `backend/internal/service/auth.go` / `user.go` to take the new repository through the existing interface (likely no behavioral change — just type swap).
  - Modified: `backend/go.mod` / `go.sum` after `go mod tidy`.
- **Infrastructure / config:**
  - Modified: `.env.example` (drop `REDIS_*` block).
  - Modified: `docker-compose.yml` (drop `redis` service + `redisdata` volume).
  - Modified: `docker-compose.prod.yml` (same plus `depends_on` and env injection).
- **Docs:**
  - Modified: `README.md` (Redis mentions in setup + env table + prod stack description).
  - Modified: `openspec/config.yaml` (project description).
- **Spec:**
  - Modified `auth` capability: 5 requirements re-worded to use `sessions` table instead of "Redis key".
- **Tests:**
  - New repository integration tests (`backend/internal/repository/session_db_test.go`).
  - Existing service / handler tests updated to use a Postgres-backed fake or sql-mock — tiny mechanical changes since the interface is preserved.
  - Existing tests that depended on a real Redis container (if any) drop the dependency.
- **CI:** if CI's `backend-test` workflow specifies a Redis service, it gets removed. Docker-build job is unaffected.
- **No third-party dependencies added; one removed (`github.com/redis/go-redis/v9`).**

## Risks / safeguards

- **Risk:** sliding TTL implementation requires UPDATE on every authenticated request. **Mitigation:** at this scale (~100 active sessions, request rate at hundreds-per-second peak) the write rate is negligible. PK update is ~1ms. The existing tests for "Refresh extends TTL" carry over to verify the SQL path.
- **Risk:** lazy cleanup of expired rows lets the `sessions` table grow if no sweep runs. **Mitigation:** add a background sweep goroutine in the backend with a 1-hour interval running `DELETE FROM sessions WHERE expires_at < NOW()` (see design D-4). Bounded growth (sessions are per-active-user; old expired rows are short-lived).
- **Risk:** in-tx behavior of `DeleteUserSessions` differs subtly. **Mitigation:** existing requirement "Admin disable terminates live sessions" stays exactly the same in user-visible behavior; the SQL is `DELETE FROM sessions WHERE user_id = $1` which is unconditional and atomic, simpler than the SCAN+MGET+DEL of the Redis implementation.
- **Risk:** drop of Redis dependency cascades through `go.sum` / `Dockerfile`. **Mitigation:** the apply runs `go mod tidy` and the existing CI `backend-test` + `docker-build` jobs will catch any leftover import or container reference.
- **Risk:** missed Redis reference in some test fixture. **Mitigation:** final task is `grep -RIn redis backend openspec README.md docker-compose*.yml .env.example` returning zero hits outside archive history.
