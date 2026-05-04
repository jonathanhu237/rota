## Why

Rota can show the weekly assignment matrix in the browser, but admins cannot download the in-progress or final schedule as a spreadsheet for offline review, printing, or sharing. This is especially limiting while a publication is in `ASSIGNING`, where the partially-filled schedule and visible vacancies are useful for checking work before publication.

## What Changes

- Add a real `.xlsx` export for a publication's current baseline schedule assignments.
- Add a download endpoint for the schedule matrix, available to admins during `ASSIGNING`, `PUBLISHED`, and `ACTIVE`, and to employees during `PUBLISHED` and `ACTIVE`.
- Add frontend download controls on the admin assignment board and employee roster views.
- Export a single-sheet matrix with time rows and weekday columns, using localized sheet title, headers, vacancy text, and metadata labels.
- Include every scheduled `(slot, position)` block, every assigned user name, and one visible vacancy row per unfilled required headcount.
- Generate filenames on the client using the publication name, localized roster label, and a local timestamp.

## Non-goals

- Do not export CSV or an Excel-compatible plain text file; this change produces true `.xlsx`.
- Do not export a date-specific execution roster or expand the publication into every calendar week.
- Do not include leave-driven or occurrence-level temporary overrides in this export.
- Do not add a generic reporting or data-export framework.
- Do not expose assignment-board intermediate results to employees before publication.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `scheduling`: adds a publication schedule XLSX export contract, including visibility rules, matrix shape, localization inputs, and exclusion of occurrence-level temporary changes.

## Impact

- Backend API: adds an authenticated publication schedule download endpoint.
- Backend service/repository: reuses the baseline assignment-board shape or equivalent schedule snapshot to render the matrix without date-specific override expansion.
- Backend dependencies: adds an XLSX writer library.
- Frontend: adds download actions to the assignment board and roster pages, sends the current language to the export endpoint, and saves the returned blob with a localized timestamped filename.
- Tests: adds backend authorization/content tests and frontend helper/UI tests for download availability and filename generation.
