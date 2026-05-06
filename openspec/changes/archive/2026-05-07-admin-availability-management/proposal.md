## Why

Admins currently cannot inspect or correct employee availability submissions from the application. Once a publication reaches scheduling, a small correction requires direct database edits, which is risky and makes the scheduling workflow harder to operate during gray release.

## What Changes

- Add an admin-only availability management entry point under each publication and from the assignment board.
- Add a paginated, searchable employee availability table that includes active, publication-relevant employees even when they submitted zero cells.
- Add a single-employee availability editor that shows the publication template grid, lets admins draft changes locally, and saves the target availability set atomically.
- Allow admin availability reads in all publication states, while allowing admin writes only in `COLLECTING` and `ASSIGNING`.
- Validate every saved cell against the target employee's current qualifications and the publication template before committing the replacement.
- Audit every added or removed availability cell caused by an admin edit.
- Refactor publication child routes so the publication detail page, assignment board, availability management, and shift-change admin view render as standalone subpages instead of stacking child pages under the detail card.
- Preserve assignment behavior: availability edits affect future auto-assign candidate pools only and do not automatically mutate existing assignments.

## Non-goals

- No availability export, heatmap, or coverage analytics in this change.
- No per-slot reverse lookup showing all employees available for a specific cell.
- No multi-employee bulk edit workflow.
- No changes to employee self-service availability submission semantics.
- No fix to assignment draft atomicity, assignment history, or published-roster mutation strategy; those remain a separate scheduling change.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `scheduling`: Admins can list and edit employee availability for a publication through state-gated, qualification-validated, atomic per-user replacement APIs and standalone publication subpages.
- `audit`: The audit action taxonomy includes per-cell admin availability create and delete events emitted after successful admin replacements.

## Impact

- Backend publication routes, services, repositories, request/response DTOs, error handling, and tests.
- Audit action constants and audit metadata tests.
- Frontend publication routes, assignment-board entry actions, availability management pages, API query helpers, type definitions, i18n strings, and tests.
- OpenSpec deltas for `scheduling` and `audit`.
