# Design: Fix audit findings

## Overview

This is one cross-layer release-hardening task because all fixes share the same
production preview, timezone matrix, localization checks, and final release
gate. The task preserves existing API shapes and the UTC wall-clock scheduling
model while adding one stable duplicate-leave error contract.

## 1. Schedule-time boundary

Create a small frontend schedule-time module that owns the distinction between:

- **schedule wall-clock timestamps** — ISO timestamps whose UTC date/time fields
  are the business calendar values and must be formatted with
  `timeZone: "UTC"`;
- **event instants** — creation, decision, publication-window, and audit
  timestamps that keep the existing browser-local formatting.

The schedule-time module will provide:

- an `Intl.DateTimeFormat` constructor/helper that always adds
  `timeZone: "UTC"`;
- conversion from an ISO schedule timestamp to `datetime-local` by preserving
  its UTC fields;
- conversion from a schedule `datetime-local` value back to an ISO `Z`
  timestamp.

Leave preview/workbench/detail and attendance occurrence displays will use the
UTC formatter. Attendance inputs will use the UTC-preserving input helpers.
Created/decided timestamps will continue using normal localized formatters.

## 2. Duplicate leave invariant

### Database owner

Add migration `00022` with a partial unique index on
`shift_change_requests(requester_user_id, requester_assignment_id,
occurrence_date)` where `leave_id IS NOT NULL` and `state IN
('pending', 'approved')`.

Before creating the index, rank an `approved` leave-bearing row first when one
exists, then rank remaining rows by `created_at, id` within the unique key.
Mark every non-winning `pending` row `invalidated`, setting `decided_at` when
absent. This preserves an already-applied assignment transfer; groups containing
only pending rows preserve their earliest workflow.

The Down migration removes only the new index; invalidated legacy duplicates
are not resurrected.

### Repository/service/API flow

The unique violation occurs when `SetLeaveIDTx` turns the newly created
shift-change row into a leave-bearing row. Repository mapping converts that
constraint violation to the shared sentinel `ErrLeaveAlreadyExists`. The
transaction then rolls back the request and leave inserts together.

The service exports the sentinel, and the leave handler maps it to
`409 LEAVE_ALREADY_EXISTS`. The frontend API error union and both locale trees
receive the matching code.

### Preview behavior

The leave repository exposes the active occurrence keys for a user and
publication. `PreviewOccurrences` loads those keys once and filters matching
assignment/date rows before constructing direct-candidate data. The frontend
invalidates the exact preview query family after successful leave creation.

The database unique index remains authoritative under concurrency; preview
filtering is a user-experience optimization, not the correctness boundary.

## 3. Dependency remediation

Update the Go directive and production builder to Go 1.26.5. Upgrade Excelize
to 2.11.0 with Go module tooling locally so `go.mod` and `go.sum` remain the
source of truth. Preserve the existing schedule-export tests as the compatibility
contract and rerun `govulncheck` on Centaurus.

## 4. Attendance query states

Both attendance routes will branch in this order:

1. loading;
2. query error rendered through `getTranslatedApiError`;
3. successful empty result;
4. populated result.

The administrator page will not derive `shifts = []` into an empty state when
`dayQuery.isError`. The selected-shift query error will also be visible if the
summary request succeeded but detail loading failed.

Add `common.loading` to both locales and reuse the existing
`attendance.errors.*` catalog.

## 5. Responsive containment

Add `min-w-0` to `SidebarInset`, the flex item that owns authenticated content.
Wide data tables retain their existing `overflow-x-auto` containers. This is
preferred over page-specific width patches because the same flex minimum-size
behavior caused overflow on three unrelated routes.

## 6. Language consistency

- i18n initialization sets `<html lang>` immediately and on every
  `languageChanged` event.
- `DatePicker` derives both its display formatter and `react-day-picker` locale
  from `i18n.resolvedLanguage` (`zhCN` or `enUS`).
- Roster date-only formatting receives the active language and uses UTC to
  preserve the selected calendar date.
- Preference success handling awaits the language change and obtains the toast
  text from i18n after the new language is active.
- Sidebar title, description, trigger, and rail labels use shared locale keys.

## Compatibility and data migration

- No response fields or request bodies change.
- One new 409 error code is additive and only replaces the current accidental
  duplicate success/internal-error paths.
- Existing terminal leaves remain queryable and do not block resubmission.
- Legacy duplicate active requests are normalized during migration before the
  unique index is created.
- The migration is transaction-safe under Goose's statement block.

## Rollback

- Code can be rolled back independently after running migration Down to remove
  the partial unique index.
- Dependency rollback is a normal `go.mod`/`go.sum` and Dockerfile revert, but
  would reintroduce known vulnerabilities and is not an acceptable release
  state.
- UI fixes are isolated to shared formatting/i18n helpers and route state
  branches; no persisted data depends on them.
