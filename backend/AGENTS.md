# Backend

## Code style

- SQL statements in Go code should indent to match the surrounding Go.
- Use `any`, never `interface{}`.

## Testing

- Unit tests live beside the code they cover (`foo_test.go` next to `foo.go`).
- Service layer tests use stateful mocks implementing the repository interface.
- Repository integration tests live behind `//go:build integration` and run against a real Postgres; open the shared DB with `openIntegrationDB(t)`.

## Conventions

- Inject a `Clock` interface into services; never call `time.Now` directly in service code.
- Errors: sentinel values in `model/`, aliased at `service/`, mapped to HTTP codes at `handler/`.
- Audit logging: `audit.Record(ctx, audit.Event{...})`. Actor and IP come from the request context. Never include passwords, tokens, or session IDs in metadata.
