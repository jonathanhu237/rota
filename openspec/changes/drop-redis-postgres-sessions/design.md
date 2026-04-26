## Context

The session store is the only thing in this app currently using Redis. The decision to add Redis was reasonable when more uses were expected (caching, queues, rate limiting), but those never materialized. Today Redis is a single-purpose stateful service whose load fits comfortably inside Postgres. This change removes Redis as a project dependency, replaces the session store with a Postgres-backed equivalent, and preserves every observable behavior of the auth flow.

## Goals / Non-Goals

**Goals:**

- Replace the Redis-backed `session.Store` with a Postgres-backed equivalent.
- Preserve the public auth API exactly: cookie shape, TTL, sliding refresh, disable-terminates, password-reset-terminates.
- Drop the Redis service from dev / prod compose, the Redis env vars from config, and the `go-redis` dependency from `go.mod`.
- Implement bounded-growth cleanup of expired session rows.

**Non-Goals:**

- Any new auth feature (admin session listing, multi-device tracking, etc.).
- Caching, queueing, or any non-session use of an in-memory store.
- Live migration of in-flight Redis sessions (no production data).
- Cross-region replication.

## Decisions

### D-1. Schema

```sql
CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX sessions_user_id_idx     ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx  ON sessions (expires_at);
```

`id` reuses the existing 32-byte hex random — same generator (`crypto/rand` → hex), same length, same semantics. Storing it as `TEXT PRIMARY KEY` is fine; collisions are 1-in-2^256.

`ON DELETE CASCADE` from `users` is a behavioral upgrade: today's Redis store has no FK and disabled-then-deleted-user paths could leave orphan keys. With a CASCADE FK, hard-deleting a user (rare; admin path) automatically cleans up their sessions. The disable-but-not-delete case keeps using `DeleteUserSessions(user_id)`.

`expires_at` is the absolute expiry time computed at create / refresh as `NOW() + SESSION_EXPIRES_HOURS`.

### D-2. Repository placement

**Decision:** put the implementation in `backend/internal/repository/session.go`. The thin `backend/internal/session/` package keeps only the `ErrSessionNotFound` sentinel (so existing imports continue to work) — or alternatively the sentinel moves to `model/session.go` and the `session/` package is deleted.

**Choice:** delete the `session/` package and move the sentinel to `repository/session.go` as `ErrSessionNotFound` exported there. Callers (`service/auth.go`, `handler/auth.go`) update their imports from `session.ErrSessionNotFound` → `repository.ErrSessionNotFound`. This is consistent with how the project already names DB-touching errors (`repository.ErrUserNotFound` etc).

**Alternative considered:** keep the `session/` package as a thin sentinel-only shim. Rejected — pure clutter; the package adds nothing once the Redis impl is gone.

### D-3. SessionRepository surface

```go
type SessionRepository struct {
    db *sql.DB
    expires time.Duration
}

func NewSessionRepository(db *sql.DB, expires time.Duration) *SessionRepository

func (r *SessionRepository) Create(ctx context.Context, userID int64) (string, int64, error)
// generates id, INSERTs row with expires_at = NOW() + expires
// returns (id, expiresInSeconds, nil)

func (r *SessionRepository) Get(ctx context.Context, sessionID string) (int64, error)
// SELECT user_id FROM sessions WHERE id = $1 AND expires_at > NOW()
// not-found → ErrSessionNotFound

func (r *SessionRepository) Refresh(ctx context.Context, sessionID string) (int64, error)
// UPDATE sessions SET expires_at = NOW() + $expires WHERE id = $1
//   AND expires_at > NOW() RETURNING true
// not-found / expired → ErrSessionNotFound; on success returns expiresInSeconds

func (r *SessionRepository) Delete(ctx context.Context, sessionID string) error
// DELETE FROM sessions WHERE id = $1

func (r *SessionRepository) DeleteUserSessions(ctx context.Context, userID int64) error
// DELETE FROM sessions WHERE user_id = $1
```

The shape mirrors today's `*session.Store` exactly, so service-layer interfaces don't need to change.

### D-4. Cleanup of expired rows

**Decision:** lazy filtering on read + periodic background sweep.

- **Lazy:** every `Get` and `Refresh` filters by `expires_at > NOW()`. An expired row is invisible to auth flow. No risk of accidental "session resurrection".
- **Periodic sweep:** a goroutine started by `cmd/server/main.go` runs `DELETE FROM sessions WHERE expires_at < NOW() - INTERVAL '1 day'` every 6 hours. The 1-day grace is purely a hygiene buffer — it keeps the table small without trying to delete-the-instant-something-expires (avoids hot lock contention on the `expires_at` index).

