# API

## Style guide references

When unsure how to write idiomatic Go, consult Effective Go, Go Code Review
Comments, the Google Go Style Guide, and the Uber Go Style Guide. Project
conventions below override them where they conflict.

## Code style

- SQL statements in Go code should indent to match the surrounding Go.
- Use `any`, never `interface{}`.
- Run `gofmt` on changed Go files.

## Testing

- Unit tests live beside the code they cover (`*_test.go`).
- Service tests use stateful mocks implementing the narrow repository interfaces.
- PostgreSQL repository and concurrency tests use `//go:build integration` and
  `TEST_POSTGRES_DSN` (or the integration runner) against real Postgres.

## Conventions

- Inject a `Clock` interface into services; never call `time.Now` directly in
  service code.
- Errors: sentinel values in `model/`, aliased at `service/`, and mapped to
  HTTP problem responses at the adapter boundary.
- Audit logging uses `audit.Record(ctx, audit.Event{...})`. Never include
  passwords, tokens, or session IDs in metadata.
