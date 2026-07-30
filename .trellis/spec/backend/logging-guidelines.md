# Logging and Audit Guidelines

## Operational Logging

The service uses the standard `log/slog` package. Composition code passes a
`*slog.Logger` (or a narrow logger interface) to workers and services that need
it; constructors may fall back to `slog.Default()` when the existing API
documents that behavior.

- `Info`: lifecycle events such as the server starting.
- `Warn`: recoverable configuration or delivery conditions that require
  attention but do not stop the process.
- `Error`: failed startup, background work, or unexpected dependency failure.

Use a concise message followed by structured key/value fields:

```go
logger.Error("Failed to send outbox message", "message_id", message.ID, "error", err)
```

Follow the patterns in `backend/cmd/server/main.go`,
`backend/cmd/server/outbox_worker.go`, and `backend/internal/service/setup.go`.
Use discard loggers in tests when output is not under assertion.

## Audit Records

Security- and business-relevant mutations use:

```go
audit.Record(ctx, audit.Event{Action: "attendance.arrival.recorded", ...})
```

The actor and request IP are supplied through context. Use stable action names,
include identifiers needed for investigation, and keep audit recording aligned
with the success of the business operation. Follow
`backend/internal/audit/audit.go` and existing service calls.

## Sensitive Data

Never log or place in audit metadata:

- passwords or password hashes;
- raw setup, reset, or email-change tokens;
- session IDs or cookies;
- authorization headers;
- SMTP credentials or other environment secrets.

Prefer durable numeric entity IDs over full payloads. Email addresses and names
are personal data: include them only when an established audit requirement
needs them, and never as a substitute for an entity ID.

## Avoid

- Unstructured `fmt.Printf` logging in server code.
- Logging the same propagated error in repository, service, and handler.
- Treating audit records as debug logs or operational logs as audit records.
- Swallowing an error solely because it was logged.