```go
// In main.go after building dependencies:
go func() {
    ticker := time.NewTicker(6 * time.Hour)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            _, _ = db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < NOW() - INTERVAL '1 day'`)
        }
    }
}()
```

Errors are logged, not surfaced — the sweep is best-effort. `ON DELETE CASCADE` in the FK already covers user-deletion cleanup.

**Alternative considered:**

- *No sweep, table grows forever*: at 100 active users with 2-week TTL, table size is bounded ~tens of rows of expired data per day, but accumulates. Sweep is cheap insurance.
- *Sweep on every request*: writes contention; pointless given the lazy filter on read.
- *External cron / pg_cron extension*: adds infra; goroutine is fine for a single-instance service.

### D-5. Sliding TTL on Authenticate

Today's Redis flow is `Get` then `Refresh` as separate calls. The Postgres implementation preserves that five-method interface so service-layer construction and tests stay mechanical: `Get` filters `expires_at > NOW()` and returns the user id, then `Refresh` performs a single `UPDATE … RETURNING` to extend the expiry and return the new TTL. This accepts two lightweight indexed statements per authenticated request, which is well under any UX-relevant threshold for the expected scale.

**Optimization considered:** collapse Get + Refresh into one `UPDATE … RETURNING user_id` call. Rejected for this change because the explicit goal is a storage-backend swap with the existing session API shape preserved. If authenticated-request latency ever matters, a follow-up can add a `RefreshAndGet` method and update `Authenticate` around that narrower performance change.

### D-6. Removed config

Remove from `Config` struct + tests + `.env.example`:

- `REDIS_HOST`
- `REDIS_PORT`
- `REDIS_PASSWORD`
- `REDIS_DB`

Remove from `docker-compose.yml`:
- service `redis`
- volume `redisdata`

Remove from `docker-compose.prod.yml`:
- service `redis`
- volume `redisdata`
- `REDIS_HOST: redis` env on the migrate / backend services
- `redis:` entry in `depends_on:` blocks

Remove from `backend/go.mod`:
- `github.com/redis/go-redis/v9` direct require (and any indirect lines after `go mod tidy`).

Remove from `README.md`:
- "Set local values for Postgres, Redis, ..." → "Set local values for Postgres, ..."
- "Start Postgres and Redis:" → "Start Postgres:"
- Prod stack description "Postgres, Redis, …" → "Postgres, …"
- The four `REDIS_*` rows in the env-var table.

Remove from `openspec/config.yaml`:
- "Go backend + PostgreSQL + Redis" → "Go backend + PostgreSQL"

### D-7. Migration strategy

Single migration file: `migrations/000NN_create_sessions_table.sql` (next sequential number).

```sql
-- +goose Up
-- +goose StatementBegin

CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX sessions_user_id_idx     ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx  ON sessions (expires_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS sessions_expires_at_idx;
DROP INDEX IF EXISTS sessions_user_id_idx;
DROP TABLE IF EXISTS sessions;

-- +goose StatementEnd
```

No data migration. CI's `migrations-roundtrip` exercises Up/Down.

### D-8. Test impact

**Unit / service-layer tests:** today's `authSessionStoreMock` matches the same five-method interface; only the embedded Go type name changes. Most tests untouched.

**Integration / repository tests:** new `backend/internal/repository/session_db_test.go` mirroring how `user_db_test.go` tests its repo: shared Postgres via `openIntegrationDB(t)`, BEFORE-EACH cleanup, table-driven cases for Create / Get / Refresh / Delete / DeleteUserSessions / expired-row invisibility / FK cascade on user delete.

**End-to-end tests:** if any test today spins up a real Redis container (we should grep for `dockertest.Run.*redis` or similar), drop that container.

### D-9. Rollback / recovery

If the change ships and we discover a problem post-merge:

- Migration is reversible (`make migrate-down` drops the table).
- Code revert via `git revert <merge-commit>` restores the Redis impl.
- No data lost in either direction (sessions are ephemeral; users re-login).

## Risks / Trade-offs

- **Risk:** authenticated request latency jumps from ~0.5ms (Redis) to ~2-3ms (Postgres × 2 calls; or ~1.5ms after the D-5 single-call optimization). → Mitigation: small enough to be invisible at this scale. If/when latency matters, add an in-process LRU cache in front (no infrastructure cost).
- **Risk:** `UPDATE … RETURNING` on every authenticated request creates write load on the sessions table. → Mitigation: 100 sessions × ~10 reqs/min average = ~17 writes/min total. Trivial.
- **Risk:** the goroutine-based cleanup tickets only run if the backend process is up. If it crashes / restarts mid-day, expired rows accumulate until next sweep. → Mitigation: lazy filter on read still works — auth never resurrects expired sessions. Worst case: a few thousand stale rows for a few hours.
- **Trade-off:** single point of failure becomes Postgres only. → That was already true (Postgres being down means no user creation, no scheduling, etc). Removing Redis just removes a *second* SPoF, not adds one.

## Migration Plan

Single shipping unit on `change/drop-redis-postgres-sessions`. Deployed as one atomic merge to `main`. After merge:

1. `make migrate-up` adds the `sessions` table (production).
2. The new backend image starts up, doesn't connect to Redis (the env var isn't read anymore).
3. `docker compose down redis` stops the Redis container; volume can be wiped at leisure.
4. Existing user browsers re-login on next visit (their old `session_id` cookies map to empty Redis → now they map to no row in `sessions` either → 401, log in again).

No staged rollout needed; the auth surface contract is identical from the user's perspective.

## Open Questions

None.
