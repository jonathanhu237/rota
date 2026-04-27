## Context

`/roster` was the last frontend page still rendering with the old "weekday accordion of position cards each duplicating time/headcount" pattern. After Playwright-confirmed inspection (`.playwright-mcp/roster-baseline.png`) the failure modes are identical to the pre-redesign assignment board: time labels duplicated 2-3 times per cell, no per-cell status color, position dashed-cards crammed 2-up inside narrow weekday columns. The fix is the same fix we already shipped on the assignment board — pivot the per-weekday payload into a 2D `(time × weekday)` grid, label time once on the left and weekday once on the top, render seats inside cells without repeating their context.

This change is frontend-only and preserves the only interactive behavior on the roster — the self-chip swap / give-direct / give-pool action menu when the publication is `PUBLISHED`.

## Goals / Non-Goals

**Goals:**

- 2D grid: rows = distinct `(start_time, end_time)` time blocks (sorted ascending), columns = weekdays Mon-Sun.
- Cell renders a single `已分配 X / 需求 N` badge with `full` / `partial` / `empty` status color and a seat stack grouped by position, with no per-position time or headcount duplication.
- Off-schedule cells (no slot at this time runs on this weekday) render `—` with muted background.
- Today's column header highlighted; self-chip highlighted; preserved swap/give actions on self chips when state is `PUBLISHED`.
- Pure frontend change; backend / API / spec-of-other-capabilities untouched.

**Non-Goals:**

- Editable cells. The roster stays read-only modulo the existing PUBLISHED self-chip menu.
- Backend / API shape changes.
- Per-occurrence detail panels.
- Filters, search, printable view, mobile-specific layout.
- Cross-week comparisons.

## Decisions

### D-1. Grid layout

CSS Grid template:

```
grid-template-columns: 110px repeat(7, minmax(140px, 1fr))
```

Rows:
- Row 0: header — empty corner cell + 7 weekday-name cells.
- Row N (N ≥ 1): time-label cell (left) + 7 cells (one per weekday, scheduled or off-schedule).

The whole grid is wrapped in `overflow-x-auto` so narrow viewports scroll. Total minimum width ≈ 110 + 7×140 = 1090px.

```
┌──────────┬─────────┬─────────┬─────────┬─────────┬─────────┬─────────┬─────────┐
│          │  星期一  │  星期二  │  星期三  │  星期四  │  星期五  │  星期六  │  星期日  │
│          │         │  今天   │         │         │         │         │         │
├──────────┼─────────┼─────────┼─────────┼─────────┼─────────┼─────────┼─────────┤
│ 09:00    │ 已3/3   │ 已3/3   │ 已3/3   │ 已3/3   │ 已3/3   │   —     │   —     │
│ 10:00    │ 前台    │ 前台    │ ...     │         │         │         │         │
│          │ 员工 5  │ 员工 1  │         │         │         │         │         │
│          │ ...     │ ...     │         │         │         │         │         │
├──────────┼─────────┼─────────┼─────────┼─────────┼─────────┼─────────┼─────────┤
│ 10:30    │ ...     │ ...     │ ...     │ ...     │ ...     │   —     │   —     │
│ 12:30    │         │         │         │         │         │         │         │
└──────────┴─────────┴─────────┴─────────┴─────────┴─────────┴─────────┴─────────┘
```

**Rejected — table element instead of CSS Grid.** Tables would handle row/column semantics natively but make per-cell click handlers + scroll-pinning headers awkward. CSS Grid + ARIA roles is enough.

**Rejected — column widths in `fr` only.** Fixed minimum keeps cells from collapsing on narrow weekday-empty columns.

### D-2. Cell rendering

**Scheduled cell:**

