# Temvia/Rota migration design and implementation checklist

Status: implementation and direct final review verified. Employee-role clarification approved and implemented as `rota.self`; current-source business browser and remote vulnerability checks passed. See `temvia-rota-rebuild-final-verification.md` final checkpoint for authoritative evidence; earlier checkpoints below retain historical blockers.

## Fixed inputs and release provenance

- Baseline commit: `290be51287caa3831a3b8a14e196664964a2802d`.
- Branch: `temvia-rota-rebuild`.
- Source: the public npm registry, not the local `../temvia` checkout.
- Generator package: `create-temvia@0.5.0` (the `latest` dist-tag also resolved to
  `0.5.0` at investigation time, but implementation uses the exact version).
- Registry tarball:
  `https://registry.npmjs.org/create-temvia/-/create-temvia-0.5.0.tgz`.
- npm metadata observed before source changes (2026-09-13):
  - SHA-512 integrity:
    `sha512-EqplgnDgU28sH/mv2nNVZm+/vT5UwBCDDKvLk4M9DvLK8Zmzqe/dqhP8zeniHRpBXhaDY3VhFl1foufC+xSQeA==`
  - SHA-1 shasum: `02799cc9d83b23651b654a74d41632a095278c9d`
  - downloaded tarball SHA-256:
    `002a2a4aa54cb0f31d5d6e262d88d00055e100df00c79ec44cff0bd82050999e`
  - npm registry signature key id:
    `SHA256:DhQ8wR5APBvFHLF/+Tc+AYvPOdTpcIDqOhxsBHRwC7U`
  - repository: `git+https://github.com/jonathanhu237/temvia.git`
  - package engine: Node `>=24`; generated admin package manager: pnpm `11.24.0`.
- Exact generation command:
  `npx --yes --package=create-temvia@0.5.0 create-temvia <empty-dir> --module github.com/jonathanhu237/rota/api`.
- Fresh generated base used for inspection:
  `/tmp/rota-temvia-npm-base-20260913-r1.lMwv6d`.
  It was generated into a new empty directory, initialized its own uncommitted
  Git repository, and was not used as an upstream working checkout.
- Isolated Centaurus validation directory established before large changes:
  `/home/jonathanhu237/rota-temvia-rebuild-20260913-r1` (remote mode `700`,
  owner `jonathanhu237`, verified empty). SSH host alias `centaurus` is a
  ProxyJump through the documented `centaurus-frp` arrangement in
  `~/.ssh/config`; no credentials or secret values are copied into the repo or
  this document.

## Design

### Foundation ownership

Import the generated npm foundation into this repository as the source tree:
`api/`, `admin/`, root `compose.yaml`, root `Makefile`, root `.env.example`,
root and generated license/upstream notices. The generated applications remain
independent copies: there is no runtime dependency on Temvia, no local template
checkout, and no automatic upstream upgrade mechanism. Preserve the existing
Rota Git history and retain approved planning/ADR files.

The template's common systems are authoritative and are not forked beside the
old Rota implementations:

- authentication, sessions, password recovery, setup and invitation acceptance
- UUID account identity and account lifecycle/deletion history
- roles, permissions, role assignment and authorization checks
- transactional email tasks/outbox and retry worker
- personal account settings, email settings and system identity/branding icon
- operation history, online-user monitoring and in-process abuse protection

Rota-specific code is an extension of the generated API/admin applications,
not a second common auth stack. Business handlers use the template's
`PrincipalAuthenticationService` and persisted roles; they do not inspect a
second user/password/status table.

### Identity and history

Template account UUIDs are the canonical user identity. Scheduling entities
retain UUID user references and use the template's `auth_deleted_user_identities`
history when an account is deleted; the Rota `users` compatibility view combines
live `auth_users` rows with those preserved identity snapshots. This keeps
schedule, request, attendance, qualification, and audit-relevant displays
stable without copying credentials, roles, or password hashes into Rota.
Current candidate and permission queries use live `auth_users`, current
qualifications, and the template principal, so deleted or disabled accounts
cannot be newly scheduled. No legacy Rota IDs are guessed or reused, and no old
data is dropped as part of this work. A future legacy data migration must
explicitly map old integer IDs to UUIDs and verify every history reference before
rollout.

