## ADDED Requirements

### Requirement: Publication schedule XLSX export visibility

`GET /publications/{id}/schedule.xlsx` SHALL return a true `.xlsx` workbook containing the publication's current baseline schedule matrix. The endpoint SHALL require `RequireAuth`.

The endpoint SHALL allow administrators to export a publication whose effective state is `ASSIGNING`, `PUBLISHED`, or `ACTIVE`. The endpoint SHALL allow non-admin employees to export a publication only when its effective state is `PUBLISHED` or `ACTIVE`.

Export requests for `DRAFT` or `COLLECTING` publications SHALL be rejected with HTTP 409 and error code `PUBLICATION_NOT_ACTIVE`. Export requests by non-admin employees for an `ASSIGNING` publication SHALL also be rejected with HTTP 409 and error code `PUBLICATION_NOT_ACTIVE`.

The endpoint SHALL accept an optional `lang` query parameter. `lang=zh` and `lang=en` SHALL be supported. If `lang` is absent, the backend SHALL use the viewer's language preference when present, otherwise English. Unsupported `lang` values SHALL be rejected with HTTP 400 and error code `INVALID_REQUEST`.

#### Scenario: Admin exports an assigning schedule

- **GIVEN** an authenticated administrator and a publication whose effective state is `ASSIGNING`
- **WHEN** the administrator calls `GET /publications/{id}/schedule.xlsx?lang=zh`
- **THEN** the response is HTTP 200
- **AND** the response content type is `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`
- **AND** the response body is a valid `.xlsx` workbook

#### Scenario: Employee cannot export an assigning schedule

- **GIVEN** an authenticated non-admin employee and a publication whose effective state is `ASSIGNING`
- **WHEN** the employee calls `GET /publications/{id}/schedule.xlsx`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_ACTIVE`

#### Scenario: Employee exports a published schedule

- **GIVEN** an authenticated non-admin employee and a publication whose effective state is `PUBLISHED`
- **WHEN** the employee calls `GET /publications/{id}/schedule.xlsx?lang=en`
- **THEN** the response is HTTP 200
- **AND** the response body is a valid `.xlsx` workbook

#### Scenario: Draft schedule export is refused

- **GIVEN** an authenticated administrator and a publication whose effective state is `DRAFT`
- **WHEN** the administrator calls `GET /publications/{id}/schedule.xlsx`
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_ACTIVE`

#### Scenario: Unsupported export language is rejected

- **GIVEN** an authenticated user and an export-visible publication
- **WHEN** the user calls `GET /publications/{id}/schedule.xlsx?lang=fr`
- **THEN** the response is HTTP 400 with error code `INVALID_REQUEST`

### Requirement: Publication schedule XLSX workbook content

The schedule export workbook SHALL contain exactly one sheet. The sheet name, header labels, state label, exported-at label, weekday labels, and vacancy label SHALL be localized according to the resolved export language.

The workbook SHALL represent the publication's current baseline `assignments` snapshot. Direct administrator assignment edits SHALL be reflected in subsequent exports. Leave-driven or occurrence-level shift-change overrides SHALL NOT be applied.

The matrix SHALL use one row per distinct time range, sorted by `(start_time, end_time)` ascending, and fixed weekday columns Monday through Sunday. Each scheduled `(time range, weekday)` cell SHALL include every position configured for that slot. Each position block SHALL include the position name, the required headcount, each assigned user's name on its own line, and one localized vacancy line per unfilled required headcount. User email addresses SHALL NOT be included in the workbook.

#### Scenario: Workbook contains localized metadata and one sheet

- **GIVEN** an export-visible publication named `Spring Rota`
- **WHEN** a caller downloads `GET /publications/{id}/schedule.xlsx?lang=en`
- **THEN** the workbook contains exactly one sheet named `Roster`
- **AND** the sheet includes the publication name
- **AND** the sheet includes localized state and exported-at metadata labels
- **AND** the matrix header includes `Time`, `Mon`, `Tue`, `Wed`, `Thu`, `Fri`, `Sat`, and `Sun`

#### Scenario: Matrix preserves vacancies and one name per line

- **GIVEN** a scheduled Monday slot from `09:00` to `12:00`
- **AND** the slot has position `Front Desk` with `required_headcount = 3`
- **AND** two users named `Alice` and `Bob` are assigned to that `(slot, weekday, position)`
- **WHEN** a caller downloads the schedule workbook in English
- **THEN** the `09:00-12:00` / `Mon` cell includes `Front Desk (3)`
- **AND** the same cell includes `Alice`, `Bob`, and `Empty` on separate lines
- **AND** the same cell includes exactly one `Empty` line for the missing headcount

#### Scenario: Empty scheduled position is still exported

- **GIVEN** a scheduled Tuesday slot has position `Cashier` with `required_headcount = 2`
- **AND** no users are assigned to that `(slot, weekday, position)`
- **WHEN** a caller downloads the schedule workbook
- **THEN** the Tuesday cell for that slot includes `Cashier (2)`
- **AND** the cell includes two localized vacancy lines

#### Scenario: Occurrence-level override is not applied

- **GIVEN** a baseline assignment assigns `Alice` to a Monday slot
- **AND** an occurrence-level override assigns `Bob` to that baseline assignment for one concrete Monday date
- **WHEN** a caller downloads the schedule workbook
- **THEN** the workbook lists `Alice` for the Monday slot
- **AND** the workbook does not replace `Alice` with `Bob`

#### Scenario: User emails are omitted

- **GIVEN** a scheduled slot assignment for user `Alice` whose email is `alice@example.com`
- **WHEN** a caller downloads the schedule workbook
- **THEN** the workbook includes `Alice`
- **AND** the workbook does not include `alice@example.com`

### Requirement: Schedule XLSX download controls

The frontend SHALL provide schedule XLSX download controls for export-visible publications. The admin assignment board SHALL show the control when the publication state is `ASSIGNING`, `PUBLISHED`, or `ACTIVE`. The roster page SHALL show the control when a roster publication is present. The frontend SHALL pass the current UI language to the export endpoint.

The frontend SHALL save the returned workbook using a client-local timestamped filename:

```text
<publication name>-<localized roster label>-YYYYMMDD-HHmm.xlsx
```

Filename-unsafe characters SHALL be replaced with `-`.

#### Scenario: Admin assignment board shows download in assigning state

- **GIVEN** an authenticated administrator viewing the assignment board for an `ASSIGNING` publication
- **WHEN** the page renders
- **THEN** the page shows an enabled localized `Download Excel` control

#### Scenario: Roster page saves a localized timestamped filename

- **GIVEN** a user viewing a visible roster publication named `Spring/Rota`
- **AND** the user's interface language is English
- **WHEN** the user downloads the schedule workbook at local time `2026-05-04 15:30`
- **THEN** the frontend requests `/publications/{id}/schedule.xlsx?lang=en`
- **AND** the downloaded filename is `Spring-Rota-roster-20260504-1530.xlsx`