```tsx
<div className={cn(
  "flex flex-col gap-2 rounded-md border p-2 text-xs",
  statusColorClass(cell.totals),  // bg-emerald / bg-amber / bg-red variants
  isToday && "ring-1 ring-primary/30",
)}>
  <div className="flex items-center justify-between gap-2">
    <span className="font-medium">{t("roster.cell.summary", { assigned, required })}</span>
    <StatusDot status={cell.totals.status} />  {/* small colored dot */}
  </div>
  {cell.positions.map(positionEntry => (
    <div key={positionEntry.position.id} className="grid gap-1">
      <span className="text-muted-foreground">{positionEntry.position.name}</span>
      {/* seat list per D-3 */}
    </div>
  ))}
</div>
```

**Status colors** (matching assignment-board):
- `full` (X = N): `border-emerald-200 bg-emerald-50 dark:border-emerald-900/40 dark:bg-emerald-950/30`
- `partial` (0 < X < N): `border-amber-200 bg-amber-50 dark:border-amber-900/40 dark:bg-amber-950/30`
- `empty` (X = 0): `border-red-200 bg-red-50 dark:border-red-900/40 dark:bg-red-950/30`

**Off-schedule cell:**

```tsx
<div
  className="flex items-center justify-center rounded-md border border-dashed bg-muted/40 p-2 text-muted-foreground"
  aria-label={t("roster.offSchedule")}
>
  —
</div>
```

**Rejected — render off-schedule cells as fully empty (no border).** Keeping a faint dashed cell preserves grid alignment and makes the "this slot doesn't run today" reading explicit, mirroring the assignment-board.

### D-3. Seat rendering inside a cell

For each position group:

- The position name is shown once at the top of the group (small, `text-muted-foreground`).
- Each filled seat is a chip with the assignee's name. Self-chip variant: `border-primary/40 bg-primary/10 text-primary`. Self-chip when state is `PUBLISHED` includes the `MoreHorizontal` dropdown trigger for swap/give actions (existing behavior preserved verbatim).
- Each unfilled seat (when `assignments.length < required_headcount`) is a placeholder chip rendered as `border-dashed text-muted-foreground` with the localized text "空缺" / "Empty".

Seats stack vertically (no horizontal layout). At narrow column widths chips wrap their text on a second line if needed.

**Rejected — render only filled seats, omit empty-seat placeholders.** Hides the gap. The whole point of the redesign is showing coverage state per cell.

**Rejected — render assignees as compact `+1` / `+2` overflow chips when many.** Roster slots have at most 2-3 assignees per position in this app's data; not needed.

### D-4. Today column highlight

Today's weekday is computed once at the top of the component:

```ts
const today = new Date().getDay()
const todayWeekday = today === 0 ? 7 : today  // Sunday → 7
```

The header cell for today gets a primary-tinted background (`bg-primary/10 text-primary`) and a `今天 / Today` chip. Each scheduled cell in today's column gets a `ring-1 ring-primary/30` so the highlight extends visually down the column.

**Rejected — full-column background tint.** Too loud against the status-color cells.

### D-5. Pivot helper

`components/roster/roster-grid-cells.ts` exports:

```ts
export type RosterCellTotals = {
  assigned: number
  required: number
  status: "full" | "partial" | "empty"
}

export type ScheduledRosterCell = {
  kind: "scheduled"
  weekday: number
  slot: PublicationSlot
  occurrence_date: string
  positions: RosterPositionEntry[]
  totals: RosterCellTotals
}

export type OffScheduleRosterCell = {
  kind: "off-schedule"
  weekday: number
  timeBlockIndex: number
}

export type RosterCell = ScheduledRosterCell | OffScheduleRosterCell

export function pivotRosterIntoGridCells(
  weekdays: RosterWeekday[],
): {
  timeBlocks: { startTime: string; endTime: string }[]
  weekdays: number[]              // always [1..7]
  cells: RosterCell[][]           // [timeBlockIndex][weekdayIndex 0..6]
}
```

Implementation sketch:

```ts
// 1. Collect distinct (start_time, end_time) tuples across all weekdays' slots.
//    Sort ascending by start_time then end_time.
// 2. For each weekday in [1..7]:
//      For each timeBlock:
//        Find the slot in this weekday's slots whose (start_time, end_time)
//        matches.
//        - Match found → emit ScheduledRosterCell with totals computed from
//          sum(required_headcount) vs sum(assignments.length) across all
//          positions, and status derived as full/partial/empty.
//        - No match → emit OffScheduleRosterCell.
// 3. Return matrix.
```