Business entities (positions, templates, slots, publications, assignments,
requests, leave and attendance records) may retain numeric IDs where they are
not account identities; their user/account fields and serialized API models use
UUID strings consistently.

### Authorization

The existing effective boundary is preserved:

- management operations currently protected by `RequireAdmin` require the
  corresponding Rota management permission(s), initially granted to the
  template's immutable `Super Admin` role;
- self-service schedule, availability, shift-change, leave and leader-attendance
  operations remain available to authenticated eligible users and keep their
  existing contextual ownership/qualification checks;
- a principal without the required permission receives the template's
  forbidden problem response; contextual business rejections retain distinct
  baseline error codes and status behavior.

The live permission catalog is extended rather than duplicated. Rota keys are
owned by the application and are included in role responses so delegated roles
can be tested explicitly. No route may infer authorization from a client role
label or a stale cached read.

### API and admin structure

The generated auth handler owns `/api/*` common endpoints. Rota endpoints are
mounted below `/api/rota/` to avoid duplicate route registration while allowing
the generated handler to remain authoritative. The admin uses the generated
TanStack Router/shadcn shell and adds Rota feature routes under the same
authenticated layout. API path changes are allowed by the fixed decision; each
new route is recorded in the parity matrix and has a success and rejection
check. The old Rota route paths are not retained as a second API implementation.

The admin preserves both employee and administrator workflows, including
responsive board/roster views, drag/drop assignment editing, localized English
and Chinese strings, and download behavior. User-visible strings remain in
translated resources; business forms use React Hook Form and the repository's
pinned Zod v3 import convention. The npm template currently ships Zod 4.5.4,
so its common schemas will be adapted to the project-required `zod/v3` API (or
pinned to the compatible v3 package) rather than silently violating the
admin conventions.

### Data and migrations

Use the generated migration runner and naming convention (`*.up.sql` and
`*.down.sql`). Start with the template's common migrations, then add a
forward-only Rota business schema migration set. The schema must cover the
complete baseline model: positions and qualifications, templates/slots/slot
weekdays/requirements, publications and lifecycle fields, availability,
assignments and occurrence overrides, shift-change requests and leaves,
attendance arrivals/overtime/settings, plus indexes and constraints needed by
existing validation. Keep Postgres authoritative; do not add Redis or a
message broker, in line with ADR 0001 and ADR 0003. If a cache is later
considered, specify it per read model and invalidation contract per ADR 0002.

The business schema stores historical snapshots before any account deletion can
clear a UUID reference. Migration/rollout of existing production data is
explicitly deferred by the approved scope; schema work must still be
non-destructive and must not use `down` migrations against a live legacy
installation as a data-removal shortcut.

### Common-system adaptations and conflicts

The following are recorded decisions for this implementation rather than
silent behavior changes:

1. **Integer IDs/bcrypt vs UUID/Argon2id:** use template UUID/Argon2id for new
   auth and adapt business references. Legacy ID/data conversion is deferred.
2. **`IsAdmin` vs template RBAC:** map the old admin boundary to a role with the
   required Rota management grants; contextual employee checks remain in
   business code. Do not grant business management permissions to a role merely
   because its UI label says “admin”.
3. **Postgres 17 + `database/sql`/`lib/pq` vs template Postgres 18 + `pgx`:**
   use the generated stack and verify on Centaurus with mise Go 1.27.0; retain
   SQL semantics and error mappings through tests.
4. **Rota product/organization branding vs template System Identity:** extend
   the template identity/settings surface to retain both baseline branding
   fields without creating a parallel branding service/table. System emails and
   shell labels must use the authoritative identity values.
5. **Rota email outbox vs template email tasks:** use the template task/outbox
   dispatcher and adapt all Rota invitation, password, email-change,
   shift-change, leave and notification producers to its typed durable task
   contract. Do not retain the old worker.
6. **Rota admin Zod v3 convention vs npm template Zod v4:** adapt imports and
   incompatible schema calls while preserving the template behavior.
