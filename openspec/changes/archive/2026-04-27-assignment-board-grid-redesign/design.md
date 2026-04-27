## Context

The current `/publications/{id}/assignments` page renders one inline block per `(slot, weekday, position)` cell stacked vertically — for the realistic seed, that's ~50 blocks below the publication header. The visual loop is "weekday → time block → position → candidates list / assigned list / understaffed warning"; each level adds vertical space and labels. An admin scanning for "what's missing" has to read every block.

Two structural facts about the data make a 2D table the natural fit:

1. **Cells don't overlap.** Within a weekday, the slot trigger forbids time-overlap. Across weekdays, slots are independent. Every assignment lives in exactly one `(time block × weekday)` square.
2. **Composition is per-slot, not per-cell.** A slot's positions are the same on every weekday in its set, so a cell's "what's needed here" question is fully answered by its slot's composition — no cell-specific data shape variation.

This redesign collapses the current vertical-stack layout into a 2D grid (left, primary) plus a side-panel editor (right) that opens when a cell is selected. The editor handles in-cell add/remove via clicks; cross-cell moves continue to use drag-and-drop, narrowed to a single source/target combination (side-panel chip → grid cell). The deferred-submission draft model carries over wholesale.

## Goals / Non-Goals

**Goals:**

- A 2D grid where rows are time blocks (sorted by `start_time`), columns are weekdays Mon-Sun, and each cell shows `已分配 X / 需求 N` plus a color state (`full`, `partial`, `empty`).
- Off-schedule cells (slot doesn't run on that weekday) render shaded `—` and are not selectable / not droppable.
- A side panel that opens for the selected cell: cell context header, per-position blocks, assigned chips with running-hours badges, candidate chips, the existing "show all qualified" toggle, all in one fixed-width region.
- Click in the side panel for in-cell stage; drag side-panel chip → grid cell for cross-cell stage. Same draft entries, same warning + confirmation rules, same Submit replay against unchanged backend endpoints.
- A summary view in the side panel when nothing is selected: total demand vs assigned, list of cells still missing coverage with click-to-jump.

**Non-Goals:**

- Backend / API changes. Every endpoint involved (`GET /assignment-board`, `POST /assignments`, `DELETE /assignments/{id}`) keeps its current request and response shape.
- Auto-assign behavior changes.
- Per-occurrence-date editing.
- Multi-cell selection; batch operations.
- Resizable side panel; collapsible side panel; mobile / narrow-viewport layout (< 1024px).
- Drag-and-drop within the side panel (clicks only inside the panel) or between grid cells (chips don't render in grid cells).
- Removing `@dnd-kit` — narrowed in scope, kept as a dependency.

## Decisions

### D-1. Two-pane layout

Single page, two flex children:

- **Left pane** (`flex-1`, ≈ 70%): the grid.
- **Right pane** (`w-96`, fixed): the side panel.

The split is fixed because resize / collapse adds state-machine surface area without buying scanning utility. Desktop-first.

**Rejected — single pane with selected-cell-as-modal:** modal hides the grid behind the dialog, so admins lose the "what else is empty" peripheral view they're trying to gain. Side panel is always-visible, modal isn't.

**Rejected — selected-cell expands inline (accordion-style row):** brings back the vertical-scroll problem we're trying to solve. The whole point is the right pane is a fixed-position editor.

### D-2. Grid cell shape

Each cell renders:

- `已分配 X / 需求 N` text (one line)
- A status pill in one of three colors: `bg-green-100` for full (X == N), `bg-amber-100` for partial (0 < X < N), `bg-red-100` for empty (X == 0)
- A subtle border highlight when selected (`ring-2 ring-primary`)
- For cells whose slot doesn't run on that weekday: `bg-muted/40` background, `—` glyph, `cursor-not-allowed`, no click handler, no droppable

The total demand `N` is summed across the cell's positions (e.g., a daytime cell with `{前台负责人 × 1, 前台助理 × 2}` has `N = 3`). The total assigned `X` is the count of `assignments` rows in the API response for that `(slot, weekday)` summed across all positions.

**Rejected — show per-position breakdown in the cell** (`前台负责人 1/1, 前台助理 2/2`): too dense for a scan view; per-position detail belongs in the side panel where there's room.

**Rejected — show emoji or icon instead of color**: colors carry status faster on a glance; icons require parsing.

### D-3. Side-panel layout

```
┌─────────────────────────────┐
│  周一 09:00-10:00       [×] │   ← header with close button
│  已分配 3 / 需求 3           │   ← cell-level totals
├─────────────────────────────┤
│  前台负责人  1 / 1           │   ← per-position block
│  ─ 已分配                    │
│    [员工 34 (4.7h)]          │   ← chip (click ×, or drag)
│  ─ 候选人                    │
│    (no extra candidates)     │   ← when assigned == required and pool empty
├─────────────────────────────┤
│  前台助理   2 / 2            │
│  ─ 已分配                    │
│    [员工 1 (2h)]  [员工 26]  │
│  ─ 候选人                    │
│    [员工 22 (5h)]  ...       │
│  ☐ 显示所有具备资格的员工    │   ← per-block toggle
└─────────────────────────────┘
```

Header is sticky inside the panel. Per-position blocks are scrollable as a single inner scroll region. The `显示所有具备资格的员工` toggle stays scoped per-block (matches today's behavior).

**Rejected — single toggle for the whole panel:** today the toggle is per-`(slot, position)`; carrying that scope forward keeps the API and the user model consistent.

### D-4. Selection state

Selection is page-level state: `{ slotID: number, weekday: number } | null`.

- Initial state: `null` → side panel shows the summary view (D-6).
- Click a cell → set selection → side panel re-renders for that cell.
- Click the same cell again → clears selection (toggle).
- Click the side-panel `[×]` close button → clears selection.
- Switching cells preserves draft state (the draft-state machine is already per-`(slot, weekday, position, user)` and decoupled from selection); a user staging changes in cell A then clicking cell B sees those staged changes still recorded against cell A.

**Rejected — keyboard-only selection (arrow keys to move):** nice but adds keyboard map state; out of scope. Tab + Enter is the accessibility floor and that's enough for now.

### D-5. Click-to-stage

Inside the side panel:

- **Candidate chip** click → `enqueueAdd(draftState, { user_id, slot_id, weekday, position_id })`. Chip visually relocates to the assigned column with a `+` hint badge.
- **Assigned chip** click (or its `×` button) → `enqueueRemove(draftState, { user_id, slot_id, weekday, position_id, assignment_id })`. Chip stays in the assigned column but with a strikethrough or `−` hint badge.
- **Click a chip already staged in the inverse direction** → remove the inverse draft entry (cancel the staging). E.g., click an assigned-and-staged-for-removal chip → drop the `unassign` draft entry, chip returns to plain assigned visual state.

The existing `draft-state.ts` already has `enqueueAdd` / `removeFirstDraftOp`; minor renaming may be needed to align with `enqueueRemove`. The bookkeeping logic itself doesn't change.

### D-6. Summary view (no cell selected)

When `selection === null` the side panel shows:

```
┌─────────────────────────────┐
│  Realistic Rota Week 总览    │
│  已分配 N / 需求 M  [%]      │
├─────────────────────────────┤
│  仍缺人手 (k 个 cell)        │
│  ─ 周一 09:00-10:00 (1/3)    │
│  ─ 周三 19:00-21:00 (0/2)    │
│  ─ 周日 19:00-21:00 (0/2)    │
│  ...                         │
└─────────────────────────────┘
```

Each gap entry is clickable; click → set selection to that cell, panel switches to the editor view.

The gap list is sorted ascending by `(weekday, start_time)`, includes only cells where `X < N`, and excludes off-schedule cells.

**Rejected — show "fully covered" cells in the summary too:** redundant with the green grid; the summary's job is to surface what's left to do.

### D-7. Drag-and-drop topology

Drag source: any side-panel chip (assigned or candidate).
Drop target: any grid cell that is on-schedule (off-schedule cells reject drops).

Drag-end handler logic:

```
on dragEnd(source, target):
    if target.cellKey == currentSelection:
        # Drop into the currently-selected cell — same as a click on the chip
        enqueueAdd_or_Remove_by_kind(source)
    else:
        # Cross-cell move
        if source.kind == "assigned":
            enqueueRemove(source.from_cell, source.user)
            enqueueAdd(target.cell, source.user, source.position_id)
            # Note: the assign keeps the same position_id; if the target cell
            # doesn't have that position, the draft entry is marked
            # `isUnqualified: true` per the existing warning rules.
        elif source.kind == "candidate":
            enqueueAdd(target.cell, source.user, source.position_id)
```

The cross-cell `enqueueAdd` to a target cell whose composition doesn't include `source.position_id` triggers the existing `isUnqualified` warning, which the existing confirm dialog already handles. The user-facing message updates to mention "different cell" instead of "different position" when the source's position exists in the source cell but not the target — covered in i18n updates.

Drop targets visually highlight on `over`. Off-schedule cells render with `aria-disabled` and reject `over` events at the `useDroppable` config level.

**Rejected — "swap" semantic when dropping on a cell that already has the user assigned to a different position:** today's behavior is to emit four entries (unassign A from old, unassign B from old, assign A to new, assign B to new). With the new layout, drag is always one-directional (side-panel → grid), so true atomic swap is no longer expressible in a single gesture. Drop the swap shortcut; admins who want a swap make two passes.

### D-8. Component shape

```
assignment-board.tsx (page, ~150 lines)
├── grid section (left)
│   └── assignment-board-grid.tsx (~200 lines)
│       └── assignment-board-cell.tsx (~80 lines)
│           ├── click handler → page-level setSelection
│           └── useDroppable from @dnd-kit
└── side-panel section (right)
    └── assignment-board-side-panel.tsx (~250 lines)
        ├── when selected: cell editor
        │   └── per-position block
        │       ├── assigned chips (click to stage remove, draggable)
        │       └── candidate chips (click to stage assign, draggable)
        └── when not selected: summary view
```

The page-level `assignment-board.tsx` owns:
- `selection` state
- `draftState` state (existing)
- `DndContext` provider
- the API mutation handlers wired to `applyDraftToBoard` (existing)

The grid and side panel consume `selection`, `draftState`, and the API data; they emit selection changes and draft mutations.

`assignment-board-grid-cells.ts` (new) provides the helper:

```ts
export function pivotIntoGridCells(
  slots: AssignmentBoardSlot[],
): { timeBlocks: TimeBlock[]; weekdays: number[]; cells: GridCell[][] } {
  // 1. Collect distinct (start_time, end_time) pairs into time blocks, sorted.
  // 2. Build a 7-column lookup keyed on weekday.
  // 3. For each (timeBlock, weekday) compute the cell:
  //    - If a slot exists for that timeBlock and the slot's weekday set
  //      includes weekday → cell carries slot_id, totals, positions[].
  //    - Otherwise → cell is `{ kind: "off-schedule" }`.
  // 4. Return the matrix.
}
```

This replaces `group-assignment-board-shifts.ts`'s `groupAssignmentBoardSlotsByWeekday` (deleted).

### D-9. Draft state continuity

`draft-state.ts` keys entries on `(slot_id, weekday, position_id, user_id)` — orthogonal to UI selection. No change needed in that module beyond minor function-rename hygiene if `enqueueRemove` doesn't already exist. The existing tests stay; new integration-level tests cover "stage in cell A, click cell B, return to A, drafts intact."

### D-10. Tests

Aim for these unit tests at the component layer:

- Grid renders correct color state per cell (full/partial/empty/off-schedule).
- Off-schedule cell is not clickable and not droppable.
- Selecting a cell switches the side panel from summary to editor.
- Click-staging:
  - Click a candidate → assigned chip with `+` hint, draft entry exists.
  - Click an assigned chip → strikethrough, draft entry exists.
  - Click an `+`-staged chip → entry removed, chip returns to candidate list.
- Drag-staging:
  - Drag an assigned chip onto another cell → two draft entries (remove + add) emitted.
  - Drag a candidate chip onto another cell → one draft entry (add) emitted.
  - Drag onto an off-schedule cell → no entries emitted, drop rejected.
- Summary view:
  - Renders only when `selection === null`.
  - Lists every `X < N` cell, sorted by `(weekday, start_time)`.
  - Click on a gap entry sets selection.
- Submit replay flushes the draft via the existing handlers — already covered by the existing `draft-state.test.ts` and `assignment-board.test.tsx`; carry tests forward, update DOM queries.

## Risks / Trade-offs

- **Risk:** drag distance from side-panel chip to a grid cell on the far edge of a wide grid is long. → Mitigation: `@dnd-kit`'s `auto-scroll` activates near viewport edges; click-fallback always works.
- **Risk:** small grid cells make drop targets imprecise. → Mitigation: cell drop padding is generous (each cell's full bounding box is the droppable area, not just the badge); on-hover highlight makes the active target unambiguous.
- **Risk:** the `pivotIntoGridCells` helper duplicates logic the API already does (the API returns slots with weekdays). → Trade-off accepted: the API returns slot-keyed data, the grid wants cell-keyed data; a frontend pivot is the cheap translation.
- **Trade-off:** removing the cell-to-cell swap shortcut means an admin who wants to swap two assigned users does it as two passes (cell A: stage remove A's user; cell B: stage remove B's user; cell A: stage add B's user; cell B: stage add A's user). 4 actions vs the old 1 drag. → Acceptable: swap is rare in real workflows compared to "fix one specific cell," and the redesign optimises for the common case.
- **Risk:** a future "per-occurrence override" feature might want a 3D grid (time block × weekday × week). → Out of scope for this change; the 2D pivot helper is straightforward to extend.

## Migration Plan

Single shipping unit. Frontend-only:

1. Apply: refactor + new components + tests.
2. CI runs `pnpm lint && pnpm test && pnpm build` (per Done definition).
3. Manual smoke: with realistic seed loaded, walk a publication's assignment page — confirm grid renders, off-schedule cells render `—`, clicking a cell opens the editor, click-stage works, drag-stage works.
4. Rollback = revert the change; the backend stays untouched.

## Open Questions

None.
