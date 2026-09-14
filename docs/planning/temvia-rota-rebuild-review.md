# Temvia/Rota cumulative implementation review

Fixed baseline: `290be51287caa3831a3b8a14e196664964a2802d`.
Fixed requirements: confirmed rounds in `temvia-rota-rebuild.md` and ADR 0004. Design/parity documents operationalize those requirements, not replace them.
Reviewer: main agent directly, no delegated reviewers.
Final status: review 3 PASS after direct fixes and user-approved employee mapping; see `temvia-rota-rebuild-final-verification.md` final checkpoint. Reviews 1 and 2 below are retained history.
Review 1: FAIL. No commit was authorized by that review.

This document preserves the historical failed-review findings. The second
corrective implementation pass below is status evidence, not a final review or
approval.

## Review surface

`git diff 290be51287caa3831a3b8a14e196664964a2802d...HEAD` and corresponding commit list are empty because the implementation is intentionally uncommitted. Review therefore covers the cumulative tracked working-tree diff (`git diff <baseline>`) AND every nonignored new source/config/document file under `api/`, `admin/`, root and planning/ADR paths. Generated source compared against the preserved npm 0.5.0 scaffold for context; ported business source compared against baseline backend/frontend. `git diff --check` passes. No claim is made that legacy-app tests validate the new apps.

## Standards

### S1 — P1: new business code lacks required regression tests

Rule: root AGENTS Done definition requires success and rejection/error tests per new service method, and SQL integration tests; backend conventions require stateful service mocks and tagged real-Postgres repository tests. Parity matrix also explicitly requires ported business/UI tests.

Evidence: `api/internal/rota/service/` has only `autoassign_test.go`, `template_validation_test.go`, `validation_test.go`; there are no tests exercising the migrated publication/leave/shift-change/attendance service methods. Repository tests are only error mapping and a schema/mail-material smoke test. `migration_integration_test.go` is not build-tagged and does not exercise scheduling repositories or concurrency. In `admin/`, the only Rota feature tests are `features/rota/queries.test.ts` plus an access qualification dialog test; migrated assignment drafts, availability, lifecycle, leave, roster and attendance lack the required migrated tests. Running unchanged `frontend/` tests validates the old app, not the new ports.

Fix: port/adapt baseline service/HTTP/repository and required frontend logic tests to actual UUID-backed `api/` and template-integrated `admin/`; add success/rejection regression cases for integration differences. Run actual migrated suites on Centaurus. Do not satisfy this by counting old tests or merely changing checklist text.

### S2 — P1: verification wiring still targets the retired implementation

Rule: root workflow requires project checks and durable accurate verification records. Evidence: `.github/workflows/ci.yml` still uses `backend/go.mod`, tests/builds `backend/`, builds/tests `frontend/`, and builds old Dockerfiles; `scripts/test-integration.sh` targets the old Compose/schema. Root AGENTS Commands and Done definition still direct developers to old paths and Make targets removed by the new Makefile. New `admin/package.json` has `check`, not the documented lint contract. A green existing CI would not check the new product.

Fix: migrate CI, integration runner and active instructions to actual api/admin runtime, use correct fresh Postgres migrations, and preserve meaningful build/vet/lint/typecheck/unit/integration/image checks. Preserve language conventions in the new applicable paths. Validate updated entry points on Centaurus.

### S3 — P2: completion/evidence records overstate actual verification

Rule: root requires satisfied acceptance checklist and durable verification evidence before completion/commit. Evidence: design Phase 3 checks off per-method tests and all critical browser paths despite S1 and F1/F2 below; parity status says implementation complete without row-level implemented/test locations. Design identity section promises nullable live-user foreign keys and per-row snapshots while migration 13 actually uses non-null unconstrainted UUIDs plus deleted-identity view. ADR 0004 still says implementation has not been approved. No reproducible business acceptance script is in repo; reported broad API/browser sweeps alone are not durable tests.

Fix: correct status and implementation description without weakening confirmed requirements. Attach real test/command evidence per parity area after fixes, including deleted-identity retention and actual migrations. Keep govulncheck explicitly blocked until a compatible scanner and reachable DB produce a result; never mark it clean based on other tests.

## Spec

### F1 — P1: ordinary employees cannot discover their retained workflows

Requirement: all employee/admin business capabilities and effective permission boundaries retained; page/navigation changes cannot remove access.

