# Audit Capability

## Purpose

The audit log is an append-only record of every state-changing domain operation and every significant authentication event. It exists so administrators can answer "who did what, when, and from where" without reconstructing history from business tables.

In scope: mutations emitted by the service layer (user management, position/template/publication lifecycle, availability submissions, assignments, shift changes) and authentication events (login success/failure, logout, password reset request, password set). Out of scope: read operations, health checks, static asset requests, rate-limit rejections, and a reader API/admin UI (v1 inspection is DB-only).

## Requirements

### Requirement: Mutating domain operations emit audit events

The service layer SHALL emit an audit event at the successful end of every state-changing domain operation covered by the action taxonomy (user management, position/template/publication lifecycle, availability submissions, assignments, shift changes). Operations that return an error SHALL NOT emit an audit event.

#### Scenario: Successful mutation is recorded

- **WHEN** a service method in the action taxonomy completes successfully
- **THEN** exactly one audit event is recorded with the matching action constant

#### Scenario: Failed mutation is not recorded

- **GIVEN** a service method that returns an error before completing the mutation
- **WHEN** the caller observes the error
- **THEN** no audit event is recorded for that call

### Requirement: Authentication events emit audit events

The auth service SHALL emit audit events for login success, login failure, logout, password reset request, and password set.

#### Scenario: Login success is recorded with the user as target

- **WHEN** a user authenticates successfully via `AuthService.Login`
- **THEN** an `auth.login.success` event is recorded with `target_type = user` and `target_id` equal to the authenticated user's id
- **AND** the metadata includes the `email` used

#### Scenario: Login failure is recorded without a typed target

- **WHEN** `AuthService.Login` takes any failure branch
- **THEN** an `auth.login.failure` event is recorded with `target_type` and `target_id` both NULL
- **AND** the metadata includes `email` and `reason`

#### Scenario: Password reset request is recorded without a typed target

- **WHEN** `AuthService.RequestPasswordReset` is invoked for any email
- **THEN** an `auth.password_reset.request` event is recorded with `target_type` and `target_id` both NULL
- **AND** the metadata includes `email` and `user_found`

#### Scenario: Logout and password set target the user

- **WHEN** `AuthService.Logout` or `AuthService.SetupPassword` completes successfully
- **THEN** the corresponding `auth.logout` or `auth.password.set` event is recorded with `target_type = user` and `target_id` equal to the acting user's id

### Requirement: Read operations are not audited

Listing, getting, and exporting data SHALL NOT produce audit events.

#### Scenario: Read call produces no audit row

- **WHEN** a handler invokes any list/get/export service method
- **THEN** no row is inserted into `audit_logs` as a result of that call

### Requirement: Health checks, static assets, and rate-limit rejections are not audited

The system SHALL NOT record audit events for health-check requests, static asset requests, or rate-limit rejections.

#### Scenario: Rate-limited request produces no audit row

- **GIVEN** an HTTP request that is rejected by the rate limiter before it reaches a service method
- **WHEN** the response is returned to the client
- **THEN** no row is inserted into `audit_logs` as a result of that request

### Requirement: Audit events are persisted to audit_logs

Audit events SHALL be persisted to the `audit_logs` table with columns `id` (BIGSERIAL PK), `occurred_at` (TIMESTAMPTZ NOT NULL, default `NOW()`), `actor_id` (BIGINT, nullable), `actor_ip` (TEXT, nullable), `action` (TEXT NOT NULL), `target_type` (TEXT, nullable), `target_id` (BIGINT, nullable), and `metadata` (JSONB NOT NULL, default `{}`).

#### Scenario: Event row is inserted at mutation time

- **WHEN** a service records an event via `audit.Record`
- **THEN** one row is inserted into `audit_logs` with `occurred_at` set by the database and the event's action, target, metadata, actor, and IP fields populated from context

### Requirement: actor_id has no foreign key to users

The `actor_id` column SHALL NOT have a foreign-key constraint to `users`. Deleting or renaming a user SHALL NOT alter or remove existing audit rows.

#### Scenario: User deletion leaves audit history intact

- **GIVEN** an audit row with `actor_id = U`
- **WHEN** the user with id `U` is deleted from `users`
- **THEN** the audit row remains present with `actor_id = U` unchanged

