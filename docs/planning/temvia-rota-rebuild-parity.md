# Rota feature parity and acceptance matrix

Status: final cumulative review PASS. Latest Centaurus evidence: API build/vet/unit/integration passed; admin lint/typecheck/build and 35 files/186 tests passed; 4 current business browser tests passed; remote govulncheck reports 0 reachable vulnerabilities (4 uncalled module findings). See `temvia-rota-rebuild-final-verification.md` final checkpoint.

This matrix fixes the acceptance surface against the behavior at
`290be51287caa3831a3b8a14e196664964a2802d`. The npm foundation inventory is
from the generated `create-temvia@0.5.0` project, not from the local Temvia
checkout. A row is complete only when the business operation, effective
permission boundary, persistence rule, UI path where applicable, and both
success and rejection evidence are present.

Legend: **Template** means use/adapt the npm foundation's implementation;
**Port** means migrate Rota behavior into the generated application;
**Verify** means the required evidence to mark the row complete.

## Common foundation inventory

| Capability | npm 0.5.0 API evidence | npm 0.5.0 admin evidence | Acceptance treatment |
| --- | --- | --- | --- |
| Initial setup and bootstrap admin | `auth/application/setup.go`, `auth/adapter/httpapi/routes.go`: `GET /api/setup/status`, `POST /api/setup` | `routes/setup.tsx`, `features/auth/setup-form.tsx` | **Template**. Preserve one-time setup, no automatic session, setup token handling, and rejection/error responses. Verify setup success, expired/reused token, invalid input, and no second setup. |
| Login, current principal, logout and session status | `authentication.go`, `routes.go`: `/api/auth/login`, `/api/auth/me`, `/api/auth/session-status`, `/api/auth/logout` | `features/auth/login-form.tsx`, `session-monitor.tsx`, `routes/login.tsx`, authenticated layout | **Template**. Use UUID principal, Argon2id, Postgres session/version revocation, same-origin write protection and fail-closed session checks. Verify success, bad credentials, disabled/revoked/expired session, origin rejection. |
| Password recovery | `password_recovery.go`, `mail_dispatcher.go`: `/api/auth/password-reset/request`, `/complete` | `features/auth/password-reset-request-form.tsx`, `password-reset-form.tsx`, `routes/forgot-password.tsx`, `reset-password.tsx` | **Template**. Preserve enumeration resistance, keyed one-time authority, session clearing, outbox retry and localized mail. Verify known/unknown request equivalence, invalid/used/expired token, short password and delivery retry. |
| Invitations | `access.go`, `routes.go`: `/api/user-invitations`, resend/revoke, `/api/auth/invitations/accept` | `features/access/invitations-page.tsx`, `invitation-acceptance-form.tsx`, `routes/accept-invitation.tsx` | **Template**. Adapt Rota user creation to template invitations/roles; preserve expiry, resend, revoke, one-time acceptance and no implicit login. Verify role scope, duplicate email, revoked/expired/used invitation and success. |
| Roles and permissions | `auth/domain/access.go`, `auth/application/access.go`: `/api/roles`, `/api/access/role-options`, role CRUD | `features/access/roles-page.tsx`, access queries/forms | **Template plus catalog extension**. Add Rota business grants to the live catalog, map old admin boundary to grants, retain Super Admin protections and optimistic revisions. Verify each grant denial and immutable/system-role/revision rejection. |
| User lifecycle and account settings | `user_lifecycle.go`, `personal_settings.go`, access routes, avatar/profile/password/email-change routes | `features/access/users-page.tsx`, `features/settings/personal-account-settings-page.tsx`, auth forms | **Template**. Canonical UUID/account status/deleted identity is authoritative; preserve profile, locale/theme, password, email change, avatar, deactivate/reactivate/delete. Verify self-protection, last-Super-Admin protection, revision conflict, deletion history and session revocation. |
| Online users | `online.go`: `GET /api/online-users`, `POST /api/online-users/{id}/kick` | `features/access/online-users-page.tsx`, route | **Template**. Preserve valid-session aggregation and force sign-out. Verify read/write permission independence, deleted/revoked exclusion, and current/other user kick. |
| Operation history | `operation_log.go`, `operation_logs.go`: `/api/operation-logs`, `/status` | `features/operation-log/operation-logs-page.tsx`, route | **Template**. All Rota writes record best-effort success/failure with safe actor/target snapshots; retain filters, cursor and retention policy. Verify authorized read, denied read, failure record, cleanup/retention revision conflict and no credentials/tokens. |
| Email settings and mail task management | `settings.go`, `mail_tasks.go`, `routes.go`: settings, warnings, test mail, `/api/mail-tasks` | `features/settings/email-settings-page.tsx`, `features/email-tasks/email-tasks-page.tsx` | **Template**. Use encrypted SMTP settings and durable task/outbox dispatcher; adapt Rota notification kinds. Verify secure TLS/config validation, stale revision, no durable outbox, retry/delete authorization and task failure. |
| System identity/icon and themes/i18n | `identity.go`, identity routes and `shared/i18n`, `shared/theme` | `features/settings/system-identity-page.tsx`, `shared/theme`, `shared/i18n` | **Template, extended for Rota organization name**. No parallel branding service; preserve Rota product/organization labels and template icon behavior. Verify validation, version conflict, invalid icon, locale fallback and public identity. |