7. **Template API `/api/...` and Rota API paths:** use `/api/rota/...` for the
   business extension and update the generated admin client; this is a path
   adaptation, not a capability removal.

Any newly discovered conflict involving business meaning, permissions,
state-transition legality, or historical data must be added to this register
and stopped for confirmation before changing that semantic. Suspected bugs are
reported with baseline evidence and are not “fixed” by the migration unless the
fixed acceptance matrix says so.

## Implementation checklist

### Phase 0 — evidence and safety (complete before product code)

- [x] Read root/API/admin conventions, issue-tracker/domain guidance,
      ADR 0001–0004, and the approved planning record.
- [x] Verify the published npm package, exact version, tarball integrity,
      registry signature metadata, repository, and Node requirement.
- [x] Generate a fresh base in a temporary empty local directory with the exact
      package; do not generate from `../temvia`.
- [x] Enumerate current Rota routes, services, models, migrations, pages,
      API clients, and tests; enumerate generated common routes/services/pages/
      tests.
- [x] Verify Centaurus SSH connectivity, available mise versions and rsync,
      and create an empty mode-700 validation directory.
- [x] Write the design/checklist and complete parity/acceptance matrix before
      product source edits.
- [x] Record an immutable local evidence snapshot of baseline route/service/
      schema behavior and resolve any material semantic conflict with the user.

### Phase 1 — import and common foundation

- [x] Import the exact generated foundation without `.env`, runtime data,
      dependency caches, or generated nested Git metadata.
- [x] Preserve `LICENSE`, `admin/UPSTREAM.md`, and relevant notices.
- [x] Adapt template common schemas to the admin Zod v3 convention and run
      generated API/admin tests.
- [x] Extend template identity/permission catalog only where the parity matrix
      requires Rota behavior; keep common service ownership in template code.
- [x] Add a root `.env.example` inventory for every new variable with safe
      defaults/blank secret entries; never overwrite `.env`.

### Phase 2 — business data and application layers

- [x] Add non-destructive Rota migrations with UUID account references,
      historical snapshots, constraints, indexes, and down files.
- [x] Port/reshape positions and qualifications with all CRUD and rejection
      paths.
- [x] Port templates, slots, weekdays, requirements and clone/delete rules.
- [x] Port publications, state resolution/automatic transitions, create/edit/
      delete/publish/activate/end operations and rejection rules.
- [x] Port availability submissions and administrator availability editing.
- [x] Port manual assignment, drag/drop draft operations and min-cost-flow
      automatic assignment with qualification, availability, overlap, slot
      uniqueness and status checks.
- [x] Port assignment board, roster, current roster and localized XLSX export.
- [x] Port occurrence-specific overrides, shift-change requests (give, direct,
      swap), expiry/invalidation and leave coverage workflows.
- [x] Port attendance leader arrival/overtime, administrator corrections,
      orphan handling and settings.
- [x] Port Rota notifications/producers through the template mail-task/outbox
      dispatcher with localized text/html output and retry-safe transactions.
- [x] Add generated-admin-compatible Rota routes/components/queries/i18n and
      preserve employee/admin navigation access boundaries.

### Phase 3 — verification loop

- [x] Add/retain a success and rejection/error test for every new service
      method and business write route; include concurrency/constraint cases.
- [x] Run focused API/admin tests and checks after each capability group.
- [x] Rsync one-way to the isolated Centaurus directory using excludes for
      `.env`, `.git`, build/dependency caches and runtime volumes; do not use
      `--delete` outside this isolated directory.
- [x] On Centaurus, use mise-managed Go 1.27.0, Node 24.x and pnpm 11.24.0;
      run API build/vet/unit/integration and admin
      lint/typecheck/test/build. Forward service ports only from the isolated
      stack for smoke exercise.
- [x] Run pinned govulncheck v1.2.0 on Centaurus: 0 reachable vulnerabilities;
      4 uncalled module findings retained in the record. Official-service TLS
      access used a temporary allowlisted SSH relay; checksums remained enabled.
- [x] Run migration up/down checks against an isolated database and verify no
      data-bearing volume is removed.