### Requirement: Audit table indexes cover core query patterns

The `audit_logs` table SHALL provide indexes on `(occurred_at DESC)`, `(actor_id, occurred_at DESC)`, `(target_type, target_id, occurred_at DESC)`, and `(action, occurred_at DESC)`.

#### Scenario: Indexes are created by migration

- **WHEN** migration `00008_create_audit_logs_table.sql` runs
- **THEN** `audit_logs_occurred_at_idx`, `audit_logs_actor_idx`, `audit_logs_target_idx`, and `audit_logs_action_idx` exist on `audit_logs`

### Requirement: Services emit audit events directly via audit.Record

Services SHALL call `audit.Record` at the successful end of a mutation, describing only the domain action, target, and metadata. The audit system SHALL resolve actor and IP from the request context rather than requiring the caller to pass them.

#### Scenario: Service call records an event with resolved actor and IP

- **GIVEN** an authenticated HTTP request processed by `AuditMiddleware` and `RequireAuth`
- **WHEN** the handler's service invokes `audit.Record` with an action, target, and metadata
- **THEN** the persisted row has `actor_id` set to the authenticated user and `actor_ip` set to the caller IP derived by the middleware

### Requirement: AuditMiddleware injects recorder and caller IP

`AuditMiddleware` SHALL wrap every HTTP request, injecting the `audit.Recorder` into the request context via `audit.WithRecorder` and the caller IP via `audit.WithActorIP`.

#### Scenario: Middleware wraps all HTTP requests

- **WHEN** any HTTP request enters the server
- **THEN** its context contains an `audit.Recorder` and a caller IP value before reaching any handler

### Requirement: RequireAuth attaches the authenticated actor

Once the session has been validated, `RequireAuth` SHALL attach the authenticated user to the context via `audit.WithActor`.

#### Scenario: Authenticated handler sees actor in context

- **GIVEN** a request with a valid session
- **WHEN** `RequireAuth` passes the request to the next handler
- **THEN** the context carries the authenticated user as the audit actor

### Requirement: audit.Record never returns an error

`audit.Record` SHALL NOT return an error value to its caller. Failures during encoding, persistence, or validation SHALL be handled internally without propagating to the domain operation.

#### Scenario: Persistence failure does not surface to caller

- **GIVEN** a recorder whose underlying INSERT will fail
- **WHEN** a service calls `audit.Record`
- **THEN** the call returns without an error and the surrounding mutation completes normally

### Requirement: Empty action is rejected and not persisted

`audit.Record` SHALL reject events whose `Action` is the empty string. Rejected events SHALL be logged at `slog.Warn` and SHALL NOT be persisted.

#### Scenario: Empty action drops the event

- **WHEN** `audit.Record` is called with `Action == ""`
- **THEN** no row is inserted into `audit_logs`
- **AND** a `slog.Warn` entry is emitted

### Requirement: Missing recorder falls through to a no-op

When no recorder is present in the context, `audit.Record` SHALL fall through to a `noopRecorder` that silently discards events.

#### Scenario: Test context without recorder discards events

- **GIVEN** a context that has no `audit.Recorder` attached
- **WHEN** code calls `audit.Record`
- **THEN** the call returns without error and no row is persisted

### Requirement: Metadata must not contain secrets

Event metadata SHALL NOT contain passwords, plaintext tokens, session IDs, or any other secret value.

#### Scenario: Guard test rejects secret-bearing metadata

- **WHEN** `TestUserServiceAuditMetadataHasNoSecrets` scans recorded events emitted by user flows
- **THEN** no event metadata contains password, token, or session-id values
- **AND** any new user call site that breaks this invariant causes the test to fail

### Requirement: Metadata values must be JSON-serialisable

Metadata values SHOULD be small, typed JSON-serialisable values (strings, numbers, booleans, arrays of IDs). Timestamps SHALL be encoded as RFC3339 strings.

#### Scenario: Timestamps are encoded as RFC3339

- **WHEN** a service records an event with a timestamp in metadata
- **THEN** the metadata value is a string in RFC3339 format

### Requirement: auth.login.failure and auth.password_reset.request omit typed target