Evidence: `admin/src/features/auth/authenticated-shell.tsx:53,130` shows Rota navigation only for `rota.read` / `rota.manage` / Super Admin. `admin/src/routes/_authenticated/rota/index.tsx:11-16` redirects a plain authenticated employee (no Rota management/read grant) to Home. Home is only a welcome message. Backend correctly leaves self-service routes authenticated-only, but those employees cannot reach availability/roster/requests/leaves/leader attendance via normal navigation. Granting all employees `rota.read` instead would also expose admin read models and is not a valid fix.

Fix: expose authenticated employee dashboard/self-service navigation independently of management/read grants; keep management-only links and endpoints protected. Test a real employee with no Rota administrative grants, not just reader/manager personas. Ensure leader attendance and all baseline routes are reachable.

### F2 — P1: notification action links point to nonexistent routes

Requirement: preserve functional notification/leave/shift-change workflows while adapting URLs.

Evidence: `api/internal/rota/email/shift_change.go:100-105` still builds `/requests` and `/leaves/{id}`. The new router only declares `/rota/requests` and `/rota/leaves/$leaveId`; there are no old-path aliases. Existing ported email tests assert old URLs, so they pass while mail links open Not Found.

Fix: generate new canonical application routes (or explicit safe supported aliases), update EN/ZH tests and verify following generated email links in an authenticated browser reaches the intended page.

### F3 — P2: organization branding capability was dropped

Requirement: keep all existing functions; design explicitly promises both product and organization fields through template identity ownership, without a second branding table.

Evidence: `api/internal/rota/repository/branding.go:25` selects `system_name, '', revision`, unconditionally discarding organization name. Template auth identity schemas/services and new settings UI have no organization field; only unused translation keys remain. Baseline `backend/internal/service/branding.go` and `frontend/src/components/settings/branding-form.tsx` provide editable validated organization branding used in email output.

Fix: extend authoritative template identity end-to-end for optional organization name (validation/version conflict/public projection/UI/email rendering), without reviving old branding service. Add success/rejection/localization tests.

### F4 — P1: Rota cache survives logout/account switching

Requirement: preserve ownership/permission effects and integrate with template authentication lifecycle.

Evidence: `features/rota/queries.ts` adds independent `['rota','auth','me']`, unowned `['me','leaves',...]`, `['attendance','current']`, publication submissions/shift-change and other business keys. Template shell logout and account-switch effects only clear known common namespaces (`authenticated-shell.tsx:65-96`); route/auth-error cleanup likewise excludes Rota. Login sets only `['auth','current-user']`. Thus A's business data and Rota principal remain after logout/login B: `ensureQueryData` can return A's cached permissions immediately; useQuery can render A's private leave/submission data while fetching B, or retain it on errors.

Fix: integrate one authoritative current-principal query and an owner/session-aware business cache contract; clear/cancel all protected Rota queries on logout, auth failure, account switch, revoked session and relevant permission changes. Test A→logout→B and in-flight/failed-refetch cases; B must never see A's data or cached management affordances. Avoid just hiding controls while retaining stale private data.

### F5 — P2: old parallel apps/common systems remain in active checkout

Requirement: template common systems authoritative, no two common implementations; approved repository plan imports template then cleans replaced old implementation to lower maintenance cost.

Evidence: entire legacy `backend/`, `frontend/`, old `migrations/`, and old Compose deployments remain alongside `api/` and `admin/`; original CI still maintains those old apps. Old auth/password/session/mail workers are retained as runnable applications, not an immutable out-of-tree baseline. The old app is being installed/tested separately while migration is marked complete.

Fix: first finish ported regression tests and parity, then remove superseded runnable source/config paths and dead duplicated common code in the new Rota port. Git baseline already preserves old implementation. Preserve user `.env`, actual data and unrelated assets; do not delete data-bearing paths/volumes. Update all active references; legacy ADR/history may remain clearly marked.

## Loop state

Reviews completed: 3 / 3; final direct verification PASS. Employee-role decision approved and implemented.
Fix attempts completed: S1=2, S2=2, S3=2, F1=2, F2=2, F3=2, F4=2, F5=2.
Next: task-only Conventional Commit after successful checks, then user acceptance. No further Luna delegation; persistent issues were resolved and verified directly.

## Review 2 — FAIL (cumulative direct review)

Fixed baseline/spec unchanged. Source changes show progress; `git diff --check` passes. Initial implementation and fix-attempt self-reported commands are not substitutes for the code-level acceptance evidence below.

### Standards