## Rota business backend matrix

| Domain capability | Baseline evidence (routes / service / persistence) | Generated integration target | Acceptance and rejection evidence |
| --- | --- | --- | --- |
| Positions | Admin `GET/POST /positions`, `GET/PUT/DELETE /positions/{id}`; `service/position.go`, `repository/position.go`, `model/position.go`; migration 00002 | Add Rota API under `/api/rota/positions` and admin positions feature | CRUD; unique/blank name, missing ID, delete-in-use and non-management denial. Baseline tests: `handler/position_test.go`, `service/position_test.go`, `repository/position_test.go`. |
| Qualifications | Admin `GET/PUT /users/{id}/positions`; `service/user_position.go`, `repository/user_position.go`, `model.Position`; migration 00003 | UUID account qualification join, historical-safe deletion, admin editor in user feature | Replace/list qualified positions; unknown position/user, duplicate IDs, disabled/non-admin denial. Baseline tests: `handler/user_position_test.go`, `service/user_position_test.go`, `repository/user_position_test.go`. |
| Templates | Admin `GET/POST/PUT/DELETE /templates`, `POST /templates/{id}/clone`; `service/template.go`, `repository/template.go`, `model/template.go`; migration 00004/00010/00016 | Rota template list/detail feature in generated admin | Create/update/delete/clone; non-empty/in-use delete rejection, missing template, invalid date/time and management denial. Baseline tests: `handler/template_test.go`, `service/template_test.go`, `repository/template_test.go`; frontend `templates-table`, `template-form-dialog`, clone/delete tests. |
| Slots, weekdays and requirements | Admin slot CRUD and slot-position CRUD under `/templates/{id}/slots...`; `service/template.go`, `repository/template_slot.go`; migrations 00010/00016; `attendance_responsible` in 00021 | Template detail route/components: slot dialog and slot-position dialog | Preserve overlapping-time exclusion per weekday, weekday removal cascades submissions/assignments, position membership, positive requirements, exactly one attendance responsible where applicable. Baseline repository `template_slot_db_test.go`; frontend `template-slot-dialog.test.tsx`. |
| Publications and lifecycle | Admin list/create/get/patch/delete plus `/publish`, `/activate`, `/end`; `service/publication.go`, `publication_pr4.go`, `repository/publication.go`, `model/publication.go`; migrations 00005/00009/00011 | Publication list/detail routes and lifecycle dialogs | Preserve states `DRAFT`, `ASSIGNING`, `PUBLISHED`, `ACTIVE`, `ENDED`, planned active windows, automatic effective-state resolution, single non-ended invariant, legal transition checks, delete restrictions and management denial. Baseline `service/publication*_test.go`, `handler/publication_test.go`, frontend publication dialog/table tests. |
| Automatic transitions/current publication | `ResolvePublicationState`, `GetCurrentPublication`, `GET /publications/current`; publication repository/state fields | Dashboard, availability and current-publication cards | Planned active/ended windows resolve at read time without mutating state; only one effective current publication. Verify before/within/after window, no current result, invalid window and conflicting publication. |
| Employee availability | `GET /publications/{id}/submissions/me`, `POST /submissions`, `DELETE /submissions/{slot}/{weekday}`; `service/publication.go`, `repository/publication.go`; migration 00005/00010/00015/00016 | `/availability`, publication availability tab and availability grid | Submit/delete only qualified/live user slots for eligible publication; duplicate, wrong weekday/slot, outside lifecycle, assignment conflict and unauthorized rejection. Baseline service tests; frontend `availability-grid.test.tsx`, route tests. |
| Administrator availability board/detail/edit | `GET /publications/{id}/availability-board`, `GET/PUT .../availability-submissions/{user_id}`; `service/admin_availability.go`, repository admin methods | Publication availability board/editor routes/components | Show employees/submission cells, edit only legal cells, preserve qualification/lifecycle and UUID identity. Verify unknown user, invalid cell, non-admin and stale/missing submission. Baseline `handler/admin_availability_test.go`, `service/admin_availability_test.go`; frontend `-admin-availability.test.tsx`. |
| Automatic assignment solver | `POST /publications/{id}/auto-assign`; `service/autoassign.go`, `publication_pr5.go`, `repository/assignment.go`; migrations 00006/00010/00011/00016 | Assignment board auto-assign action/dialog | Min-cost-flow assignment must preserve qualification, availability, no duplicate user in slot, time overlap, disabled/revoked-user exclusion and stable capacity semantics; no fairness optimization. Verify no candidate, partial coverage, revoked/disabled user, conflicting graph and non-assigning state. Baseline solver and service integration tests. |
| Manual assignments | `POST/DELETE /publications/{id}/assignments`; `service/publication_pr4.go`, `repository/assignment.go`; migrations 00006/00010/00011/00016 | Assignment board drag/drop, draft confirm and seat components | Preserve qualified/live user checks, slot-position membership, one user per slot, time overlap, draft/batch UX, unassign and optimistic error handling. Verify every conflict and disabled/unknown user rejection. Baseline handler/service/repository tests and assignment-board component/draft tests. |
| Assignment board | `GET /publications/{id}/assignment-board`; `AssignmentBoardResult` and board helpers | Publication assignment board, directory, grid, seats and side panel | Full grid with slot/position/headcount cells, coverage status, employee directory, search/sort, drag/drop, replacement/unassign, unsubmitted availability warning and before-unload draft protection. Verify rendering and rejected drop paths via assignment board test suite. |
| Roster/current roster | `GET /publications/{id}/roster`, `GET /roster/current`; `GetPublicationRoster`, `GetCurrentRoster`, occurrence projection | `/roster`, dashboard/current roster, weekly grid | Preserve assigned users, position/slot composition, weekday/occurrence dates, assignment overrides and deleted-user snapshots; authenticated read only. Verify no current roster, missing publication, deleted/disabled historical display and unauthorized access. Baseline response/service tests; frontend `weekly-roster` and route tests. |
| XLSX roster export | `GET /publications/{id}/schedule.xlsx`; `service/schedule_export.go`, Excelize | Assignment board and roster download actions | Localized workbook, occurrence/time rows, positions/seats, safe filename and no credential leakage. Verify language selection, empty roster, unknown publication and unauthorized response. Baseline `schedule_export_test.go`; frontend `lib/publications.test.ts`, assignment/roster tests. |
| Occurrence-specific assignment changes | `assignment_overrides` migration 00011; roster/assignment services and shift-change resolution | Roster/shift-change components and APIs | Preserve occurrence date validation, override identity, date/time semantics and cascade invalidation when base assignment changes. Verify invalid date, non-occurring weekday, stale assignment and deleted identity. |
| Shift-change requests: direct/give/swap | `POST/GET /publications/{id}/shift-changes`, detail and approve/reject/cancel; `service/shift_change.go`, `repository/shift_change.go`, `model/shift_change.go`; migrations 00009/00011/00012/00022 | `/requests`, publication shift-change page, give-direct/give-pool/swap dialogs | Preserve reciprocal qualification, eligible counterpart, pending/approved/rejected/cancelled/expired/invalidated states, one occurrence transfer, authorization visibility, time overlap and race safety. Verify disabled/revoked receiver, non-qualified counterpart, same user, expired request, cancellation/approval races. Baseline handler/service/repository + scheduling edge integration tests. |
| Leave requests and coverage | `POST /leaves`, pool/detail/cancel, mine/preview, publication compatibility; `service/leave.go`, `repository/leave.go`, `model/leave.go`; migrations 00012/00022 | `/leaves`, `/leaves/new`, `/leaves/:id`, leave cards and detail actions | Preserve public vs direct coverage visibility, category/reason, pending does not transfer responsibility, qualified active direct candidates, claim/approve/reject/cancel behavior, failure after occurrence start and one active leave constraint. Verify duplicate active leave, invalid occurrence, unauthorized visibility/action, disabled candidate and expired coverage. Baseline leave handler/service/repository/migration tests and frontend leave tests. |
| Attendance leader flow | `GET /attendance/current`, `POST /attendance/arrivals`, `POST /attendance/overtime`; `service/attendance.go`, `repository/attendance.go`, `model/attendance.go`; migration 00021 | `/attendance` employee leader page | Only attendance-responsible assigned leader sees eligible occurrence; arrival default/start window lock, idempotent recording, overtime window/note validation and localized status. Verify non-leader, too early/late, duplicate arrival, missing/long note and unauthorized user. Baseline `attendance_test.go`, frontend attendance route tests. |
| Attendance administrator corrections | `GET /publications/{id}/attendance`, shift detail, arrival PUT/DELETE, overtime POST/PATCH/DELETE, settings PATCH | Publication attendance admin page | Preserve date/shift listing, arrival set/change/clear, overtime create/edit/delete, orphan records, responsible-position constraints and settings. Verify non-admin, wrong publication/occurrence/user, invalid times/note, stale/missing record. Baseline handler/service/repository attendance tests; frontend publication attendance tests. |
| Notifications/email | Existing Rota email package/templates, outbox worker, shift-change/leave/publication notification producers; template email task dispatcher is authoritative | Template email settings/tasks plus Rota-triggered UX | Adapt every invitation/password/email-change/shift-change/leave/publication notification to typed template tasks, transactionally enqueued and localized EN/ZH with text+HTML. Verify missing branding/settings/outbox failure, retry/dead-letter and no notification on failed business transaction. Baseline `email`, `service/email_*`, outbox worker tests. |
| Rate limits/security boundary | Rota `handler/ratelimit.go`, config and auth tests; template Postgres abuse-protection limiter | Generated auth middleware and Rota write middleware | Template limiter is authoritative; preserve fail-closed limits, source identity safety, same-origin writes, audit-safe metadata and no secrets. Verify each reached anonymous bucket, 429 vs 503, trusted-proxy handling and body preservation. |