Events for `auth.login.failure` and `auth.password_reset.request` SHALL set `target_type` and `target_id` to NULL. Presence of a matching user SHALL be captured only in metadata (e.g. `user_found`).

#### Scenario: Failed login omits typed target

- **WHEN** an `auth.login.failure` event is recorded for an email that matches a user
- **THEN** `target_type` and `target_id` on the persisted row are both NULL
- **AND** metadata includes `reason` and `email`

#### Scenario: Password reset request omits typed target

- **WHEN** an `auth.password_reset.request` event is recorded
- **THEN** `target_type` and `target_id` on the persisted row are both NULL
- **AND** metadata includes `email` and `user_found`

### Requirement: User actions are emitted by the user and user-position services

The system SHALL emit `user.create`, `user.update`, `user.status.activate`, `user.status.disable`, `user.invitation.resend`, and `user.qualifications.replace` events with `target_type = user`, from the corresponding `UserService` and `UserPositionService` methods on their success paths.

#### Scenario: Creating a user emits user.create

- **WHEN** `UserService.CreateUser` completes successfully
- **THEN** a `user.create` event is recorded with `target_type = user`, `target_id` equal to the new user's id, and metadata including `email` and `is_admin`

#### Scenario: Status change emits the matching activate or disable action

- **WHEN** `UserService.SetUserStatus` completes on the activate path
- **THEN** a `user.status.activate` event is recorded with metadata including `status`
- **WHEN** `UserService.SetUserStatus` completes on the disable path
- **THEN** a `user.status.disable` event is recorded with metadata including `status`

#### Scenario: Replacing qualifications emits user.qualifications.replace

- **WHEN** `UserPositionService.ReplaceQualifications` completes successfully
- **THEN** a `user.qualifications.replace` event is recorded with `target_type = user` and metadata including `position_ids`

### Requirement: Position actions are emitted by the position service

The system SHALL emit `position.create`, `position.update`, and `position.delete` events with `target_type = position` from the corresponding `PositionService` methods on their success paths.

#### Scenario: Position lifecycle events

- **WHEN** `PositionService.CreatePosition` succeeds
- **THEN** a `position.create` event is recorded with `target_type = position` and metadata including `name`
- **WHEN** `PositionService.DeletePosition` succeeds
- **THEN** a `position.delete` event is recorded with `target_type = position` and metadata including `name`

### Requirement: Template actions are emitted by the template service

The system SHALL emit `template.create`, `template.update`, `template.delete`, and `template.clone` events with `target_type = template` from the corresponding `TemplateService` methods on their success paths.

#### Scenario: Cloning a template records source id

- **WHEN** `TemplateService.CloneTemplate` succeeds
- **THEN** a `template.clone` event is recorded with `target_type = template` and metadata including `source_template_id`

### Requirement: Template shift actions are emitted by the template service

The system SHALL emit `template.shift.create`, `template.shift.update`, and `template.shift.delete` events with `target_type = template_shift` from the corresponding `TemplateService` methods on their success paths.

#### Scenario: Creating a template shift records its template and position

- **WHEN** `TemplateService.CreateTemplateShift` succeeds
- **THEN** a `template.shift.create` event is recorded with `target_type = template_shift` and metadata including `template_id`, `position_id`, and timing fields

### Requirement: Publication actions are emitted by the publication service

The system SHALL emit `publication.create`, `publication.delete`, `publication.publish`, `publication.activate`, `publication.end`, and `publication.auto_assign` events with `target_type = publication` from the corresponding `PublicationService` methods on their success paths.

#### Scenario: Creating a publication records timing metadata

- **WHEN** `PublicationService.CreatePublication` succeeds
- **THEN** a `publication.create` event is recorded with `target_type = publication` and metadata including `template_id`, `name`, `submission_start_at`, `submission_end_at`, and `planned_active_from`

#### Scenario: Auto-assign records assignment count

- **WHEN** the auto-scheduler completes a `publication.auto_assign` invocation successfully
- **THEN** a `publication.auto_assign` event is recorded with `target_type = publication` and metadata including `assignments_created`

### Requirement: Submission actions are emitted by the publication service

The system SHALL emit `submission.create` and `submission.delete` events with `target_type = availability_submission` from `PublicationService.SubmitAvailability` and `PublicationService.DeleteAvailability` on their success paths.

