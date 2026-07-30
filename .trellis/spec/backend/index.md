# Backend Development Guidelines

The backend is a Go HTTP service backed by PostgreSQL. Product features normally
flow through `handler -> service -> repository -> model/database`. Keep transport,
business rules, and persistence concerns in those layers.

## Guidelines Index

| Guide | Description |
|---|---|
| [Directory Structure](./directory-structure.md) | Package boundaries and feature placement |
| [Database Guidelines](./database-guidelines.md) | `database/sql`, transactions, queries, and Goose migrations |
| [Error Handling](./error-handling.md) | Sentinel errors and HTTP error contracts |
| [Logging Guidelines](./logging-guidelines.md) | `log/slog`, audit records, and sensitive data |
| [Quality Guidelines](./quality-guidelines.md) | Tests, checks, and forbidden patterns |

## Pre-Development Checklist

Read the files that match the change, plus `quality-guidelines.md` every time:

- HTTP/API work: `directory-structure.md` and `error-handling.md`.
- Service or domain work: `directory-structure.md`, `error-handling.md`, and the
  matching file under `../domain/`.
- SQL, transaction, or migration work: `database-guidelines.md`.
- Background jobs, email, security, or audit work: `logging-guidelines.md`.
- Cross-layer work: also read `../guides/cross-layer-thinking-guide.md`.

Before changing a material behavior, confirm that a Trellis task is active and
that the relevant domain contract has been loaded.

## Core Invariants

- Services receive clocks or clock functions; production defaults may use real
  time, but business methods do not introduce direct `time.Now()` calls.
- Domain errors remain stable across layers and are mapped to stable API codes.
- Mutations that must be atomic use one database transaction.
- Audit metadata never contains passwords, raw tokens, session IDs, or secrets.
- New service behavior has a success-path test and at least one rejection or
  error-path test.

## Quality Check

- Run `go build ./...`, `go vet ./...`, and `go test ./...` from `backend/` on
  Centaurus using the Go version declared in `go.mod`.
- Run `go test -tags=integration ./...` when SQL or migrations change.
- Confirm new service behavior has success and rejection/error coverage.
- Confirm domain errors, HTTP codes, and frontend error contracts remain
  synchronized.
- Re-read the matching domain contract for state, permission, date/time, or
  transaction changes.
