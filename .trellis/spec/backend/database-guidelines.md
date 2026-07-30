# Database Guidelines

## Stack and Ownership

The backend uses PostgreSQL through the standard `database/sql` package and
`github.com/lib/pq`; there is no ORM. SQL belongs in `backend/internal/repository`.
Schema changes are repository-root Goose migrations under `migrations/`.

## Query Patterns

- Use `QueryRowContext`, `QueryContext`, and `ExecContext` with the request
  context.
- Keep query text in a local `const query` and use positional parameters; never
  interpolate user data into SQL.
- Close rows immediately with `defer rows.Close()` and check `rows.Err()`.
- Return empty non-nil slices for collection results where the API expects
  arrays.
- Normalize occurrence dates through model helpers before queries and scans.
- Map `sql.ErrNoRows` and known PostgreSQL constraint names to stable sentinel
  errors where callers need to distinguish them.

See `backend/internal/repository/attendance.go` for scan loops and normalized
dates, and `backend/internal/repository/user.go` for constraint/error mapping.

## Transactions and Concurrency

Atomic multi-write business operations use repository transaction runners and
pass a `repository.DBTX`/`*sql.Tx` to transaction-scoped repositories. Always
roll back on early return and commit exactly once.

Use row locking (`FOR UPDATE`) when a decision depends on mutable database
state. Scheduling and shift-change operations must preserve the domain
invariants in `../domain/scheduling.md`, including retryable conflict mapping
and cascading invalidation.

Outbox messages that represent the same business action are enqueued inside
the business transaction. Do not commit the state change and enqueue email in
separate transactions.

## Migrations

- Name files with the next five-digit prefix and a descriptive snake-case
  suffix, for example `00021_add_attendance_tracking.sql`.
- Every migration contains Goose `Up` and `Down` sections.
- Use `make migrate-up`, `make migrate-down`, and `make migrate-status`.
- Add constraints and indexes in the migration that introduces the invariant.
- SQL changes require integration tests against PostgreSQL.
- Do not edit an already-applied migration to change production behavior; add a
  new migration.

## Naming

Tables and columns are plural/singular snake case following the existing
schema. Foreign keys use `<entity>_id`; timestamps use `_at`; boolean columns
read as predicates such as `is_admin` or `attendance_responsible`. Let
PostgreSQL constraint names be explicit when repository code maps them.

## Common Mistakes

- Forgetting `rows.Err()` after iteration.
- Comparing raw `time.Time` values without normalizing date-only fields.
- Adding a state-changing query outside the transaction that protects its
  corresponding read.
- Returning raw `pq` or `sql` errors when the service expects a domain error.
