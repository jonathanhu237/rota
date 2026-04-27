## 1. Pivot helper

- [x] 1.1 Add `frontend/src/components/roster/roster-grid-cells.ts` exporting `pivotRosterIntoGridCells(weekdays: RosterWeekday[]): { timeBlocks, weekdays, cells }` per design D-5. Each cell is `ScheduledRosterCell` (with `kind: "scheduled"`, `weekday`, `slot`, `occurrence_date`, `positions`, `totals`) or `OffScheduleRosterCell` (with `kind: "off-schedule"`, `weekday`, `timeBlockIndex`). Sort time blocks ascending by `(start_time, end_time)`. Verify by `pnpm tsc --noEmit`.
- [x] 1.2 Add `frontend/src/components/roster/roster-grid-cells.test.ts`: distinct (start_time, end_time) tuples produce one row each; weekdays without a matching slot produce off-schedule cells; cell `totals.status` correctly classified as `full` / `partial` / `empty`. Verify by `pnpm test roster-grid-cells`.

## 2. WeeklyRoster rewrite

- [x] 2.1 Rewrite `frontend/src/components/roster/weekly-roster.tsx` per design D-1, D-2, D-3, D-4. Replace the per-weekday-section accordion with a single CSS-grid container (`grid-template-columns: 110px repeat(7, minmax(140px, 1fr))`) wrapped in `overflow-x-auto`. Header row + body rows pivoted via the new `pivotRosterIntoGridCells` helper. Today's weekday computed once at the top via `Date#getDay()` with Sunday = 7. Verify by `pnpm tsc --noEmit`.
- [x] 2.2 Inside each scheduled cell, render the X/N badge with status color, then per-position seat group (position name once + filled chips + empty-seat placeholders). Self-chip variant uses `border-primary/40 bg-primary/10 text-primary`. PUBLISHED-state self-chip retains the `MoreHorizontal` dropdown with swap / give-direct / give-pool items (copy the existing menu near-verbatim from the old `weekly-roster.tsx:142-222`). Verify by component test in 4.2.
- [x] 2.3 Off-schedule cells render `<div className="border-dashed bg-muted/40">—</div>` with `aria-label={t("roster.offSchedule")}`. Verify by component test in 4.2.
- [x] 2.4 Today column header gets `bg-primary/10 text-primary` + the existing "今天" badge; each scheduled cell in today's column gets `ring-1 ring-primary/30`. Verify by component test.

## 3. i18n strings

- [x] 3.1 Add to `frontend/src/i18n/locales/{en,zh}.json`:
  - `roster.cell.summary` ("已分配 {{assigned}} / 需求 {{required}}" / "Assigned {{assigned}} of {{required}}")
  - `roster.cell.empty` ("空缺" / "Empty")
  - `roster.offSchedule` ("排班外" / "Off-schedule")
  Drop unused legacy strings (`roster.shiftSummary` if no longer referenced — verify via grep before deletion). Verify by `pnpm lint && pnpm tsc --noEmit`.

## 4. Frontend tests

- [x] 4.1 Update `frontend/src/components/roster/weekly-roster.test.tsx` (rewrite for the new topology):
  - Renders the 2D grid header (corner + 7 weekday cells).
  - Today's weekday header is highlighted (mock `Date` to a known weekday).
  - Self-chip variant rendered for `currentUserID` matches; default variant otherwise.
  - Off-schedule cells render `—` with the off-schedule aria-label and no chips.
  - Status colors applied: `full` for X==N, `partial` for 0<X<N, `empty` for X==0.
  - Self-chip dropdown is visible iff `publication.state === "PUBLISHED"` AND assignment is self; absent for `ACTIVE` or non-self.
  - Swap / give-direct / give-pool callbacks invoked with correct `WeeklyRosterOwnShift` payload.
  - Empty-seat placeholder chips appear when `assignments.length < required_headcount`.
- [x] 4.2 Verify by `pnpm test weekly-roster`.

## 5. Final gates

- [x] 5.1 `cd frontend && pnpm lint && pnpm test && pnpm build`. All clean.
- [x] 5.2 Manual smoke via Playwright on `localhost:5173/roster` (with Codex's stress seed currently loaded):
  - 2D grid layout: time labels in left column, weekday names in top row, no time/headcount duplication inside cells.
  - Status colors visible — full cells green, partial amber, empty red.
  - Today's column header highlighted; cells in today's column ringed primary.
  - Self chips highlighted (log in as a user who has assignments).
  - Off-schedule cells render `—` for time blocks not running on weekend.
  - In `PUBLISHED` state: self-chip dropdown opens with the three shift-change items and clicking each one fires the existing flow.
  - Take a "after" screenshot to compare against `.playwright-mcp/roster-baseline.png`.
- [x] 5.3 `openspec validate roster-page-redesign --strict`. Clean.
