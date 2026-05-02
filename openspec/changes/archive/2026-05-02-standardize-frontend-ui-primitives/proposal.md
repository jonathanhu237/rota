## Why

Rota's admin and employee screens currently mix hand-styled tables with native browser date/time inputs, which makes common workflows feel inconsistent across users, positions, templates, publications, leave, and template-slot editing. Standardizing these surfaces on shadcn UI primitives gives the app one table and date/time interaction language before additional table features such as sorting, filtering, and column controls are added.

## What Changes

- Add shadcn-compatible table primitives and a lightweight DataTable renderer built around TanStack Table.
- Migrate the top-level Users, Positions, Templates, and Publications list tables to the shared DataTable foundation while preserving current pagination, row actions, empty states, and API calls.
- Design the DataTable foundation for server/manual table state from the start so future sorting and filtering can be routed through query params and backend list APIs without client-only page-local behavior.
- Add shadcn-compatible Popover and Calendar primitives, plus thin project wrappers for `DatePicker`, `TimePicker`, and `DateTimePicker`.
- Replace current native date/time inputs in publication window forms, leave date-range entry, and template slot time entry with the new wrappers.
- Preserve existing form value formats and API payloads: date values remain `YYYY-MM-DD`, time values remain `HH:MM`, datetime values remain `YYYY-MM-DDTHH:mm`, and publication submissions still convert local datetimes to ISO/RFC3339 before reaching the backend.

## Non-goals

- Do not add sorting, filtering, column visibility controls, row selection, or search UI in this change.
- Do not migrate the assignment board grid, roster grid, or availability grid to DataTable; these are schedule matrices, not ordinary record tables.
- Do not change backend APIs, pagination response shapes, publication window semantics, leave preview query parameters, or template slot payload formats.
- Do not introduce custom time menus, natural-language date parsing, range picker presets, or date-specific roster navigation.
- Do not change the application's timezone model or add multi-timezone support.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `frontend-shell`: Standardizes ordinary list tables and date/time form controls on shared shadcn-style primitives while preserving existing route behavior and API contracts.

## Impact

- Frontend dependencies: add TanStack Table for DataTable state, plus the dependencies required by the shadcn Calendar primitive (`react-day-picker` and `date-fns`).
- Frontend UI primitives: add `components/ui/table.tsx`, `components/ui/popover.tsx`, and `components/ui/calendar.tsx`.
- Frontend shared components: add a reusable DataTable renderer and date/time picker wrappers.
- Frontend feature areas: update Users, Positions, Templates, Publications, leave request creation, and template slot dialogs.
- Tests: add focused tests for table rendering behavior, value-format preservation, and unchanged API payload/query parameter formats.
