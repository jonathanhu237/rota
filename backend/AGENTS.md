# Backend

## Style guide references

When unsure how to write something idiomatic Go, consult these in order. Project conventions below override them where they conflict.

- [Effective Go](https://go.dev/doc/effective_go) — the foundational document.
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) — short do/don't list distilled from real reviews.
- [Google Go Style Guide → Best Practices](https://google.github.io/styleguide/go/best-practices) — for finer questions (naming, error wrapping, package layout). Skip rules that assume Google-internal tooling.
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md) — secondary reference, often more pragmatic on real-world patterns (functional options, error handling).

`gofmt`, `go vet`, and `govulncheck` enforce the mechanical layer; the references above cover the judgment layer.

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
