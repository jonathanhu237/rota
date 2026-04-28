## 1. Pivot helper

- [x] 1.1 Add `frontend/src/components/availability/availability-grid-cells.ts` exporting `pivotAvailabilityIntoGridCells(shifts: QualifiedShift[]): { timeBlocks, weekdays, cells }` per design D-1. Each cell is `QualifiedAvailabilityCell` (with `kind: "qualified"`, `weekday`, `timeBlockIndex`, `slot_id`, `composition`) or `OffScheduleAvailabilityCell` (with `kind: "off-schedule"`, `weekday`, `timeBlockIndex`). Sort time blocks ascending by `(start_time, end_time)`. Verify by `pnpm tsc --noEmit`.
- [x] 1.2 Add `frontend/src/components/availability/availability-grid-cells.test.ts`: distinct `(start_time, end_time)` tuples produce one row each; weekdays without a matching qualified shift produce off-schedule cells; empty input produces an empty result. Verify by `pnpm test availability-grid-cells`.

## 2. AvailabilityGrid rewrite

- [x] 2.1 Rewrite `frontend/src/components/availability/availability-grid.tsx` per design D-6. Replace the per-weekday section list with a single CSS-grid container (`grid-template-columns: 110px repeat(7, minmax(120px, 1fr))`) wrapped in `overflow-x-auto`. Header row: corner cell + 7 weekday headers (today highlighted with `bg-primary/10 text-primary` and the "今天" / "Today" badge — D-4). Body rows: one per time block from the new pivot helper. Keep the existing component props (`shifts`, `selectedSlots`, `isPending`, `onToggle`) so the route file does not change. Verify by `pnpm tsc --noEmit`.
- [x] 2.2 Inside each qualified cell, render only a `<Checkbox>` (centered, `disabled={isPending}`, controlled by membership in `selectedSlots`) wrapped in a `<Tooltip>` whose content is the `availability.shift.composition` summary built from the cell's `composition` array. The cell's accessible label SHALL include the weekday label, time-block label, and composition summary so screen readers receive full context regardless of tooltip visibility (D-2). Verify by component test in 4.1.
- [x] 2.3 Off-schedule cells render `<div className="border-dashed bg-muted/40">—</div>` with `aria-label={t("availability.offSchedule")}` (D-3). Verify by component test in 4.1.

## 3. i18n strings

- [x] 3.1 Update `frontend/src/i18n/locales/zh.json`:
  - `sidebar.availability`: "可用性" → "提交空闲时间"
  - `availability.title`: "可用性" → "提交空闲时间"
  - `availability.description`: rephrase to action framing (e.g., "选择您能值班的时段。")
  - Add new key `availability.offSchedule`: "排班外"
  Do NOT modify the same keys in `frontend/src/i18n/locales/en.json` except to add `availability.offSchedule`: "Off-schedule" (the only addition; existing English labels stay unchanged per D-5). Verify by `pnpm lint && pnpm tsc --noEmit`.

## 4. Frontend tests

- [x] 4.1 Rewrite `frontend/src/components/availability/availability-grid.test.tsx` for the new grid topology:
  - Renders the 2D grid header (corner + 7 weekday cells), with one row per distinct time block.
  - Today's weekday header is highlighted (mock `Date` to a known weekday).
  - Qualified cell renders a checkbox whose `checked` reflects `selectedSlots`.
  - Toggling the checkbox invokes `onToggle(slot_id, weekday, checked)` with the correct payload.
  - Off-schedule cells render the `—` glyph with the off-schedule aria-label and no checkbox.
  - Tooltip content (or the cell's accessible name) includes the composition summary built from the position list.
  - Empty input still renders the grid header but no body rows (the route-level fallback handles the empty-shifts-empty-message; the grid only renders when `shifts.length > 0`).
- [x] 4.2 Verify by `pnpm test availability-grid`.

## 5. Sidebar test

- [x] 5.1 If `frontend/src/components/app-sidebar.test.tsx` asserts on the literal label of the Availability entry (it currently asserts on the i18n key `sidebar.availability`, not the resolved string, so likely no change is needed): re-verify it still passes after the i18n change. Verify by `pnpm test app-sidebar`.

## 6. Final gates

- [x] 6.1 `cd frontend && pnpm lint && pnpm test && pnpm build`. All clean.
- [x] 6.2 Manual smoke via Playwright on `localhost:5173/availability` (with a `COLLECTING` publication seeded — the Codex stress seed has one):
  - 2D grid layout: time labels in left column, weekday names in top row, no time/headcount duplication inside cells.
  - Cells with qualified shifts show a centered checkbox; toggling the checkbox calls the existing mutation (verify by checking the cell stays in its toggled state and the overall layout doesn't shift).
  - Hovering a qualified cell surfaces a tooltip with the position composition.
  - Today's weekday header is highlighted with the primary-tinted background and the "今天" badge.
  - Off-schedule cells render `—`.
  - Sidebar nav entry now reads "提交空闲时间" in Chinese; page title and description likewise. English stays "Availability".
  - Take an "after" screenshot for the archive.
- [x] 6.3 `openspec validate availability-page-redesign --strict`. Clean.