## Frontend route/component coverage

The baseline authenticated routes are `/`, `/users`, `/positions`,
`/templates`, `/templates/:templateId`, `/publications`,
`/publications/:publicationId`, `/publications/:publicationId/availability`,
`/publications/:publicationId/availability/:userId`,
`/publications/:publicationId/assignments`,
`/publications/:publicationId/shift-changes`,
`/publications/:publicationId/attendance`, `/availability`, `/roster`,
`/attendance`, `/requests`, `/leaves`, `/leaves/new`, `/leaves/:leaveId`, and
`/settings`; public routes are `/login`, `/forgot-password`, `/setup-password`,
and `/auth/confirm-email-change`. The generated foundation adds `/setup`,
`/reset-password`, `/accept-invitation`, `/users`, `/invitations`, `/roles`,
`/online-users`, `/operation-logs`, `/email-tasks`, `/settings`, and
`/personal-settings`. The migrated route tree may use the template's exact
paths where it avoids collisions, but every baseline workflow above must remain
reachable and localized.

Required frontend evidence includes the existing pure-logic/component tests
for board grid/directory/draft state, roster pivoting, availability cells,
publication lifecycle schemas, leave/shift-change forms, attendance display,
settings forms, table states and API error mapping, plus generated foundation
unit/component tests. Rendering tests remain focused and route/API behavior is
verified with pure helpers or HTTP tests unless an end-to-end scenario is
needed for a critical cross-layer path.