#### Scenario: Submitting availability records publication and slot count

- **WHEN** `PublicationService.SubmitAvailability` succeeds
- **THEN** a `submission.create` event is recorded with `target_type = availability_submission` and metadata including `publication_id` and `slot_count`

### Requirement: Assignment actions are emitted by the publication service

The system SHALL emit `assignment.create` and `assignment.delete` events with `target_type = assignment` from `PublicationService.CreateAssignment` and `PublicationService.DeleteAssignment` on their success paths.

#### Scenario: Creating an assignment records its identifying fields

- **WHEN** `PublicationService.CreateAssignment` succeeds
- **THEN** an `assignment.create` event is recorded with `target_type = assignment` and metadata including `publication_id`, `user_id`, and `template_shift_id`

### Requirement: Shift change actions are emitted by the shift-change and publication services

The system SHALL emit `shift_change.create`, `shift_change.approve`, `shift_change.reject`, `shift_change.cancel`, and `shift_change.expire.bulk` events with `target_type = shift_change_request` from the corresponding service methods on their success paths.

#### Scenario: Creating a shift change request records its type and references

- **WHEN** `ShiftChangeService.CreateRequest` succeeds
- **THEN** a `shift_change.create` event is recorded with `target_type = shift_change_request` and metadata including `type`, `publication_id`, `requester_shift_id`, and `counterpart_shift_id`

#### Scenario: Bulk expiry on activation records count

- **WHEN** publication activation expires pending shift-change requests in bulk
- **THEN** a `shift_change.expire.bulk` event is recorded with `target_type = shift_change_request` and metadata including `expired_count` and `publication_id`

### Requirement: TargetType values are restricted to the defined set

When a `target_type` is set on an audit event, it SHALL be one of: `user`, `position`, `template`, `template_shift`, `publication`, `availability_submission`, `assignment`, `shift_change_request`.

#### Scenario: Event outside the enumeration is rejected at review

- **WHEN** a new call site sets `target_type` to a value outside the enumerated set
- **THEN** reviewers reject the change until the enumeration is extended

### Requirement: Production recorder writes synchronously

The production recorder `repository.AuditRecorder` SHALL persist each event with a single synchronous `INSERT INTO audit_logs` on the request goroutine. There SHALL NOT be a retry, async buffer, durable queue, or graceful-shutdown drain.

#### Scenario: Event is persisted before the request returns

- **WHEN** a handler's service records an event
- **THEN** the `INSERT INTO audit_logs` completes on the request goroutine before the HTTP response is flushed

### Requirement: Persistence failures degrade to a warning log

JSON encoding errors and SQL errors during audit persistence SHALL be logged at `slog.Warn` with the action name and, if known, the actor id, and then swallowed. The primary mutation SHALL NOT fail because audit persistence failed.

#### Scenario: SQL failure does not fail the mutation

- **GIVEN** an `INSERT INTO audit_logs` that returns a SQL error
- **WHEN** the service has already successfully completed its mutation and records the event
- **THEN** the service returns success to the caller
- **AND** a `slog.Warn` entry is emitted including the action name

### Requirement: Audit inspection is DB-only in v1

In v1, audit inspection SHALL be performed against the `audit_logs` table via direct database access. The system SHALL NOT expose an HTTP endpoint for reading audit data.

#### Scenario: No HTTP endpoint serves audit reads

- **WHEN** a client issues an HTTP request expecting to read audit rows
- **THEN** no route handler is registered to serve such a request

### Requirement: Audit rows are retained permanently

Audit rows SHALL be retained permanently. The system SHALL NOT run automated pruning, archival, or rollover against `audit_logs`.

#### Scenario: No pruning job removes rows

- **WHEN** time passes and the system runs its scheduled jobs
- **THEN** no job deletes or archives rows from `audit_logs`

### Requirement: Application code is append-only against audit_logs

Service code SHALL NOT issue `UPDATE` or `DELETE` statements against `audit_logs`. Only operational database access may touch existing rows.

#### Scenario: No service method mutates existing audit rows

- **WHEN** any service method executes
- **THEN** it does not issue `UPDATE` or `DELETE` against `audit_logs`
