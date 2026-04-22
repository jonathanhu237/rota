# Audit Logging

## Overview

The audit log is an append-only record of every state-changing domain operation and every significant authentication event in the system. It exists so administrators can answer "who did what, when, and from where" without reconstructing history from business tables.

In scope:

- Mutations emitted by the service layer: user management, position/template/publication lifecycle, availability submissions, assignments, shift changes.
- Authentication events: login success, login failure, logout, password reset request, password set.

Out of scope:

- Read operations (list, get, export) — not audited.
- Health checks, static asset requests, rate-limit rejections (see Decisions).
- A reader API or admin UI. v1 inspection is DB-only; a future `/audit` admin view is tracked separately.

Scheduling-related actions are cross-referenced by name here; their domain semantics live in the scheduling spec. Auth flow semantics live in the auth spec.

## Data model

The `audit_logs` table (migration `00008_create_audit_logs_table.sql`):

| Column        | Type                       | Nullable | Notes                                                                 |
| ------------- | -------------------------- | -------- | --------------------------------------------------------------------- |
| `id`          | `BIGSERIAL PRIMARY KEY`    | no       | Monotonic append order.                                               |
| `occurred_at` | `TIMESTAMPTZ NOT NULL`     | no       | Defaults to `NOW()` at insert.                                        |
| `actor_id`    | `BIGINT`                   | yes      | NULL means the event was unauthenticated (e.g. failed login).         |
| `actor_ip`    | `TEXT`                     | yes      | NULL when no IP could be derived (e.g. internal scheduled sweep).     |
| `action`      | `TEXT NOT NULL`            | no       | One of the constants in `backend/internal/audit/audit.go`.            |
| `target_type` | `TEXT`                     | yes      | NULL for events that do not identify a single entity (login failure). |
| `target_id`   | `BIGINT`                   | yes      | NULL when `target_type` is NULL, or when the target has no numeric ID.|
| `metadata`    | `JSONB NOT NULL DEFAULT`   | no       | Defaults to `{}`. Free-form per-action payload.                        |

Indexes:

- `audit_logs_occurred_at_idx (occurred_at DESC)` — "what happened recently" timeline.
- `audit_logs_actor_idx (actor_id, occurred_at DESC)` — "everything this user did".
- `audit_logs_target_idx (target_type, target_id, occurred_at DESC)` — "history of this entity".
- `audit_logs_action_idx (action, occurred_at DESC)` — "all events of a given kind".

`actor_id` intentionally has no foreign key to `users`: historical rows must survive user deletion so the audit trail remains complete.

## Emission model

Services emit events directly at the successful end of a mutation:

```go
audit.Record(ctx, audit.Event{
    Action:     audit.ActionUserCreate,
    TargetType: audit.TargetTypeUser,
    TargetID:   &userID,
    Metadata:   map[string]any{"email": user.Email, "is_admin": user.IsAdmin},
})
```

The caller only describes the domain action. Actor and IP are resolved from the request context:

- `AuditMiddleware` (in `backend/internal/handler/audit_middleware.go`) wraps every HTTP request and injects the `audit.Recorder` plus the caller IP (via `audit.WithRecorder` and `audit.WithActorIP`).
- `RequireAuth` attaches the authenticated user with `audit.WithActor` once the session has been validated.

`audit.Record` never returns an error. An empty `Action` is rejected with a `slog.Warn` and is not persisted.

Metadata rules:

- Must NOT contain passwords, plaintext tokens, session IDs, or any other secret. The unit test `TestUserServiceAuditMetadataHasNoSecrets` scans recorded events to guard this invariant; new call sites must keep that test passing.
- Should be small, typed-JSON-serialisable values (strings, numbers, bools, arrays of IDs). Timestamps use RFC3339 strings.
- Should include the fields an investigator would want without opening the row's target record, e.g. `email` on user events, `reason` on login failures, `assignments_created` on auto-assign.

## Persistence model

The production recorder is `repository.AuditRecorder` (`backend/internal/repository/audit.go`). It writes synchronously — one `INSERT INTO audit_logs` per event — on the request goroutine.

Failure handling:

- JSON encoding errors and SQL errors are logged at `slog.Warn` level with the action name and (if known) actor ID, then swallowed.
- The primary operation never fails because audit persistence failed.
- There is no retry, no async buffer, no durable queue, and no graceful-shutdown drain.

At this project's scale (~100 employees, a few thousand events per year), synchronous writes cost a few milliseconds per mutation and give durability without coordination. See Decisions for why async was rejected.

When no recorder is present in the context, `audit.Record` falls through to a `noopRecorder` that silently discards events. This keeps tests that don't care about audit output simple.

## Actions