## Acceptance gate

A row can move from `Port`/`Template` to `Complete` only after:

1. API behavior and storage are implemented without a route placeholder or
   scaffold-only page.
2. Effective permissions match the baseline boundary; a UI hide is never the
   only authorization check.
3. Success and rejection/error tests cover each new service method/write route,
   with integration tests for SQL constraints/concurrency.
4. Admin and employee UI paths render real data and execute the operation,
   including English/Chinese resources and responsive states.
5. Migration up/down and non-destructive history behavior are exercised on an
   isolated database.
6. The exact Centaurus commands and outputs are appended to the design
   document's verification record after one-way rsync from the local source.

No legacy data migration, destructive reset, or production rollout timing is
part of this matrix; those require a separate approved plan.

## Current corrective-pass parity status

The second corrective pass now has code-level evidence for every public Rota
service method: success paths are exercised by
`api/internal/rota/service/migrated_services_test.go`, and invalid-input or
missing-resource boundaries are centralized in
`migrated_service_method_boundaries_test.go` and the domain-specific boundary
suite. Stateful fakes cover publication lifecycle, template slots, assignment
and roster projections, shift changes, leaves, and leader/administrator
attendance. Tagged PostgreSQL tests cover UUID identity, transactions, foreign
keys/check constraints, locked templates, duplicate submissions/arrivals,
assignment races, occurrence overrides, shift-change/leave transactions, and
outbox material.

