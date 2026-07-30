# Backend Directory Structure

## Layout

```text
backend/
├── cmd/
│   ├── server/              # composition root, HTTP server, workers
│   └── seed/                # development data scenarios
├── internal/
│   ├── audit/               # request-scoped audit abstraction
│   ├── config/              # environment configuration
│   ├── email/               # message builders and embedded templates
│   ├── handler/             # HTTP parsing, authorization, response mapping
│   ├── model/               # domain values and shared sentinel errors
│   ├── repository/          # PostgreSQL access and transaction runners
│   └── service/             # business rules and orchestration
└── go.mod
migrations/                  # repository-root Goose SQL migrations
```

The Go module is `github.com/jonathanhu237/rota/backend`. Keep packages under
`internal/`; new public library packages are not part of the current design.

## Feature Placement

For a normal persisted feature:

1. Put domain values, states, and invariant errors in `internal/model`.
2. Add database operations and parameter structs in `internal/repository`.
3. Define the smallest repository interface next to the consuming service in
   `internal/service`; do not make the service depend on a broad concrete
   repository.
4. Put validation, permissions, state transitions, audit calls, and
   orchestration in the service.
5. Put request/response DTOs, strict JSON decoding, HTTP status mapping, and
   route wiring in `internal/handler`.
6. Wire concrete dependencies in `cmd/server/main.go`.

`internal/service/attendance.go` and its handler/repository/model siblings are
a representative complete feature. `internal/service/setup.go` is the example
for transactional token plus outbox flows.

## Naming and File Rules

- Use lower-case Go file names; multiword feature files use underscores, such
  as `shift_change.go`.
- Keep unit tests beside their source as `*_test.go`.
- Repository integration tests use the `_db_test.go` convention where useful
  and include `//go:build integration`.
- Prefer feature-focused files over catch-all `utils.go` or `helpers.go`.
- Response-only transport structs stay in `handler`; persistence parameter
  structs stay in `repository`.

## Avoid

- SQL or database types in handlers.
- HTTP status decisions in services.
- Business validation in route registration or repository scan code.
- A service importing another feature's concrete repository when a narrow
  interface will do.