Every action constant declared in `backend/internal/audit/audit.go` is listed below, grouped by domain. The "Emitted by" column names the service method that records the event at its happy path; failures (returned errors) never emit. The "Typical metadata" column lists the keys usually present — metadata is not schema-enforced, so new call sites may add fields.

### Users

| Action                         | TargetType | Emitted by                                     | Typical metadata                    |
| ------------------------------ | ---------- | ---------------------------------------------- | ----------------------------------- |
| `user.create`                  | `user`     | `UserService.CreateUser`                       | `email`, `is_admin`                 |
| `user.update`                  | `user`     | `UserService.UpdateUser`                       | changed fields (name, is_admin, …)  |
| `user.status.activate`         | `user`     | `UserService.SetUserStatus` (activate path)    | `status`                            |
| `user.status.disable`          | `user`     | `UserService.SetUserStatus` (disable path)     | `status`                            |
| `user.invitation.resend`       | `user`     | `UserService.ResendInvitation`                 | `email`                             |
| `user.qualifications.replace`  | `user`     | `UserPositionService.ReplaceQualifications`    | `position_ids`                      |

### Positions

| Action             | TargetType | Emitted by                      | Typical metadata |
| ------------------ | ---------- | ------------------------------- | ---------------- |
| `position.create`  | `position` | `PositionService.CreatePosition`| `name`           |
| `position.update`  | `position` | `PositionService.UpdatePosition`| changed fields   |
| `position.delete`  | `position` | `PositionService.DeletePosition`| `name`           |

### Templates

| Action              | TargetType | Emitted by                      | Typical metadata     |
| ------------------- | ---------- | ------------------------------- | -------------------- |
| `template.create`   | `template` | `TemplateService.CreateTemplate`| `name`               |
| `template.update`   | `template` | `TemplateService.UpdateTemplate`| changed fields       |
| `template.delete`   | `template` | `TemplateService.DeleteTemplate`| `name`               |
| `template.clone`    | `template` | `TemplateService.CloneTemplate` | `source_template_id` |

### Template shifts

| Action                    | TargetType       | Emitted by                            | Typical metadata                     |
| ------------------------- | ---------------- | ------------------------------------- | ------------------------------------ |
| `template.shift.create`   | `template_shift` | `TemplateService.CreateTemplateShift` | `template_id`, `position_id`, timing |
| `template.shift.update`   | `template_shift` | `TemplateService.UpdateTemplateShift` | changed fields                       |
| `template.shift.delete`   | `template_shift` | `TemplateService.DeleteTemplateShift` | `template_id`                        |

### Publications

| Action                       | TargetType    | Emitted by                                       | Typical metadata                                                             |
| ---------------------------- | ------------- | ------------------------------------------------ | ---------------------------------------------------------------------------- |
| `publication.create`         | `publication` | `PublicationService.CreatePublication`           | `template_id`, `name`, `submission_start_at`, `submission_end_at`, `planned_active_from` |
| `publication.delete`         | `publication` | `PublicationService.DeletePublication`           | `name`                                                                       |
| `publication.publish`        | `publication` | `PublicationService` (PR4 lifecycle)             | `published_at`                                                               |
| `publication.activate`       | `publication` | `PublicationService` (PR4 lifecycle)             | `activated_at`                                                               |
| `publication.end`            | `publication` | `PublicationService` (PR4 lifecycle)             | `ended_at`                                                                   |
| `publication.auto_assign`    | `publication` | `PublicationService` (PR5 auto-scheduler)        | `assignments_created`                                                        |

### Submissions

| Action                | TargetType                | Emitted by                                    | Typical metadata                           |
| --------------------- | ------------------------- | --------------------------------------------- | ------------------------------------------ |
| `submission.create`   | `availability_submission` | `PublicationService.SubmitAvailability`       | `publication_id`, `slot_count`             |
| `submission.delete`   | `availability_submission` | `PublicationService.DeleteAvailability`       | `publication_id`                           |

### Assignments

| Action                | TargetType   | Emitted by                                    | Typical metadata                                      |
| --------------------- | ------------ | --------------------------------------------- | ----------------------------------------------------- |
| `assignment.create`   | `assignment` | `PublicationService.CreateAssignment`         | `publication_id`, `user_id`, `template_shift_id`      |
| `assignment.delete`   | `assignment` | `PublicationService.DeleteAssignment`         | `publication_id`, `user_id`, `template_shift_id`      |

### Shift changes

