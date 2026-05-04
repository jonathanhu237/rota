## Context

The existing schedule surfaces are split by publication state and role:

- Admins use `GET /publications/{id}/assignment-board` during `ASSIGNING`, `PUBLISHED`, and `ACTIVE` to see and edit the baseline assignment matrix.
- Employees use `GET /publications/{id}/roster` and `GET /roster/current` only after the publication is `PUBLISHED` or `ACTIVE`; those reads are week-specific and apply occurrence-level overrides.
- The weekly roster UI already pivots the schedule into a time-by-weekday matrix, but there is no downloadable artifact.

The export requested here is the current baseline assignment result, not a date-specific execution roster. That means it follows the assignment-board data model more closely than the existing weekly roster endpoint: it includes the latest direct admin edits to `assignments`, but it does not expand all calendar weeks and does not apply leave or occurrence-level shift-change overrides.

## Goals / Non-Goals

**Goals:**

- Provide a true `.xlsx` download for the current baseline schedule matrix.
- Allow admins to export in-progress schedules during `ASSIGNING`.
- Allow employees to export only after the schedule is visible to them in `PUBLISHED` or `ACTIVE`.
- Preserve vacancies in the export so partial schedules remain useful.
- Keep one backend implementation and one file format across admin and employee entry points.

**Non-Goals:**

- No database schema changes.
- No CSV export.
- No export of every concrete week in the publication active window.
- No occurrence-level override, leave, or actual-execution report.
- No generic export framework for other resources.

## Decisions

### D-1. Add an authenticated publication schedule XLSX endpoint

Add:

```text
GET /publications/{id}/schedule.xlsx
```

The route uses `RequireAuth`, then service-layer authorization checks the viewer and publication effective state:

- admin + `ASSIGNING`, `PUBLISHED`, or `ACTIVE`: allowed
- employee + `PUBLISHED` or `ACTIVE`: allowed
- everyone else: rejected

Rejected alternative: `RequireAdmin` plus a separate employee roster export endpoint. That would duplicate export logic and make it easier for the two downloads to drift.

Error codes used by this endpoint:

- `UNAUTHORIZED` (401): existing session middleware behavior.
- `INVALID_REQUEST` (400): invalid publication id or unsupported export language.
- `PUBLICATION_NOT_FOUND` (404): publication id does not exist.
- `PUBLICATION_NOT_ACTIVE` (409): the publication state is not export-visible for the viewer.
- `INTERNAL_ERROR` (500): unexpected XLSX generation or write failure.

### D-2. Export baseline assignments, not weekly roster rows

The export should use a service method such as `ExportScheduleXLSX(ctx, publicationID, viewer, opts)` that loads the publication, resolves its effective state, applies the state/role visibility rule, then builds an export model from the baseline assignment-board shape or an equivalent repository query.

This intentionally does not call `GetPublicationRoster(..., weekStart)` because roster reads are date-specific and apply occurrence-level overrides. The export should reflect direct admin edits to `assignments`; those are the user's "modified table". It should not reflect temporary leave replacements.

Rejected alternative: call the weekly roster endpoint for a representative week. That would make the export depend on date-window selection and would pull in occurrence override semantics that do not belong to this file.

### D-3. Generate XLSX on the backend with `excelize/v2`

Use a backend XLSX writer dependency, preferably `github.com/xuri/excelize/v2`, to create the workbook. The implementation should keep the dependency behind a small schedule-export renderer so tests can assert the service authorization separately from workbook styling.

Rejected alternatives:

- Browser-side XLSX generation with SheetJS or a similar frontend package: duplicates role/state enforcement concerns across UI entry points and sends formatting logic into the browser.
- CSV: rejected because the target artifact is a formatted matrix with multi-line cells, metadata, headers, and basic styling.

### D-4. Workbook shape

Create one sheet named by language:

- `zh`: `排班表`
- `en`: `Roster`

The sheet contains compact metadata rows, then the matrix:

```text
<publication name>
状态: <localized effective state>
导出时间: <localized generation timestamp>

时间 | 周一 | 周二 | 周三 | 周四 | 周五 | 周六 | 周日
09:00-12:00 | <cell> | <cell> | ...
```

Rows are sorted by `start_time ASC, end_time ASC`. Columns are fixed Monday through Sunday. Each matrix cell contains zero or more position blocks for that `(time range, weekday)`:

```text
<position name> (<required headcount>)
<assigned user name>
<assigned user name>
<vacancy label>
```

If a position has two vacancies, the vacancy label appears twice. Empty scheduled positions are preserved. Off-schedule cells remain blank.

When the XLSX library supports the necessary rich text in a maintainable way, position labels should be bold and vacancy lines should use a muted/italic style. If rich text inside a wrapped cell becomes brittle, the minimum acceptable styling is bold headers, wrapped text, borders, sensible column widths, and preserved vacancy text.

### D-5. Localization and filenames

The frontend passes the current UI language to the endpoint using a simple query parameter, for example:

```text
GET /publications/{id}/schedule.xlsx?lang=zh
```

Supported values are `zh` and `en`. If omitted, the backend falls back to the viewer's language preference if present, then English. Unsupported values are rejected as `INVALID_REQUEST`.

The backend localizes workbook labels and state names. The frontend generates the downloaded filename using the same language and one client-local timestamp:

```text
<publication name>-<localized roster label>-YYYYMMDD-HHmm.xlsx
```

Filename-unsafe characters are replaced with `-`. The backend should still return `Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` and a generic attachment `Content-Disposition` as a fallback; the frontend blob save controls the user-facing filename.

### D-6. Frontend entry points

Add a download button with a download icon:

- Admin assignment board header: visible and enabled when the publication state is `ASSIGNING`, `PUBLISHED`, or `ACTIVE`.
- Roster page header: visible when a roster publication is present, for both employees and admins.

The frontend should call the backend endpoint as a blob, disable the button while the request is pending, and show a localized destructive toast if the download fails.

## Risks / Trade-offs

- XLSX styling can become implementation-heavy -> keep styling restrained and make content correctness the acceptance baseline.
- Employee export endpoint shares an assignment-board-like data source -> ensure the export model only includes names and never serializes emails into the workbook.
- `PUBLICATION_NOT_ACTIVE` is reused for "not export-visible" states -> acceptable because the existing scheduling API already uses this code for roster visibility; document it in the spec.
- Backend and frontend timestamps can differ slightly -> the filename uses client-local time; the workbook metadata records backend generation time and is informational only.

## Migration Plan

1. Add the backend dependency and renderer.
2. Add service and handler support behind the new route.
3. Add frontend query/blob download helpers and buttons.
4. Run backend and frontend checks.

Rollback is straightforward: remove the route, renderer, dependency, frontend buttons, and spec delta. There are no database migrations or persisted data changes.

## Open Questions

None. The export scope, permissions, workbook shape, localization behavior, and non-goals were resolved during exploration.
