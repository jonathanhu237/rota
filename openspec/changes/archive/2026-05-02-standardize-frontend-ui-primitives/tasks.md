## 1. Setup and Shared Primitives

- [x] 1.1 Start implementation from `main` on branch `change/standardize-frontend-ui-primitives`. Verify: `git status --short --branch`
- [x] 1.2 Add frontend dependencies `@tanstack/react-table`, `react-day-picker`, and `date-fns`, updating the lockfile. Verify: `cd frontend && pnpm install --frozen-lockfile`
- [x] 1.3 Add shadcn-compatible `Table`, `Popover`, and `Calendar` UI primitives under `frontend/src/components/ui`. Verify: `test -f frontend/src/components/ui/table.tsx && test -f frontend/src/components/ui/popover.tsx && test -f frontend/src/components/ui/calendar.tsx`
- [x] 1.4 Implement the shared DataTable renderer around caller-owned TanStack table instances, including loading, empty, and pagination rendering without sorting/filtering/search controls. Verify: `cd frontend && pnpm lint`
- [x] 1.5 Add focused DataTable tests for row rendering, loading state, empty state, and manual pagination callbacks. Verify: `cd frontend && pnpm test -- data-table`

## 2. Ordinary List Table Migration

- [x] 2.1 Migrate the Users list to define TanStack columns in the users feature and render through the shared DataTable while preserving actions, loading, empty, and server pagination behavior. Verify: `cd frontend && pnpm test -- users-table`
- [x] 2.2 Update or add Users table tests for columns, row actions, empty/loading states, and page-change behavior. Verify: `cd frontend && pnpm test -- users-table`
- [x] 2.3 Migrate the Positions list to define TanStack columns in the positions feature and render through the shared DataTable while preserving actions, loading, empty, and server pagination behavior. Verify: `cd frontend && pnpm test -- positions-table`
- [x] 2.4 Add Positions table tests for columns, row actions, empty/loading states, and page-change behavior. Verify: `cd frontend && pnpm test -- positions-table`
- [x] 2.5 Migrate the Templates list to define TanStack columns in the templates feature and render through the shared DataTable while preserving detail navigation, loading, empty, and server pagination behavior. Verify: `cd frontend && pnpm test -- templates-table`
- [x] 2.6 Add Templates table tests for columns, detail navigation, empty/loading states, and page-change behavior. Verify: `cd frontend && pnpm test -- templates-table`
- [x] 2.7 Migrate the Publications list to define TanStack columns in the publications feature and render through the shared DataTable while preserving state badges, actions, loading, empty, and server pagination behavior. Verify: `cd frontend && pnpm test -- publications-table`
- [x] 2.8 Update Publications table tests for columns, state badges, row actions, empty/loading states, and page-change behavior. Verify: `cd frontend && pnpm test -- publications-table`
- [x] 2.9 Confirm assignment board, roster, availability, and publication shift-change review screens are not migrated to DataTable. Verify: `! rg -n "DataTable" frontend/src/components/assignments frontend/src/components/availability frontend/src/components/roster 'frontend/src/routes/_authenticated/publications/$publicationId/shift-changes.tsx'`

## 3. Date and Time Controls

- [x] 3.1 Implement shared `DatePicker`, `TimePicker`, and `DateTimePicker` wrappers that accept and emit `YYYY-MM-DD`, `HH:MM`, and `YYYY-MM-DDTHH:mm` strings respectively. Verify: `cd frontend && pnpm lint`
- [x] 3.2 Add wrapper tests for value parsing, emitted formats, empty-string handling, and TimePicker's styled time-input behavior. Verify: `cd frontend && pnpm test -- date-picker time-picker date-time-picker`
- [x] 3.3 Replace create-publication and publication-detail datetime inputs with `DateTimePicker`, preserving local-form values and ISO/RFC3339 API conversion. Verify: `cd frontend && pnpm test -- create-publication-dialog publications`
- [x] 3.4 Add or update publication datetime tests covering unchanged create and planned-active payload formats. Verify: `cd frontend && pnpm test -- create-publication-dialog publications`
- [x] 3.5 Replace leave request date inputs with `DatePicker`, preserving `from=YYYY-MM-DD` and `to=YYYY-MM-DD` preview query parameters. Verify: `cd frontend && pnpm test -- leaves`
- [x] 3.6 Add or update leave request tests covering unchanged date query parameters and validation behavior. Verify: `cd frontend && pnpm test -- leaves`
- [x] 3.7 Replace template slot start/end time inputs with `TimePicker`, preserving `start_time` and `end_time` as `HH:MM`. Verify: `cd frontend && pnpm test -- template-slot-dialog`
- [x] 3.8 Add or update template slot tests covering unchanged time payload format and validation behavior. Verify: `cd frontend && pnpm test -- template-slot-dialog`

## 4. Final Verification

- [x] 4.1 Run frontend checks. Verify: `cd frontend && pnpm lint && pnpm test && pnpm build`
- [x] 4.2 Validate this OpenSpec change. Verify: `openspec validate standardize-frontend-ui-primitives --strict`
- [x] 4.3 Validate the full OpenSpec tree. Verify: `openspec validate --all --strict`
- [x] 4.4 Confirm all task checkboxes are completed before reporting apply done. Verify: `! rg -n "^- \\[ \\]" openspec/changes/standardize-frontend-ui-primitives/tasks.md`