- **S1 remains P1 (partial).** `migrated_services_test.go` adds real service calls, but no rejection test exists for `UpdatePublication`, `EndPublication`, `CreateAssignment`, `DeleteAssignment`, `AutoAssignPublication`, or `GetCurrentRoster` (search across all new service test files); only success calls appear around lines 410/438/443/446/465/482. Audit every other method as well, not just these examples. The sole new business repository integration test only exercises position/template creation, publication single-row concurrency, availability upsert and disabled submission. It never calls assignment/override/shift-change/leave/attendance repository operations or their transaction/constraint races. The only Rota HTTP test still mocks positions. UUID/storage/transaction changes in those real repositories remain effectively untested. Frontend adds draft/grid/query tests but not the required lifecycle and leave/shift-change form validation regressions. Port baseline scheduling edge, slot DB, assignment, shift-change, leave, attendance, export, and critical handler tests with UUID fixtures; test success + business rejection and constraint/race paths. A 31-file admin pass is not full parity.
- **S2 remains P1 (partial/new regression within same verification finding).** CI and commands now point at api/admin, but `.github/workflows/ci.yml` replaces vulnerability checking with `govulncheck-blocked` and `if: ${{ false }}`. This disables the check for every future run based on a temporary local scanner/network issue, allowing green CI without scanning. Restore a real executable scanner job with a compatible pinned toolchain/scanner and normal failure propagation; keep inability to execute it in the verification record, not a permanent CI bypass. Include explicit API build verification. Do not run heavy checks locally: fix-attempt return reports local API tests/vet/build, which were not authorized as a Centaurus fallback.
- **S3 remains P2.** Design lines 65–73 still claim nullable FKs/write-time business snapshots, contrary to migration 13's retained UUID/deleted-identity view design. Status still claims acceptance complete and only govulncheck missing, despite S1. The latest checkpoint explicitly reuses r1/r2 browser evidence for r3 corrections; those earlier runs predate employee-nav/link/branding/cache fixes and cannot validate them. Record real current-source browser/HTTP acceptance of F1–F4 with retained reproducible script/test paths and clear achieved/missing parity; update descriptions without changing original requirements. Review 1 history should remain intact.

### Spec

- **F1 source fix observed:** employee Rota dashboard guard removed and shell exposes self-service links for all authenticated users. Still needs actual no-grant persona navigation/denial test, not merely route registration (`-rota.test.ts`). Track missing evidence under S1/S3.
- **F2 source fix observed:** email actions now use `/rota/requests` and `/rota/leaves/{id}` and EN/ZH text expectations updated. Follow generated links in current browser acceptance; missing evidence under S3.
- **F3 source fix observed:** migration 14, identity organization field, HTTP/application validation and template mail integrations added. Include non-empty organization save/reload/localization/current SMTP output in acceptance; missing browser evidence under S3.
- **F4 remains P1 (partial, precise residual).** Common principal query and cache-clear helper are added, but `admin/src/app/query-client.ts:7` only recognizes `isUnauthenticated`, which strictly requires `ApiProblemError`. Rota's actual fetch adapter (`features/rota/api.ts:49-53`) throws `RotaApiError(401, ...)`. Therefore a Rota query/mutation returning `UNAUTHENTICATED` bypasses cache cleanup entirely, leaving cached private business data until an unrelated common request/heartbeat notices it. `cache.test.ts` tests only helper calls, not this integration. Add an app-query-client regression that seeds A's private cache, rejects actual Rota read/write with 401, and asserts cleanup; retain data on non-auth 403/network errors as appropriate. Also add real logout/login B tests including delayed in-flight response and failed B refetch, and check role-change invalidation. Ensure transition cleanup completes before rendering another principal's protected views; don't erase the newly fetched authoritative user without repopulating it.
- **F5 remains P2 (partial).** Old top-level apps/deployments removed (good), but copied duplicate common systems remain in the new port: `api/internal/rota/email/email.go` still defines `NewSMTPEmailer`, SMTP transport and invitation/password/email-change builders with corresponding common templates/tests, while runtime is template mail dispatcher. `admin/src/features/rota/queries.ts` retains dead `createUser`, `updateUser`, `updateUserStatus`, `requestPasswordReset`, `updateBranding`, `/branding` fallback and other old common endpoints not mounted by the new router. Remove dead legacy common APIs/types/transport/templates and adapt tests to business-only mail rendering plus template common mail tests. Preserve active shift-change/leave notifications and template-owned identity/auth/mail management.

Review 2 totals: Standards 3 open findings (worst P1); Spec 2 open findings (worst P1), 3 source fixes observed pending cross-layer acceptance. No additional product scope is introduced.

## Corrective implementation pass 2 status (not a final review)

The second corrective pass addressed the historical findings without changing
the fixed baseline or authorized product scope:

