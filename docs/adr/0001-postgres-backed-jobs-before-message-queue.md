# ADR 0001: Prefer Postgres-backed jobs before introducing a message queue

Date: 2026-05-08

Status: Accepted

## Context

Rota is currently a Go monolith backed by Postgres. The application already uses
a transactional email outbox: producer services enqueue email intents in the
same database transaction as the business write, and a background worker claims
pending rows with `FOR UPDATE SKIP LOCKED`, applies retries with backoff, and
marks rows as sent or failed.

This pattern solves the important consistency problem for request-path side
effects: the request writes one durable system, Postgres, and the asynchronous
worker handles delivery after commit. Introducing a broker directly from request
handlers would reintroduce a database-plus-message-queue dual-write problem
unless it is still paired with a transactional outbox or CDC relay.

The team discussed whether long-running asynchronous work should imply adding a
message queue. The conclusion is that long asynchronous work requires a durable
asynchronous execution mechanism, but not necessarily an external message queue.

## Decision

Do not introduce RabbitMQ, Kafka, NATS, Redis Streams, or another external
message queue for the current application shape.

For near-term asynchronous work, prefer a Postgres-backed jobs/outbox mechanism:

- Store job or event intent in Postgres in the same transaction as the business
  state that created it.
- Process pending rows with a worker using leases, retries, terminal failure
  states, and idempotent handlers.
- Generalize the existing email outbox only when more task types need the same
  machinery; do not create one bespoke worker per feature.
- Keep message payloads typed and versionable.
- Treat handlers as at-least-once: they must be safe under retry or duplicate
  processing.

If a broker is introduced later, it should normally sit behind an outbox relay
or CDC pipeline:

```text
business transaction -> outbox_events table -> relay or CDC -> message queue -> consumers
```

The broker is not a replacement for the outbox; it is a distribution mechanism
after the durable local write.

## Why not a message queue now?

An external message queue would add operational and implementation surface area:

- broker provisioning, local development, monitoring, backups, and incident
  handling
- producer and consumer libraries
- payload schema/version management
- retry and dead-letter behavior
- idempotency and poison-message handling
- database/MQ consistency via outbox or CDC relay

For a single Go service with Postgres and modest asynchronous volume, these
costs are not justified by the current requirements. Postgres-backed workers are
also easier to inspect and debug with the existing database tooling.

## When to reconsider

Revisit this decision when one or more of these become true:

- Multiple independently deployed services need to consume the same domain
  event.
- Worker capacity must scale independently from the API process.
- Event throughput or latency requirements exceed comfortable Postgres polling.
- The system needs fanout, consumer groups, replay, long event retention, or
  broker-native routing.
- Cross-language or cross-team integrations need a shared event boundary.
- The team already operates broker infrastructure as a standard platform
  primitive.

Examples that still fit Postgres-backed jobs first:

- sending transactional email
- batch notifications
- schedule export generation
- auto-assignment or optimization jobs
- cleanup tasks

Examples that may justify a broker later:

- `publication.published` consumed by notification, analytics, search indexing,
  and external integration services
- a separately deployed optimization worker pool with independent autoscaling
- near-real-time event streaming into a data platform

## Consequences

Near-term asynchronous features should build on a shared job/outbox abstraction
instead of introducing a broker-specific API. A reasonable next step is:

```text
email_outbox
  -> generic jobs or outbox_events table
  -> handler registry by type
  -> shared claim/retry/failure/observability behavior
```

This keeps the current consistency model simple while leaving a clean migration
path to a broker if the application later grows beyond a single-service
background job model.
