# Implementation plan: Fix audit findings

## Phase A — Contracts and backend correctness

- [x] Update scheduling contract with active-leave uniqueness, terminal
  resubmission, preview filtering, and `LEAVE_ALREADY_EXISTS`.
- [x] Add migration `00022` to normalize legacy active duplicates and install
  the partial unique index.
- [x] Add the shared duplicate-leave sentinel and repository constraint mapping.
- [x] Map `LEAVE_ALREADY_EXISTS` to HTTP 409 in the leave handler.
- [x] Filter active leave occurrences from preview.
- [x] Add service and handler success/rejection tests.
- [x] Add PostgreSQL integration tests for uniqueness, concurrency, terminal
  resubmission, and preview-key reads.

## Phase B — Schedule-time and attendance UI

- [x] Add shared UTC schedule-time formatting/input helpers with timezone
  regression tests.
- [x] Apply schedule-time formatting to leave and attendance occurrence
  displays without changing event-instant formatting.
- [x] Round-trip leader/admin attendance `datetime-local` values as UTC
  wall-clock timestamps.
- [x] Add explicit translated error branches to leader/admin attendance pages,
  including selected-shift detail failures.
- [x] Add `common.loading` and `LEAVE_ALREADY_EXISTS` in both locale trees.
- [x] Add/adjust route tests for loading, error, empty, and timezone behavior.

## Phase C — Responsive and i18n consistency

- [x] Add shared flex containment to `SidebarInset` and a regression assertion.
- [x] Synchronize `<html lang>` on initialization and language changes.
- [x] Localize DatePicker display/calendar and roster date-only labels.
- [x] Make preference success toast use the newly selected language.
- [x] Localize sidebar accessibility/title copy.
- [x] Add i18n, date picker, preferences, roster, and sidebar tests.

## Phase D — Dependency remediation

- [x] Upgrade Go directive and production builder from 1.26.3 to 1.26.5.
- [x] Upgrade Excelize from 2.10.1 to 2.11.0 and refresh Go sums locally.
- [x] Verify schedule-export workbook tests remain unchanged in behavior.

## Local review gate

- [x] Search all `occurrence_start`, `scheduled_start`, `datetime-local`,
  `Intl.DateTimeFormat`, `LEAVE_ALREADY_EXISTS`, and affected translation-key
  consumers.
- [x] Confirm normal event instants were not accidentally forced to UTC.
- [x] Confirm task requirements and changed domain contracts agree.
- [x] Confirm `git diff --check` is clean.

## Centaurus validation

- [x] Rsync the local source one-way to `/home/jonathanhu237/code/rota`,
  excluding Git metadata, local secrets, and dependency caches.
- [x] Backend: `go build ./...`, `go vet ./...`, `go test -count=1 ./...`,
  and `go test -race -count=1 ./...`.
- [x] Backend SQL: run the isolated integration test command with all
  migrations and integration-tagged tests.
- [x] Security: run current `govulncheck ./...` and require no reachable
  findings.
- [x] Frontend: `pnpm lint`, `pnpm test`, and `pnpm build`.
- [x] Production preview: reseed isolated scenarios, exercise duplicate leave
  rejection/resubmission, verify 08:00 remains 08:00 in Asia/Shanghai, and
  verify attendance API errors render as errors.
- [x] Measure document `scrollWidth - clientWidth` at 768/1024/1440 on users,
  templates, and publications; require zero.
- [x] Verify Chinese `<html lang>`, DatePicker, roster dates, preference toast,
  sidebar labels, and browser console.

## Validation evidence

- Centaurus backend: build, vet, unit, race, full PostgreSQL integration, and
  the migration-specific legacy-normalization test passed with Go 1.26.5.
- Migration `00022` passed Up/Down/Up at version 22. Legacy fixtures preserved
  an approved workflow over an older pending row and preserved the earliest row
  when both duplicates were pending.
- Centaurus `govulncheck ./...` reported no reachable vulnerabilities.
- Centaurus frontend: lint, 69 test files / 324 tests, and production build
  passed; the timezone helper also passed with `TZ=Asia/Shanghai`.
- Forwarded production preview: duplicate create returned
  `409 LEAVE_ALREADY_EXISTS`, cancellation allowed resubmission, an active
  occurrence was absent from preview, and an `08:00–10:00` occurrence remained
  `08:00–10:00`.
- Browser: attendance 409 rendered its Chinese configuration error; all nine
  768/1024/1440 admin-page measurements had zero document overflow; 390px
  navigation opened; `<html lang>`, DatePicker/roster/sidebar labels, and the
  post-save toast were Chinese; console had no warnings or errors.

## Finish

- [x] Run `trellis-check` full-scope review.
- [x] Update durable Trellis specs with final executable contracts.
- [x] Check every acceptance criterion in `prd.md` and every box above.
- [x] Commit the implementation on `main` using Conventional Commits.
- [x] Run `trellis-finish-work` and archive the task record.
