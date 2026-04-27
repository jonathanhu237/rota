## Why

The `/roster` page renders the same kind of mess that `/assignments` had before its grid + seat-stack redesign:

- **Time labels duplicated 2-3 times per cell.** Each slot's `<header>` shows "08:00-10:00 · 需要 3 人", then a bare `slotEntry.positions.length` integer floats below the header on its own line, then *each* position card inside repeats "08:00-10:00 ·需要 1 人". With two positions side-by-side the headers visually merge into "08:00-10:0008:00-10:00 ·需要 1 人 ·需要 1 人" (browser-confirmed via Playwright on the running dev server).
- **No status color per cell.** Every cell looks identical regardless of full / partial / empty. An admin scanning the week for gaps has no visual cue.
- **Position cards 2-up inside a 140px-wide weekday column.** Position dashed-bordered boxes pile horizontally inside an already-narrow column and overflow into each other.
- **Today column is ringed but the header is the only visual anchor**; mid-page, a viewer looking at a Wednesday row has no signal which column is "today."
- **Self assignments are highlighted (good)**, but the highlight competes for attention with the dashed-border position cards, the bare position-count integer, and three layers of nested padding.

The fix is straightforward: apply the same pattern this project already shipped on `/assignments` (`assignment-board-grid-redesign` + `assignment-board-employee-directory`) — pivot the weekday-keyed payload into a 2D `(time × weekday)` grid, show time once on the left, weekday once on the top, render seats inside each cell with no per-position time/headcount duplication, and add status colors so coverage gaps pop.

The roster is read-only (no drag, no chip-click-to-edit). The only interactive affordance — the self-chip dropdown for swap / give-direct / give-pool when the publication is `PUBLISHED` — is preserved unchanged.

## What Changes

### Layout

- `WeeklyRoster` is rewritten around a 2D CSS grid: leftmost column is the time-block label, then 7 weekday columns (Mon-Sun). Top row is the weekday header. Body rows are one per distinct `(start_time, end_time)` pair.
- Each `(time, weekday)` cell renders one of:
  - **Scheduled** — a small `已分配 X / 需求 N` badge with one of three status colors (`full`, `partial`, `empty`), then a seat stack grouped by position. Each position group shows the position name once and lists its assigned chips (self-chips highlighted) followed by empty-seat placeholders for any unfilled headcount. **No per-position time or headcount duplication.**
  - **Off-schedule** — a muted cell with a `—` glyph (matches the assignment-board off-schedule styling).
- Column headers: weekday name + a "today" badge on today's column. Today's column header gets a `ring-1 ring-primary/30` so the visual anchor extends down the column visibly.
- Self assignment chip highlight stays (`border-primary/40 bg-primary/10 text-primary`).
- The shift-action dropdown (swap / give-direct / give-pool) remains on self chips when `publication.state === "PUBLISHED"`. Logic and i18n unchanged.

### Pivot helper

- New `pivotRosterIntoGridCells(weekdays: RosterWeekday[])` helper analogous to the existing `pivotIntoGridCells` in `components/assignments/assignment-board-grid-cells.ts`, but consuming the roster-shaped payload. Output: `{ timeBlocks, weekdays, cells }` with each cell tagged scheduled vs off-schedule and carrying the per-position assignments where scheduled.

### Empty / loading / single-source-empty cases

- Cell-level "no assignments" state (a previously-loaded slot with zero assignments) folds into the empty-color cell — no separate empty-text card per position.
- The page-level "no current publication" copy at the route level (`routes/_authenticated/roster.tsx`) is retained; only the inner grid changes.

### Capabilities

#### New Capabilities

None.

#### Modified Capabilities

- `frontend-shell`: adds one new requirement *Roster page layout* defining the 2D grid contract, status colors, off-schedule rendering, today column highlight, self-chip highlight, and preservation of the swap / give-direct / give-pool action menu on PUBLISHED self chips.

### Frontend

- `frontend/src/components/roster/weekly-roster.tsx` — full rewrite (~240 lines → ~250 lines, similar size, simpler structure).
- `frontend/src/components/roster/roster-grid-cells.ts` (new) — the pivot helper + types.
- `frontend/src/components/roster/roster-grid-cells.test.ts` (new) — pivot helper unit tests.
- `frontend/src/components/roster/weekly-roster.test.tsx` — rewrite test fixtures for the new grid topology; keep coverage of the today-column highlight, self-chip highlight, and shift-action menu visibility.
- i18n strings: add `roster.cell.summary` ("已分配 {{assigned}} / 需求 {{required}}" / "Assigned {{assigned}} of {{required}}"), `roster.cell.empty` ("空缺" / "Empty"), `roster.offSchedule` (the `—` aria label "Off-schedule"). Drop unused legacy strings (`roster.shiftSummary` reuse may shrink — verify before deletion).

## Non-goals

- **Editable roster.** The roster is read-only (modulo the existing PUBLISHED self-chip swap / give actions). Drag-drop, click-to-edit, etc. stay out of scope; that's the assignment-board's job.
- **Backend changes.** The `GET /publications/:id/roster?week=...` response shape is unchanged; the frontend only pivots it differently.
- **Per-occurrence detail pages from the roster cell.** Clicking a chip still opens the action menu on self, does nothing on others. No "click chip → see employee profile."
- **Roster filters / search / printable view.** These are valid UX requests but separate changes.
- **Mobile / narrow-viewport layout.** Desktop-first; the grid stays in `overflow-x-auto` and scrolls horizontally on narrow viewports.
- **Multi-week comparisons.** The roster shows one week at a time, with the existing prev/next navigation untouched.
- **Color / form / confirm-dialog cleanup queued for separate changes.** This change touches only roster-page colors that need the status-token treatment; broader audit-flagged color issues (`destructive` overuse on badges across the app) ride in `color-token-cleanup`.

## Impact

- **Frontend code:**
  - `WeeklyRoster` rewritten.
  - Pivot helper added.
  - Tests updated.
  - i18n strings adjusted.
- **Spec:** one new requirement added to `frontend-shell`.
- **No backend code, schema, migration, or API changes.**
- **No new third-party dependencies.**

## Risks / safeguards

- **Risk:** cells with many positions (e.g., evening slots staffing 4-5 different positions) get tall and uneven, breaking the visual rhythm of the row. **Mitigation:** seats stack vertically with consistent line-height; rows naturally sized to their tallest cell, just like assignment-board.
- **Risk:** off-schedule rendering might be confusing for users who don't know the slot doesn't run that day (e.g., evening 19:00-21:00 on weekend in the realistic seed *does* run, but daytime 09:00-10:00 doesn't run weekend). **Mitigation:** consistent `—` glyph + muted background mirror the assignment-board's already-shipped pattern; users seeing both pages get a consistent vocabulary.
- **Risk:** the grid overflows horizontally on screens narrower than ~1090px. **Mitigation:** preserved `overflow-x-auto` wrapper, same as today's implementation.
- **Risk:** the swap / give-direct / give-pool action menu loses discoverability if the chip becomes smaller. **Mitigation:** keep the self-chip's right-side `MoreHorizontal` icon-button trigger pattern; only the chip's container shrinks slightly.