This is structurally analogous to `pivotIntoGridCells` in `components/assignments/assignment-board-grid-cells.ts` — but the input shape is `RosterWeekday[]`, not `AssignmentBoardSlot[]`, so a separate helper is cleaner than over-genericizing the existing one.

**Rejected — generic helper consuming both shapes.** The two shapes differ in non-trivial ways (roster carries `occurrence_date`, assignment-board doesn't; assignment-board has `weekdays[]` per slot, roster slots are weekday-scoped). Genericizing would make the helper a type-juggling bag.

### D-6. Component shape

```
weekly-roster.tsx (new structure, ~250 lines)
├── computes today's weekday once
├── calls pivotRosterIntoGridCells(weekdays)
├── renders <div className="overflow-x-auto"><div className="grid grid-cols-[110px_repeat(7,minmax(140px,1fr))]">...</div></div>
├── header row: corner + 7 weekday header cells (extracted to <WeekdayHeader>)
└── for each timeBlock: time-label cell (left) + 7 RosterCell renders (extracted to <RosterScheduledCell> / <RosterOffScheduleCell>)
```

Sub-components co-located in the same file for now (small total size); split later only if any one grows past ~80 lines.

### D-7. Tests

- `roster-grid-cells.test.ts`:
  - Pivot collects distinct time blocks across weekdays.
  - Pivot returns off-schedule for `(time, weekday)` pairs not in the input.
  - `totals.status` correctly classified as full / partial / empty for various assignment counts.
- `weekly-roster.test.tsx` (rewrite):
  - Renders the 2D grid: corner + 7 headers + N time rows × 7 cells.
  - Today's weekday header is highlighted (per a stub of `Date`).
  - Self-chip is highlighted; non-self chips are not.
  - Off-schedule cells render `—`.
  - Status colors applied correctly per cell totals.
  - Self-chip dropdown is visible when `publication.state === "PUBLISHED"` and `currentUserID` matches; absent for `ACTIVE` or non-self.
  - Swap / give-direct / give-pool callbacks invoked with correct `WeeklyRosterOwnShift` payload (existing test assertions kept).

### D-8. Color tokens

This change introduces `bg-emerald-* / bg-amber-* / bg-red-*` raw Tailwind classes for the cell status. That's intentional consistency with the assignment-board which already uses the same tokens. The broader "abolish raw Tailwind palette literals in favor of `--color-status-*` tokens" cleanup is queued as `color-token-cleanup`; doing it here would expand scope.

## Risks / Trade-offs

- **Risk:** rows with sparse positions (e.g., a single 1-headcount position on a small weekday) waste vertical space because all cells in that row size to the tallest. → Acceptable; the grid is taller but readable.
- **Risk:** when no publication is in PUBLISHED/ACTIVE, the page already shows the "no current roster" state at the route level — the grid never mounts. The grid component only sees populated data. **No mitigation needed**, just confirming.
- **Risk:** the `data-icon="inline-end"` and existing dropdown trigger patterns in the codebase tie us to specific class shapes. → Mitigation: copy the existing self-chip block from `weekly-roster.tsx:142-222` near-verbatim into the new seat renderer; only the surrounding container changes.
- **Trade-off:** introducing `pivotRosterIntoGridCells` duplicates ~30 lines of logic that's spiritually identical to `pivotIntoGridCells` for the assignment board. Live with the duplication for now; both are stable shapes that won't drift fast.

## Migration Plan

Single shipping unit. Frontend-only:

1. Apply per `tasks.md`.
2. Run `pnpm lint && pnpm test && pnpm build`.
3. Manual smoke via Playwright: navigate to `/roster`, observe 2D grid layout, today highlight, self-chip highlight, off-schedule cells, status colors. Open self-chip dropdown when in PUBLISHED state and confirm swap/give items.

Rollback = revert the change; backend untouched.

## Open Questions

None.
