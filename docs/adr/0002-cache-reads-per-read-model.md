# ADR 0002: Cache reads per read model, not through a generic repository layer

Date: 2026-05-08

Status: Accepted

## Context

Rota may reintroduce Redis for ephemeral infrastructure state such as HTTP
sessions and authentication rate limits. Redis can also be used as a read cache,
but read caching has different correctness risks from short-lived state.

The application has several scheduling views whose responses are assembled from
multiple tables and whose values affect user decisions: assignment boards,
current publications, rosters, shift-change requests, and leave-dependent views.
For these reads, stale cache entries can show an outdated schedule or request
state even though Postgres has already accepted a newer business write.

The team discussed the common cache-aside pattern:

```text
read:  Redis miss -> read Postgres -> populate Redis
write: write Postgres -> invalidate or refresh Redis
```

This pattern is useful, but only when stale-read tolerance and invalidation rules
are explicit for the specific endpoint or read model being cached.

## Decision

Do not introduce a generic repository-wide Redis cache layer.

Read caching must be evaluated per endpoint or per read model. Each cached read
must define:

- the cache key shape
- the stale-read tolerance
- the invalidation trigger after writes
- the fallback behavior when Redis is unavailable
- whether TTL is only a safety net or the primary freshness mechanism
- tests for the success path and at least one stale/invalidation error path

Writes and write-side validation remain Postgres-authoritative. Cached data must
not be used as the source of truth for deciding whether a scheduling write is
valid.

Low-risk cache-aside candidates can use short TTLs and write-after-invalidate
behavior:

- branding and application metadata
- mostly static position lists
- template list or summary views where short-lived stale data is acceptable

Higher-risk scheduling reads require explicit invalidation or versioned cache
keys:

- assignment board views
- current publication and roster views
- shift-change state views
- leave-dependent roster or availability views

TTL-only caching is not acceptable for these higher-risk reads unless the
product requirement explicitly accepts stale scheduling data for the TTL window.

## Why not a generic cache layer?

A transparent repository cache hides the important part of caching: invalidation.
The same repository method can be used by endpoints with different permission
scopes, response shapes, freshness requirements, and downstream write behavior.
Caching that method globally makes stale reads hard to reason about and hard to
test.

Generic caching is especially risky for aggregate scheduling views because a
single response may depend on users, positions, templates, publications,
assignments, shift-change requests, leave records, and publication state. Any
write touching one of those dependencies must invalidate the correct aggregate
keys.

## When to add a read cache

Add read caching only after there is evidence that the read path needs it:

- measured query latency or throughput pressure
- repeated high-volume reads of the same data
- an endpoint with a clear freshness tolerance
- an invalidation strategy that can be implemented and tested without broad
  coupling

Prefer query tuning, indexing, pagination, and response shaping before caching
core scheduling aggregates.

## Consequences

Redis being present in the stack does not imply that reads are cached by default.

Future OpenSpec changes that add read caching should describe the endpoint-level
cache contract in their design artifacts and update this ADR only if the project
policy changes. The first read cache should be a low-risk candidate so the team
can establish Redis wiring, observability, and invalidation conventions before
touching core scheduling reads.
