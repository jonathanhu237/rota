# Error Handling

## Error Ownership

Stable domain failures are sentinel errors, usually declared in
`backend/internal/model` and aliased by `backend/internal/service` when the
service API exposes them. Service-only failures such as invalid credentials may
be declared in the service package.

Repositories map storage-specific failures to model/service-facing sentinel
errors. Handlers use `errors.Is`, so wrapping with `%w` is allowed when context
is useful.

## Layer Pattern

- Repository: preserve unexpected database errors; translate only known
  no-row, constraint, serialization, or conflict conditions.
- Service: validate input and state, return stable domain errors for expected
  rejection paths, and propagate unexpected dependency errors.
- Handler: strictly decode input, authenticate/authorize, map expected errors to
  status plus API code, and map unknown failures to `INTERNAL_ERROR`.
- Logs: log unexpected operational failures at the boundary that has useful
  context; do not log the same error at every layer.

The switches in `backend/internal/handler/auth.go` and
`backend/internal/handler/attendance.go` are representative mappings.

## API Error Contract

All JSON errors use the shape produced by `writeError` in
`backend/internal/handler/response.go`:

```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "Invalid request body"
  }
}
```

`code` is a stable machine-readable upper-snake-case value consumed by
`frontend/src/lib/api-error.ts`. `message` is a safe fallback, not an internal
database or stack trace. When adding a code, update the frontend union and both
locale error mappings in the same change.

Use `readJSON` for normal request bodies: it caps the body at 1 MiB, rejects
unknown fields, and rejects trailing JSON. Only use
`readJSONAllowUnknownFields` for an explicitly compatibility-tolerant endpoint.

## Avoid

- String matching on error messages when `errors.Is` or a constraint identifier
  can express the contract.
- Returning raw `err.Error()` to clients.
- Converting every error to `INTERNAL_ERROR` inside the service, which destroys
  expected rejection semantics.
- Adding a backend API code without updating the frontend translator and tests.