- **S1:** Migrated service success/rejection coverage now spans every public
  service method, including current-publication lookup, lifecycle writes,
  assignment/roster, shift changes, leaves, attendance, export, and template
  slot operations. Stateful fakes are in
  `api/internal/rota/service/migrated_services_test.go` and boundary matrices
  are in `migrated_service_boundaries_test.go` and
  `migrated_service_method_boundaries_test.go`. Tagged PostgreSQL coverage in
  `repository_integration_test.go`,
  `repository_scheduling_integration_test.go`, and
  `migration_integration_test.go` exercises UUID identity, transactions,
  constraints, lock/race behavior, overrides, leaves, attendance, and outbox
  material. Admin coverage is 35 files / 185 tests, including Rota lifecycle
  contracts, schemas, drafts, grid projections, route permissions, and auth
  cache boundaries.
- **S2:** Active CI and root commands target `api/` and `admin/`; API build,
  vet, unit, serialized tagged integration, and admin lint/typecheck/test/build
  steps are wired. Integration package execution uses `go test -p 1` because
  the disposable database is shared by destructive package fixtures. The
  vulnerability job runs the pinned `govulncheck@v1.2.0` command normally; it
  is not disabled. The local scanner reports zero reachable vulnerabilities,
  while remote execution is blocked by dependency/database network timeouts.
- **S3:** Design/parity records now describe the retained UUID/deleted-identity
  view actually implemented, list the current test and command evidence, and
  state the browser evidence gap for the latest F1–F3 source fixes instead of
  inheriting stale r1/r2 claims. The local and remote verification caveats are
  explicit and no acceptance-complete or vulnerability-clean claim is made for
  unavailable evidence.
- **F4:** `isUnauthenticated` recognizes the actual Rota 401 envelope as well as
  template problem errors. Query and mutation regressions seed private Rota
  data and verify 401 cleanup while preserving data for 403/network failures;
  logout clears the protected cache before navigation and account-owner
  transitions clear it on switch.
- **F5:** Superseded top-level apps were already removed in the rebuild. The
  remaining copied Rota auth/account mail, password, email-change, SMTP,
  branding-write, and audit/translation surfaces were removed; only business
  notification rendering and a read-only adapter over Temvia's system identity
  remain in the Rota extension. No Rota model exposes `password_hash`.

The latest source-only checks pass locally and on isolated Centaurus as recorded
in the design/parity documents. Current r4 image acceptance passed the three
non-mail authentication/browser tests in `admin/e2e/auth.spec.ts`; the two
password-mail tests were skipped without Mailpit. Business browser acceptance
of F1–F3 was still required at that checkpoint; no final review or Git
commit had been performed.

## Review 3 — PASS after direct fixes and approved clarification

Main-agent cumulative review retained base `290be51287caa3831a3b8a14e196664964a2802d` and original scope. User explicitly authorized the employee role mapping; `rota.self` creates a valid employee role with own availability and no administrative grants, preserving template nonempty-role integrity and existing contextual rules.

### Standards

- S1 resolved: service method success/rejection matrices, real UUID scheduling/transaction/constraint integration suites, admin logic tests, and current-source business browser/API acceptance now run on the migrated apps. Added catalog red/green and actor-bound submission regression tests.
- S2 resolved: CI targets api/admin with an active pinned vulnerability job; final Go/admin checks and scanner ran successfully on Centaurus. Direct integration-runner hardening prevents existing-project reuse and application `.env` leakage; success and existing-volume rejection both verified.
- S3 resolved: latest evidence distinguishes historical failures from actual final checks, explains retained UUID history, and records 35 admin files/186 tests and 4 current business browser tests. Earlier local scanner results are not used as the remote evidence.

### Spec

- F1 resolved: employee-only `rota.self` role can authenticate and navigate self-service; valid own availability submit/read/delete succeeds, other-employee management is denied. No administrative grant is used to make employee login pass.
- F2 resolved: delivered Chinese leave and English pre-activation shift-change emails navigate to actual `/rota/...` pages. The original active-publication direct-change rejection remains intact.
- F3 resolved: organization name persists through template settings with validation/revision checks, English/Chinese UI and real SMTP-rendered notifications.
- F4 resolved: actual Rota 401 errors enter protected-cache cleanup; unit/cache regression tests and SPA A→logout→B with failing B detail fetch pass without rendering A's cached content.
- F5 resolved: superseded runnable applications and copied dead common Rota systems removed; common auth/roles/settings/mail remain template-owned.

Exact commands, safe remote setup/teardown, results and residual nonblocking advisories are in `temvia-rota-rebuild-final-verification.md`. Final totals: **Standards 0 open findings; Spec 0 open findings.** No additional review round or third Luna fix attempt.