- [x] Exercise representative admin/employee workflows against the running
      stack. Current business browser suite: 4 passed, 0 skipped, including
      role boundaries, own availability, branding, mail links and cache isolation.
      Broader matrix behavior is covered by the service/SQL/admin regression suites.
- [x] Update the parity matrix and verification record with exact commands,
      commit-independent evidence, and residual blockers.
- [x] Remove copied Rota auth/account mail, branding, audit, and translation
      surfaces that are owned by the template; retain only business notification
      rendering and the authoritative identity read adapter.
- [x] Stop/report guard observed: employee-role conflict was reported, user
      approval obtained, and the main agent directly fixed and re-verified it.
      No outstanding blocker remains; no incomplete work was committed.

## Verification evidence record

This section is append-only during implementation. It must contain command,
environment, date, result, and relevant output summary for every remote check.

### Validation checkpoint — 2026-09-13

- Environment: isolated Centaurus checkout
  `/home/jonathanhu237/rota-temvia-rebuild-20260913-r1`, PostgreSQL on
  `127.0.0.1:27132`, API on `http://localhost:27180`, admin on
  `http://localhost:27173`, and disposable Mailpit on `http://localhost:27125`.
  Source was synchronized one-way from the local checkout with `rsync -az`
  while excluding `.env`, `.git`, dependency caches, and build output. The
  local `.env` was not changed.
- Remote API checks (mise Go 1.27.0):
  `cd api && go test ./...`, `go vet ./...`, `go build ./...`, followed by
  `TEST_POSTGRES_DSN=postgres://temvia:pa55word@127.0.0.1:27132/temvia?sslmode=disable go test -tags=integration ./...` — all
  passed. The integration run exercises the shipped migrations and the
  historical-identity/mail-material checks.
- `govulncheck` was attempted through mise. The cached v1.7.0 scanner is
  built with Go 1.26 and cannot analyze this Go 1.27 module; rebuilding it
  could not download `golang.org/x/vuln` because outbound access timed out,
  and the binary-mode retry could not refresh `vuln.go.dev` for the same
  network reason. No vulnerability result is claimed.
- Remote admin checks (mise Node 24.20.0/pnpm 11.24.0):
  `cd admin && CI=true pnpm install --frozen-lockfile`, `pnpm test`,
  `pnpm lint`, `pnpm check`, and `pnpm build` — all passed. `admin/pnpm-workspace.yaml`
  explicitly permits the required `msw` postinstall script.
- Remote generated admin checks (mise Node 24.20.0/pnpm 11.24.0):
  `cd admin && pnpm install --frozen-lockfile`, `pnpm check`, `pnpm test`, and
  `pnpm build` — 25 files / 148 tests passed; typecheck and build passed.
  `admin/pnpm-workspace.yaml` explicitly permits the required `msw` script.
- Browser acceptance against the forwarded remote stack: generated admin
  system-identity (including icon/public auth pages, localization, and stale
  revision conflict), email-task delivery/failure/retry/bulk-delete, and
  user-lifecycle (deactivation, fresh-login restoration, deletion, historical
  actor display, and Chinese UI) Playwright suites passed. A subsequent route
  sweep covered the authenticated shell, all Rota publication/availability/
  assignment/shift-change/attendance/roster/leave routes, common settings and
  access routes, with no page errors. A read-only account sweep confirmed
  management controls are absent/disabled while self-service Rota reads remain
  available.
- API/mail acceptance: setup/login/session and origin checks; granular
  `rota.read`/`rota.manage` boundaries; Rota CRUD, publication lifecycle,
  qualifications, availability, auto/manual assignment, roster and English /
  Chinese XLSX exports; direct/pool/swap shift changes and leave coverage;
  attendance/overtime; operation-log detail/filtering; invitation create,
  resend, revoke, one-time acceptance and invalid-token rejection; account
  deactivate/reactivate/delete with invalidated sessions and historical UUID
  identities; system identity/branding; encrypted durable Rota mail tasks,
  SMTP delivery, terminal failure and retry. Mailpit confirmed rendered
  recipient names, English and Chinese subjects/bodies, and updated branding.
