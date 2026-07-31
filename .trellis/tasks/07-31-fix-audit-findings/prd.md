# Fix audit findings

## Goal

Remove the release-blocking correctness and security defects found during the
2026-07-31 repository and browser audit, then close the user-visible attendance,
responsive-layout, and Chinese-localization regressions that were reproduced in
the same audit.

## Background

- Backend build, vet, unit, race, and PostgreSQL integration tests passed, and
  frontend lint, build, and 313 tests passed, but real browser workflows exposed
  defects not represented by the current suites.
- Schedule occurrence timestamps are defined as UTC wall-clock values
  (`.trellis/spec/domain/scheduling.md:1343-1351`). In an Asia/Shanghai browser,
  a `08:00-10:00` leave occurrence was rendered as `16:00-18:00`.
- Two leave submissions for the same user, assignment, and occurrence date were
  accepted nine seconds apart. The database retained both workflows.
- `govulncheck` found four reachable vulnerabilities in Go 1.26.3 and
  Excelize 2.10.1.
- An attendance request returning
  `409 ATTENDANCE_RESPONSIBLE_REQUIRED` was rendered as an empty state after
  retries, and the loading state exposed the missing key `common.loading`.
- At tablet widths, `/users`, `/templates`, and `/publications` expanded the
  whole document horizontally instead of containing wide tables inside the
  content pane.
- Chinese pages retained `<html lang="en">`, some date controls used the host
  locale, and the preference-saved toast could use the language active before
  the save.

## Requirements

### R1: Preserve schedule wall-clock time in every browser timezone

- Concrete scheduling and attendance occurrence timestamps SHALL render using
  their UTC calendar and clock fields while still using the selected application
  language for localized text.
- A `2026-08-03T08:00:00Z` schedule occurrence SHALL display as 08:00 in both
  UTC and Asia/Shanghai browsers; it SHALL NOT display as 16:00 in
  Asia/Shanghai.
- Attendance `datetime-local` values SHALL round-trip through the API without a
  browser-timezone shift.
- True event instants such as `created_at`, `decided_at`, publication windows,
  and audit timestamps SHALL retain their existing instant/local-time behavior.

### R2: Reject duplicate active leave workflows

- At most one leave-bearing shift-change workflow may be active for the same
  `(requester_user_id, requester_assignment_id, occurrence_date)`.
- `pending` and `approved` workflows count as active.
- A concurrent or sequential duplicate create SHALL be rejected atomically with
  HTTP 409 and stable API code `LEAVE_ALREADY_EXISTS`.
- A workflow in `cancelled`, `rejected`, `expired`, or `invalidated` state SHALL
  not block a later resubmission for the same occurrence.
- Leave preview SHALL omit occurrences already covered by an active leave
  workflow so the normal UI does not continue offering a duplicate action.
- If pre-existing data contains multiple active workflows for one occurrence,
  migration SHALL preserve the approved workflow when one exists; otherwise it
  SHALL preserve the earliest pending workflow. All other pending duplicates
  SHALL become `invalidated` before installing the invariant.

### R3: Remove reachable dependency vulnerabilities

- The backend SHALL use Go 1.26.5 or a later compatible 1.26 patch release in
  `go.mod` and its production builder image.
- Excelize SHALL be upgraded to 2.11.0 or a later compatible release.
- Schedule Excel export behavior and existing workbook tests SHALL remain
  compatible.
- `govulncheck ./...` SHALL report no reachable vulnerabilities for backend
  application code.

### R4: Render attendance query failures as failures

- Leader and administrator attendance pages SHALL render translated API errors
  when their primary query fails.
- `ATTENDANCE_RESPONSIBLE_REQUIRED` SHALL be visible as its existing translated
  configuration error and SHALL never be represented as an empty attendance
  result.
- Loading, error, and empty states SHALL remain distinct, and both locale files
  SHALL define the shared loading copy they use.

### R5: Contain responsive content at tablet widths

- At 768px, 1024px, and 1440px viewport widths, the authenticated document SHALL
  not gain horizontal overflow on `/users`, `/templates`, or `/publications`.
- A wide table MAY scroll inside its own container without moving the sidebar,
  header, or whole document.
- Existing 390px mobile drawer behavior SHALL remain intact.

### R6: Keep application language consistent across UI and metadata

- The active i18n language SHALL be reflected by `document.documentElement.lang`
  on initial load and after a language change.
- Date picker labels and calendar text SHALL use the active application locale.
- Roster date-only labels SHALL use the active application locale without
  changing their calendar day.
- Saving a new language preference SHALL show the success toast in the newly
  selected language.
- Shared sidebar accessibility text SHALL be translated.

### R7: Add regression coverage at the owning boundaries

- Every new backend service behavior SHALL have a success path and at least one
  rejection/error path test.
- SQL uniqueness and resubmission semantics SHALL have PostgreSQL integration
  coverage.
- Frontend regression tests SHALL cover schedule-time formatting, attendance
  error rendering, locale metadata/date behavior, and responsive containment.
- Existing backend and frontend suites SHALL remain green.

## Acceptance Criteria

- [x] AC1 — A timezone regression test proves that an 08:00 UTC occurrence is
  displayed and submitted as 08:00 under an Asia/Shanghai browser timezone.
- [x] AC2 — Sequential and concurrent duplicate leave creation return
  `409 LEAVE_ALREADY_EXISTS`, while resubmission after a non-approved terminal
  state succeeds.
- [x] AC3 — Leave preview no longer returns an occurrence with an active leave.
- [x] AC4 — Go/Excelize versions are upgraded and Centaurus
  `govulncheck ./...` reports no reachable vulnerabilities.
- [x] AC5 — Attendance query failures render the translated server error;
  loading and empty states remain independently tested.
- [x] AC6 — Browser measurements show zero document-level horizontal overflow
  on the three audited admin pages at 768px, 1024px, and 1440px.
- [x] AC7 — Chinese mode sets `<html lang="zh">`, localizes date controls and
  roster date labels, translates sidebar accessibility copy, and displays the
  post-save toast in Chinese.
- [x] AC8 — Centaurus backend build, vet, unit, race, vulnerability, and
  integration checks pass.
- [x] AC9 — Centaurus frontend lint, unit tests, and production build pass, and
  the repaired workflows are re-exercised in the forwarded production preview.

## Constraints

- Work and Git operations remain local; source is synchronized one-way to
  Centaurus for build, test, and browser validation.
- Changes land directly on `main`, following the user's prior explicit branch
  preference for this repository.
- Existing API error shapes, publication lifecycle, permissions, and UTC
  occurrence model remain compatible.
- OpenSpec remains read-only legacy history.

## Out of Scope

- Resolving the separate contract conflict in the `stress` seed between an
  ACTIVE publication and pending regular shift-change fixtures.
- Frontend route-level code splitting or main-bundle size optimization.
- Changing the product's UTC wall-clock scheduling model to organization or
  browser timezones.
