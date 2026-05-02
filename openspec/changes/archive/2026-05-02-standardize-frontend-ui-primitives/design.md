## Context

The frontend currently has two competing UI patterns for ordinary admin workflows:

- Users, Positions, Templates, and Publications render hand-styled HTML tables with repeated loading, empty, pagination, and action markup.
- Publication windows, leave ranges, and template slot times use native browser date/time inputs directly inside feature components.

The project already uses shadcn-style primitives under `frontend/src/components/ui`, Base UI for overlays, TanStack Query/Router for data and navigation, and localized React components. The change should standardize common UI surfaces without changing backend APIs, publication state behavior, leave preview semantics, or schedule matrix rendering.

## Goals / Non-Goals

**Goals:**

- Add shadcn-compatible Table, Popover, and Calendar primitives using the existing component style and alias conventions.
- Add a lightweight shared DataTable renderer backed by TanStack Table for ordinary record lists.
- Migrate Users, Positions, Templates, and Publications list tables to the shared table foundation while preserving current columns, row actions, pagination, loading, and empty states.
- Add shared DatePicker, TimePicker, and DateTimePicker wrappers that preserve the existing external string formats.
- Replace the current native date/time fields in publication, leave, and template-slot forms with the wrappers without changing API payload formats.

**Non-Goals:**

- No backend endpoint, schema, migration, state-machine, or error-code changes.
- No sorting, filtering, search, column visibility, row selection, export, or bulk-action UI.
- No migration of assignment board, roster, availability, or shift-change detail matrices/tables.
- No timezone model changes, custom time menus, natural-language parsing, or date-range preset UI.

## Decisions

### Use TanStack Table as the table state engine

The shared DataTable renderer will accept a `Table<TData>` instance from `@tanstack/react-table` and render it through shadcn Table primitives. Feature components will own their column definitions and call `useReactTable`, keeping row actions, query state, and route behavior close to the business feature.

Rationale:

- shadcn's Data Table pattern is built around TanStack Table, so adopting it now avoids a second migration when sorting, filtering, visibility, and selection are added later.
- TanStack Table can run in manual mode, which matches Rota's existing server-owned pagination and future server-owned sorting/filtering needs.
- Keeping column definitions in feature modules avoids a generic table abstraction that has to understand every feature's dialogs, permissions, and localized labels.

Rejected alternatives:

- Continue with raw HTML tables: lower dependency cost, but duplicates state/rendering patterns and makes future table features harder to add consistently.
- Use shadcn Table primitives only, without TanStack Table: improves styling but does not solve table state, column model, or future manual sorting/filtering.
- Use a heavier grid library: adds more behavior than needed for Rota's simple CRUD lists and risks conflicting with the existing shadcn styling direction.

### Keep DataTable rendering deliberately small

The shared DataTable component will render headers, rows, cells, loading state, empty state, and pagination controls. It will not fetch data, manage dialogs, mutate rows, or render sorting/filtering/search controls in this change.

Rationale:

- Current list pages already own TanStack Query calls and mutation side effects; moving those into DataTable would couple unrelated business flows.
- A narrow renderer gives the four migrated list tables consistent structure without forcing future table features into the first phase.

Rejected alternatives:

- Build one high-level CRUD table component: would centralize too much feature-specific behavior and make permissions/actions harder to reason about.
- Add sorting/filtering/search immediately: user-visible behavior would expand the change beyond UI standardization and require backend/API decisions.

### Treat schedule matrices as out of scope

Assignment board, roster, and availability grids will remain custom matrix components. They may continue using table-like markup where appropriate, but they will not use the shared DataTable abstraction.

Rationale:

- Those surfaces are two-dimensional scheduling matrices, not record lists.
- Their cells contain domain-specific interactions, drag/drop, coverage status, or weekday/time grouping that does not map cleanly to ordinary row/column table behavior.

Rejected alternative:

- Force all `<table>` usage through DataTable: this would make matrix UI harder to maintain and blur the boundary between record lists and scheduling layouts.

### Add shadcn Calendar/Popover primitives and thin date/time wrappers

The project will add shadcn-compatible `Calendar` and `Popover` primitives, then expose `DatePicker`, `TimePicker`, and `DateTimePicker` wrappers for feature forms. The wrappers will accept and emit the same strings used today:

- `DatePicker`: `YYYY-MM-DD`
- `TimePicker`: `HH:MM`
- `DateTimePicker`: `YYYY-MM-DDTHH:mm`

The date wrappers will treat values as local calendar/time strings for display and selection. API-specific conversion remains in the existing feature layer, such as publication forms converting local datetime strings to ISO/RFC3339 before submission.

Rationale:

- shadcn's date examples compose Calendar and Popover, so a project wrapper prevents every feature form from duplicating parsing, formatting, and empty-value handling.
- Keeping string values at the wrapper boundary avoids changing form schemas, validation code, query parameters, and API clients.
- Using a styled time input for `TimePicker` gives visual consistency without inventing a custom time-selection component.

Rejected alternatives:

- Use native date/time inputs directly: preserves behavior but keeps inconsistent UI and duplicated feature-level input handling.
- Use `Date` objects as form values: creates timezone ambiguity and forces schema/API changes across existing code.
- Build a custom time menu: higher implementation and accessibility cost without current product need.

### Add `react-day-picker` and `date-fns` for Calendar behavior

The shadcn Calendar primitive will use `react-day-picker` for accessible calendar interaction and `date-fns` for local date parsing/formatting helpers where needed.

Rationale:

- These are the dependencies used by the shadcn Calendar pattern and avoid hand-rolling keyboard and date-grid behavior.
- The project already accepts focused UI dependencies when they carry real interaction complexity.

Rejected alternatives:

- Hand-roll a calendar: high accessibility and edge-case risk for little benefit.
- Use only the browser's native date input: inconsistent appearance and no shared Calendar/Popover primitive.
- Add a full date-time picker suite: more behavior and styling surface than the current forms require.

## Risks / Trade-offs

- Dependency growth from TanStack Table, React Day Picker, and date-fns -> keep usage narrow and tied to shadcn-compatible primitives so the value is explicit.
- Wrapper parsing could accidentally change timezone semantics -> keep external values as strings and leave ISO conversion in existing publication code.
- Table migration could drop row actions or pagination edge behavior -> migrate one table shape at a time and cover row actions, empty/loading states, and page changes in component tests.
- DataTable may be over-generalized too early -> keep it as a renderer around a caller-owned TanStack table instance, not as a CRUD framework.
- Calendar interactions may be harder to test than native inputs -> write focused wrapper tests around emitted string values and feature tests around preserved payload/query formats.

## Migration Plan

1. Add dependencies and shadcn-compatible primitives.
2. Implement shared DataTable and date/time wrappers with focused tests.
3. Migrate Users and Positions tables, then Templates and Publications tables, preserving current behavior.
4. Replace date/time controls in publication, leave, and template-slot forms, preserving string values and API conversion points.
5. Run frontend lint, test, and build plus OpenSpec validation.

Rollback is straightforward because no backend schema or API changes are involved: revert the frontend dependency and component changes, and the previous raw table/native input components can be restored from git.

## Open Questions

None. Sorting, filtering, column visibility, row selection, and richer date-range behavior are intentionally deferred to later scoped changes.