| Action                        | TargetType             | Emitted by                                       | Typical metadata                                       |
| ----------------------------- | ---------------------- | ------------------------------------------------ | ------------------------------------------------------ |
| `shift_change.create`         | `shift_change_request` | `ShiftChangeService.CreateRequest`               | `type`, `publication_id`, `requester_shift_id`, `counterpart_shift_id` |
| `shift_change.approve`        | `shift_change_request` | `ShiftChangeService.Approve`                     | `type`, resulting assignment IDs                       |
| `shift_change.reject`         | `shift_change_request` | `ShiftChangeService.Reject`                      | `reason`                                               |
| `shift_change.cancel`         | `shift_change_request` | `ShiftChangeService.Cancel`                      | `cancelled_by`                                         |
| `shift_change.expire.bulk`    | `shift_change_request` | `PublicationService` (PR4 publication activation)| `expired_count`, `publication_id`                      |

### Auth

| Action                           | TargetType | Emitted by                             | Typical metadata                  |
| -------------------------------- | ---------- | -------------------------------------- | --------------------------------- |
| `auth.login.success`             | `user`     | `AuthService.Login`                    | `email`                           |
| `auth.login.failure`             | —          | `AuthService.Login` (failure branches) | `email`, `reason`                 |
| `auth.logout`                    | `user`     | `AuthService.Logout`                   | —                                 |
| `auth.password_reset.request`    | —          | `AuthService.RequestPasswordReset`     | `email`, `user_found`             |
| `auth.password.set`              | `user`     | `AuthService.SetupPassword`            | `purpose` (`invitation` or `password_reset`) |

`auth.login.failure` and `auth.password_reset.request` intentionally omit `TargetType`/`TargetID`: the request is by email, which may not correspond to any user row, and we do not want to leak that distinction in the typed columns. Presence of a matching user is captured in metadata (`user_found`) where it is safe for server-side investigation only.

## TargetType enumeration

| Constant                           | Entity class                               |
| ---------------------------------- | ------------------------------------------ |
| `user`                             | `users` row                                |
| `position`                         | `positions` row                            |
| `template`                         | `templates` row                            |
| `template_shift`                   | `template_shifts` row                      |
| `publication`                      | `publications` row                         |
| `availability_submission`          | `availability_submissions` row             |
| `assignment`                       | `assignments` row                          |
| `shift_change_request`             | `shift_change_requests` row                |

## Access

In v1, audit inspection is DB-only: administrators SSH to the server and use `psql` against the `audit_logs` table. The four indexes above cover the expected queries (timeline, per-actor, per-target, per-action).

There is no HTTP endpoint for reading audit data. An admin UI at `/audit` is anticipated but out of scope for this spec; it will be proposed as a separate change.

## Retention

Audit rows are kept permanently. There is no automated pruning, archival, or rollover. Growth at the target scale (~100 employees, tens of thousands of events per year) is on the order of a few megabytes per year, so retention cost is negligible. If retention ever becomes an issue, a pruning job will be proposed as a follow-up change.

## Invariants

- **Append-only from the application.** No service code issues `UPDATE` or `DELETE` against `audit_logs`. Only operational DB access may touch existing rows, and doing so is discouraged.
- **No secrets in metadata.** Passwords, plaintext setup tokens, session IDs, and similar values must never appear in an `Event.Metadata` map. `TestUserServiceAuditMetadataHasNoSecrets` enforces this on user flows; reviewers must extend equivalent checks when adding sensitive call sites.
- **Actor has no FK.** `actor_id` references a user by ID but is not enforced at the DB level, so deleting or renaming a user does not rewrite history.
- **Best-effort durability.** A failed insert degrades to a warning log. Callers never observe audit errors and never retry.
- **Empty action is a programming error.** `audit.Record` with `Action == ""` is dropped and logged; tests must catch this before production.

## Decisions

- **Service-level emission, not middleware.** A middleware safety-net would have to understand HTTP routes, infer which handler succeeded, and synthesise domain context — ending up with either low-fidelity events ("POST /users succeeded") or duplicating service logic. Emitting from the service layer gives precise actions and rich metadata at the cost of explicit call sites, which is the right trade-off for a small surface.
- **Synchronous writes.** At this scale, the extra round-trip per mutation is unmeasurable. Async would introduce an in-memory queue, drop-on-crash risk, and graceful-shutdown complexity we don't need.
- **No FK on `actor_id`.** Audit must survive user deletion. A foreign key would either block deletion or cascade history away, both unacceptable.
- **Rate-limit rejections are not audited.** They are high-volume, often caused by misconfigured clients or scans, and have no useful target. They are observable via the standard application log and the rate-limiter's own metrics. If abuse investigations ever need durable records, a separate `security_events` table will be proposed.
- **Reads are not audited.** Listing or exporting data does not change state; the signal-to-noise ratio of audit-logging reads is poor at this scale and would dwarf the mutation stream.
