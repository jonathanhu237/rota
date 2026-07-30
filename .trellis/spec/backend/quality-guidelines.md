# Backend Quality Guidelines

## Required Patterns

- Format Go code with `gofmt`; use `any`, not `interface{}`.
- Pass `context.Context` through request, service, and repository calls.
- Inject time into services through `Clock`/clock functions. A constructor may
  choose `realClock`; business methods do not call `time.Now()` directly.
- Keep SQL indentation aligned with surrounding Go and parameterize all values.
- Preserve stable domain errors across layers.
- Record audited mutations without sensitive metadata.
- Add environment variables to `.env.example`; leave sensitive defaults blank.

## Testing

Unit tests live beside the code. Service tests use stateful mocks implementing
the narrow repository interfaces. Every new service method or material behavior
has:

- a success path; and
- at least one rejection or dependency-error path.

Handler tests assert status and response shape. Repository integration tests use
`//go:build integration`, call the shared `openIntegrationDB(t)`, and run
against PostgreSQL. SQL changes must run the integration suite.

## Quality Gate

Run from `backend/`:

```bash
go build ./...
go vet ./...
go test ./...
```

For SQL changes also run:

```bash
go test -tags=integration ./...
```

Run `govulncheck ./...` when dependencies or security-sensitive code change.
Under the Centaurus workflow these checks run on Centaurus after one-way rsync
from the local source-of-truth repository.

## Forbidden Patterns

- Direct `time.Now()` in service business methods.
- Global mutable state for repositories, clocks, or configuration.
- Secrets committed to source or placed in audit metadata.
- Handler tests that only assert status while ignoring the error code contract.
- Tests that merely restate implementation logic without exercising a failure
  boundary.

## Review Checklist

- Layer ownership and dependency direction remain intact.
- State-transition and scheduling invariants match `../domain/`.
- Transaction boundaries cover every dependent write.
- API codes and frontend contracts change together.
- New behavior has both success and rejection coverage.