- Migration compatibility fix verified by the integration suite: the direct
  legacy lifecycle migration-preservation fixture removes the newer `users`
  compatibility view before exercising migration 10; normal production
  rollback removes migration 13 first, so migration layering and retained
  identity semantics remain unchanged.
- Residual validation state in this checkpoint was disposable only: remote
  fixture users, invitations, mail settings, outbox records, Mailpit messages,
  and Rota seed rows remained until the stack teardown. No Git commit or final
  review has been performed.

### Validation checkpoint — 2026-09-13 (isolated r3 corrective-pass final)

- Environment: source synchronized one-way with `rsync -az --delete` (excluding
  `.git`, `.env`, dependency caches, and build output) to the mode-700 isolated
  Centaurus checkout `/home/jonathanhu237/rota-temvia-rebuild-20260914090000-r3`.
  Centaurus used mise Go 1.27.0, Node 24.20.0, and pnpm 11.24.0. A disposable
  `.env` containing generated test-only keys was created remotely and removed
  after validation; the local `.env` was not changed.
- API checks:
  `cd api && mise exec --raw go@1.27.0 -- go test ./...`, `go vet ./...`, and
  `go build ./...` all passed. The service regression additions for publication,
  template slots, shift changes, leaves, and attendance passed in the full
  suite.
- API/PostgreSQL integration:
  `TEST_COMPOSE_PROJECT=rota-integration-r3 POSTGRES_HOST_PORT=27152 mise exec
  --raw go@1.27.0 -- ./scripts/test-integration.sh` passed. Docker applied all
  migrations `1` through `14`, and every API integration package passed before
  the disposable Postgres volume/network were removed.
- Migration roundtrip:
  an isolated Postgres container on `27153` ran Docker `migrate up`, `migrate
  down -all`, `migrate up`, and `migrate version`; the final version was `14`.
  The volume/network were removed after the check.
- Admin checks:
  `cd admin && mise exec --raw node@24.20.0 -- pnpm install --frozen-lockfile`,
  `pnpm check`, `pnpm test -- --runInBand`, `pnpm lint`, and `pnpm build` all
  passed: 31 test files and 175 tests. Lint reported eight pre-existing
  warnings and zero errors; the production build completed successfully.
- The prior r1/r2 browser and API acceptance checkpoints remain the evidence for
  cross-layer auth, RBAC, employee/admin workflows, localization, mail delivery,
  and route sweeps. No Git commit or final review has been performed. The only
  recorded verification gap remains `govulncheck`, which is unavailable with a
  Go 1.27-compatible scanner and reachable vulnerability database.

### Validation checkpoint — 2026-09-13 (isolated r2 browser follow-up)

- Source was synchronized to `/home/jonathanhu237/rota-temvia-rebuild-20260913-r2`
  with `.env` excluded. A fresh Postgres volume was migrated through version
  13; API and admin images were rebuilt with the current local source. The
  disposable stack used API `27180`, admin `27173`, Postgres `27132`, and
  Mailpit `27125`.
- The dedicated `admin/e2e/online-users.spec.ts` suite passed 3 tests:
  aggregation of two target sessions plus force sign-out/relogin,
  read-only/no-read permission boundaries, and Chinese localization including
  self-sign-out. Its opt-in idle-expiry test was intentionally skipped because
  `E2E_ONLINE_IDLE_TIMEOUT_MS` was unset.
- A browser acceptance script created, edited, and deleted a custom role via
  the Roles page, and persona checks confirmed manager, read-only, and no-read
  accounts receive the Roles access-denied view. The authenticated Rota reader
  saw the dashboard while management-only routes redirected to Home.
- The authenticated shell now exposes `PreferencesButtons`, so the language
  and appearance controls are available after login as well as on public auth
  pages. `cd admin && pnpm check && pnpm test -- --runInBand` passed (25 files /
  148 tests).
- A fresh Centaurus Postgres container on `27142` was migrated through version
  13 and the final source was run with
  `TEST_POSTGRES_DSN=postgres://temvia:pa55word@127.0.0.1:27142/temvia?sslmode=disable mise exec --raw go@1.27.0 -- go test -tags=integration ./...`;
  all API integration packages passed. The container and temporary source
  directory were removed afterward.
