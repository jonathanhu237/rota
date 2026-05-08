# ADR 0003: Defer Redis until there is a measured runtime need

Date: 2026-05-08

Status: Accepted

## Context

The team considered adding Redis to the Rota stack for several possible uses:

- read caching before Postgres reads
- HTTP session storage with key TTLs
- asynchronous email or background job processing
- shared authentication rate limiting

Rota is currently a Go monolith with Postgres as its only durable runtime data
dependency. Sessions are stored in the `sessions` table, authenticated requests
refresh session expiry in Postgres, and security-sensitive flows can actively
revoke all or some sessions for a user. Transactional email uses a Postgres
outbox, with email intents written in the same database transaction as the
business state that produced them.

Redis is useful infrastructure, but adding it speculatively would introduce a
second stateful runtime dependency to provision, monitor, back up, secure, and
recover without a clear current bottleneck.

## Decision

Do not introduce Redis as a runtime dependency now.

Keep Postgres as the only required stateful service for the current application
shape. Redis can be reconsidered later for a specific measured need, but it
should not be added as general-purpose architecture capacity.

The current scope decisions are:

- **Read caching:** defer. ADR 0002 defines the project policy: cache reads per
  endpoint or read model, not through a generic repository-wide cache layer.
- **Sessions:** keep Postgres-backed sessions. TTL-style natural expiration is
  already represented by `expires_at`; active revocation remains important for
  password changes, email confirmation, account disablement, and logout flows.
- **Asynchronous email/jobs:** keep the Postgres outbox. ADR 0001 defines the
  project policy: prefer Postgres-backed jobs before introducing Redis Streams
  or another message queue.
- **Authentication rate limiting:** keep in-process rate limiting while the API
  is a single backend instance. Redis-backed rate limiting is a reasonable first
  Redis use case if backend instances are horizontally scaled and limits must be
  shared across processes.

## Session requirements if Redis is revisited

If sessions move to Redis later, Redis must be treated as an authentication
dependency, not an optional cache.

The implementation must preserve the existing security semantics:

- session lookup and refresh fail closed if Redis is unavailable
- sessions expire automatically via Redis TTL
- password changes can revoke every other session for the user immediately
- email confirmation and account disablement can revoke all sessions for the
  user immediately
- logout can revoke the current session immediately

TTL alone is not enough. The design needs either per-user session indexes, such
as `user_sessions:{userID}`, or a session-version scheme that invalidates older
sessions when the user's security state changes.

## Why not Redis now?

Read caching is not a generic optimization. Stale reads are acceptable for some
low-risk views, but scheduling views often aggregate multiple business tables
and may affect user-visible decisions. Those caches need endpoint-level
freshness and invalidation contracts.

Session storage is not currently a measured bottleneck. Primary-key session
lookups and expiry refreshes in Postgres are simple and preserve active
revocation semantics without adding another required service.

Redis queues would improve task wake-up latency and worker distribution, but
they do not solve the harder consistency problem between business writes and
side effects. For email and near-term background work, the Postgres outbox keeps
the durable intent in the same transaction as the business write and is easier
to inspect with SQL.

Shared rate limiting is the clearest Redis use case, but it only matters once
there is more than one backend process enforcing the same logical limit.

## When to reconsider

Revisit Redis when one or more of these become true:

- the backend is horizontally scaled beyond one instance and authentication rate
  limits must be shared
- session lookup or refresh is measured as a meaningful database bottleneck
- a specific read endpoint has measured latency or throughput pressure plus a
  safe invalidation strategy
- asynchronous workloads exceed comfortable Postgres-backed polling or need
  independently scaled worker capacity
- the team already operates Redis as a standard, monitored platform dependency

## Consequences

The production and local development stacks stay simpler: Postgres remains the
only required stateful runtime service.

Future Redis work should be introduced through a focused OpenSpec change with a
specific use case, failure mode, and verification plan. Adding Redis to the
stack should come with updated configuration, compose files, health checks,
tests, and operational notes for the chosen use case.