The migrated admin suite now includes route registration and permission tests,
logout/account-switch cache-boundary tests, lifecycle/workflow request-contract
coverage, form/schema validation, assignment draft state, availability cells,
roster projection, qualification editing, and generated-template access,
identity, settings, operation-log, online-user, email-task, and auth coverage.
The final suite is 35 files / 186 tests. Rota notification templates contain
only business shift-change/leave messages; setup, invitation, password, email
change, SMTP, and account-lifecycle mail remain template-owned. Rota branding
reads the authoritative Temvia system identity, and deleted scheduling
identities remain available through the template history view without exposing
credentials.

The previously missing current-source F1–F3 browser evidence is now supplied
by `admin/e2e/rota-business.spec.ts` and its isolated runner, not the old r1/r2
sweep. The user-approved `rota.self` employee role has real login, own-availability
success and administrative-denial evidence. Centaurus ran the pinned scanner
successfully using a temporary TLS-preserving relay to official services:
0 reachable vulnerabilities, with 4 uncalled module findings. Account-switch
browser acceptance also passed. Historical checkpoints below remain historical.

## Corrective-pass validation evidence

The final corrective-pass source was synchronized one-way to isolated
Centaurus checkout `/home/jonathanhu237/rota-temvia-rebuild-20260914090000-r3`
on 2026-09-13 using rsync with `.git`, `.env`, dependency caches, and build
output excluded. API Go 1.27.0 unit, vet, and build checks passed; the Docker
PostgreSQL integration runner passed all API integration packages after
migrations 1–14 were applied. A separate disposable Postgres check passed
`migrate up`, `migrate down -all`, `migrate up`, ending at version 14.
Admin Node 24.20.0 / pnpm 11.24.0 install, typecheck, 31-file/175-test suite,
lint, and production build all passed (lint had eight warnings and zero errors).
The earlier r1/r2 browser and API acceptance records cover cross-layer route,
RBAC, employee/admin, localization, mail, and session behavior; the new r3
service regression coverage passed in both unit and integration runs.

`govulncheck` remains blocked because no Go 1.27-compatible scanner with a
reachable vulnerability database is available. No final review or Git commit
has been performed.

### Validation checkpoint — 2026-09-13 (isolated r4 continuation)

- The current uncommitted source was synchronized one-way to
  `/home/jonathanhu237/rota-temvia-rebuild-20260913-r4` with `.git`, `.env`,
  dependency caches, and build output excluded. A disposable remote `.env` was
  used only for migration checks and was not copied to the repository.
- Centaurus Go 1.27.0 API unit, vet, and build checks passed. The serialized
  `go test -p 1 -tags=integration ./...` run passed against a fresh PostgreSQL
  18.6 database on port `27191`, including the Rota UUID transaction,
  constraint, and concurrency suites. The `-p 1` wiring prevents destructive
  package fixtures from racing on the shared disposable database.
- The fresh migration database on port `27193` passed `up`, `down -all`, `up`,
  and `version`, ending at migration `14`.
- Centaurus Node 24.20.0 / pnpm 11.24.0 admin install, typecheck, unit tests,
  lint, and production build passed: 35 files / 185 tests, with eight existing
  lint warnings and zero errors.
- The pinned remote `govulncheck` attempt remains blocked by a timeout while
  verifying `golang.org/x/vuln@v1.2.0` through `sum.golang.org`. The local scan
  reports zero reachable vulnerabilities and four uncalled module
  vulnerabilities; this is not represented as a clean remote scan.
- The current r4 image stack passed three non-mail tests in
  `admin/e2e/auth.spec.ts` (setup/login/session/logout); the two password-mail
  tests were skipped because Mailpit was not included. This is current auth
  evidence only. Business F1–F3 browser acceptance remains a documented
  follow-up rather than an inherited claim from earlier checkpoints.
- No final review or commit has been performed.