- The r2 stack, volume, network, containers, and temporary checkout were
  removed after validation. No Git commit or final review has been performed.

### Validation checkpoint — 2026-09-13 (corrective-pass continuation)

- Source changes remained local and uncommitted on `temvia-rota-rebuild`; the
  local `.env` and runtime data were not changed. The Rota auth/account cleanup
  removed copied auth, branding, and account-lifecycle translation/audit
  surfaces while retaining business qualification translations and the
  read-only system-identity adapter used by notifications.
- Focused local checks passed after the final regression additions:
  `cd api && go test ./internal/rota/...` and
  `cd admin && pnpm test -- --runInBand` (35 files / 185 tests). The full API
  and admin checks listed in the preceding r3 checkpoint remain green; the
  current source-only changes are formatting, translation cleanup, and an
  additional `GetCurrentPublication` rejection case.
- The pinned local vulnerability command
  `GOVULNCHECK_VERSION=v1.2.0 GOVULNCHECK_GO_VERSION=go@1.27.0 make vuln-check`
  completed with zero reachable
  vulnerabilities; four uncalled module vulnerabilities were reported by the
  scanner. A Centaurus rerun could not download or verify the scanner/database
  because `proxy.golang.org` and `sum.golang.org` timed out, and the local
  macOS binary was not portable to the remote architecture. No remote clean
  vulnerability result is claimed.
- No final review or Git commit has been performed. The remaining state is a
  verification caveat, not an authorization to bypass the scanner or commit.

### Validation checkpoint — 2026-09-13 (isolated r4 continuation)

- The current source was synchronized one-way with `rsync -az --delete`,
  excluding `.git`, `.env`, dependency caches, and build output, to the fresh
  mode-700 Centaurus checkout
  `/home/jonathanhu237/rota-temvia-rebuild-20260913-r4`. A disposable remote
  `.env` contained only test keys and was not synchronized back to the local
  repository.
- Centaurus API checks with mise Go 1.27.0 passed:
  `cd api && go test ./...`, `go vet ./...`, and `go build ./...`.
- The corrected serialized integration runner passed with a fresh disposable
  PostgreSQL 18.6 database on port `27191`:
  `TEST_COMPOSE_PROJECT=rota-integration-r4g POSTGRES_HOST_PORT=27191 mise exec
  --raw go@1.27.0 -- ./scripts/test-integration.sh`. All API packages,
  including the UUID scheduling repository transaction/constraint/race suite,
  passed. The runner now uses `go test -p 1` because package-level integration
  fixtures reset shared auth and Rota tables.
- An isolated migration roundtrip on port `27193` ran migration `up`, `down
  -all`, `up`, and `version`; the final version was `14`. The disposable
  container, network, and volume were removed after the check.
- Centaurus admin checks with mise Node 24.20.0 / pnpm 11.24.0 passed:
  `pnpm install --frozen-lockfile`, `pnpm check`, `pnpm test -- --runInBand`,
  `pnpm lint`, and `pnpm build` (35 files / 185 tests; lint reported eight
  existing warnings and zero errors).
- Against the rebuilt r4 API/admin images and a fresh migrated database,
  `PLAYWRIGHT_BASE_URL=http://127.0.0.1:25273 E2E_SETUP_URL=... mise exec
  --raw node@24.20.0 -- pnpm exec playwright test e2e/auth.spec.ts
  --project=chromium --retries=0 --workers=1` passed 3 authentication/browser
  tests. The two password-mail assertions were skipped because Mailpit was not
  part of this isolated stack. This verifies current setup/login/session/logout
  behavior; it is not claimed as business F1–F3 browser evidence.
- A remote vulnerability attempt was made with
  `timeout 180s mise exec --raw go@1.27.0 -- make vuln-check`; it failed while
  verifying `golang.org/x/vuln@v1.2.0` because `proxy.golang.org` could not
  reach `sum.golang.org` before timeout. The local pinned scan remains the only
  vulnerability result: zero reachable vulnerabilities and four uncalled
  module vulnerabilities. No clean remote scanner result is claimed.
