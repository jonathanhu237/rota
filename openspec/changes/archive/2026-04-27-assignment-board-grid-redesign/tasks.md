## 1. Frontend pivot helper

- [x] 1.1 Add `frontend/src/components/assignments/assignment-board-grid-cells.ts` exporting `pivotIntoGridCells(slots: AssignmentBoardSlot[]): { timeBlocks: TimeBlock[]; weekdays: number[]; cells: GridCell[][] }` per design D-8. A `GridCell` is either `{ kind: "scheduled", slotID, weekday, totals: {assigned, required, status: "full"|"partial"|"empty"}, positions: [...] }` or `{ kind: "off-schedule", weekday, timeBlockIndex }`. Sort `timeBlocks` by `(start_time, end_time)`. Verify by `cd frontend && pnpm tsc --noEmit`.
- [x] 1.2 Unit-test `pivotIntoGridCells` in `assignment-board-grid-cells.test.ts`: distinct (start_time, end_time) groups produce one row each; weekdays outside a slot's set produce off-schedule cells; cells reflect correct totals from candidates/assignments. Verify by `pnpm test assignment-board-grid-cells`.
- [x] 1.3 Delete `frontend/src/components/assignments/group-assignment-board-shifts.ts` (replaced by 1.1). Verify by grep — no remaining imports.

## 2. Page-level re-architecture

- [x] 2.1 Refactor `frontend/src/components/assignments/assignment-board.tsx` (currently 1072 lines) into a thin parent (~150 lines) that owns `selection: { slotID, weekday } | null` state, the existing `draftState`, the existing API mutation handlers, and the `DndContext` provider. Render `<AssignmentBoardGrid>` left + `<AssignmentBoardSidePanel>` right per design D-1. Verify by `pnpm tsc --noEmit`.
- [x] 2.2 Add `frontend/src/components/assignments/assignment-board-grid.tsx` rendering the 2D table per design D-2: rows = time blocks, columns = weekdays Mon-Sun, each cell consuming `<AssignmentBoardCell>`. Off-schedule cells show `—` shaded. Verify by `pnpm tsc --noEmit`.
- [x] 2.3 Add `frontend/src/components/assignments/assignment-board-cell.tsx` rendering one grid cell: progress badge + status pill, click-to-select, droppable via `useDroppable` with `disabled` set when off-schedule. Verify by `pnpm tsc --noEmit`.

## 3. Side panel (editor view)

- [x] 3.1 Add `frontend/src/components/assignments/assignment-board-side-panel.tsx` that branches on `selection`:
  - When non-null: render the cell editor per design D-3 (sticky header with cell context + close button, scrollable per-position blocks each with assigned chips, candidate chips, "show all qualified" toggle).
  - When null: render the summary view per design D-6.
- [x] 3.2 Implement chip click behavior per design D-5 in the side panel:
  - Click candidate chip → `enqueueAdd(...)`; chip relocates to assigned column with `+` hint.
  - Click assigned chip's `×` → `enqueueRemove(...)` (rename `removeFirstDraftOp` if needed, see 5.1); chip stays with strikethrough.
  - Click chip with inverse staged entry → cancel that entry; chip returns to original column.
- [x] 3.3 Make side-panel chips draggable via `useDraggable` per design D-7. Drag source data carries `{ kind: "assigned" | "candidate", user_id, slot_id, weekday, position_id, assignment_id? }`.

## 4. Drag wiring

- [x] 4.1 Rewrite `frontend/src/components/assignments/assignment-board-dnd.ts` per design D-7. The `onDragEnd` handler:
  - If drop target is the currently-selected cell → behave as a click on the chip (single stage entry).
  - If drop target is a different on-schedule cell:
    - Source `kind: "assigned"` → emit `unassign` from source cell + `assign` to target cell (two entries).
    - Source `kind: "candidate"` → emit `assign` to target cell (one entry).
  - If drop target is off-schedule → reject (no entries).
- [x] 4.2 Cross-cell `assign` entries with `position_id` not in the target cell's composition mark `isUnqualified: true` so the existing confirm dialog handles them. Verify by adding a test in `draft-state.test.ts` covering "assign Alice (cashier-only) to a cook-only cell via cross-cell drag → entry has isUnqualified: true."

## 5. Draft state alignment

- [x] 5.1 Audit `frontend/src/components/assignments/draft-state.ts` for naming hygiene per design D-9. If `enqueueRemove` (paired with the existing `enqueueAdd`) doesn't already exist, add it (or rename `removeFirstDraftOp` if that's its current shape). The persistence semantics across cell-selection changes are already correct since entries are keyed on `(slot_id, weekday, position_id, user_id)`. Verify by `pnpm test draft-state`.
- [x] 5.2 Add a draft-state test for "selection-change does not lose drafts": stage an entry, simulate selection change, verify entry still in queue. Verify by `pnpm test`.

## 6. Summary view

- [x] 6.1 Implement summary content per design D-6 in `assignment-board-side-panel.tsx`: total demand vs total assigned across all on-schedule cells (counting drafts), gap list listing every on-schedule cell where `X < N` sorted by `(weekday, start_time)`, each entry click-to-jump (sets `selection`).
- [x] 6.2 Unit test the summary view: empty state when all cells are full; gap entries sorted correctly; click on a gap entry sets selection.

## 7. Tests + i18n

- [x] 7.1 Rewrite `frontend/src/components/assignments/assignment-board.test.tsx` for the new layout. Cover: initial render shows summary; selecting a cell switches to editor; click-stage from candidates; click-stage from assigned; cancel via inverse click; cross-cell drag from assigned → two entries; cross-cell drag from candidate → one entry; drop on off-schedule rejected. Verify by `pnpm test assignment-board`.
- [x] 7.2 Update `frontend/src/i18n/locales/{en,zh}.json`: add summary-view strings (`仍缺人手 / Coverage gaps`, etc.), update warning-dialog wording to handle "different cell" wording where relevant. Drop unused strings from the old layout. Verify by `pnpm lint && pnpm tsc --noEmit`.
- [x] 7.3 If `assignment-board-state.ts` still exports helpers used by the old layout (e.g., `getVisibleNonCandidateQualified`, `isAssignmentBoardPositionUnderstaffed`), keep / refactor only the ones the new components consume; delete unused exports. Verify by grep + `pnpm tsc --noEmit`.

## 8. Spec sync

- [x] 8.1 Confirm the change-folder spec delta at `openspec/changes/assignment-board-grid-redesign/specs/scheduling/spec.md` matches the implemented behavior (new `Admin assignment board drag-drop and draft submission` requirement text, all scenarios). Do not edit `openspec/specs/scheduling/spec.md` directly — `/opsx:archive` syncs it.

## 9. Final gates

- [x] 9.1 `cd frontend && pnpm lint && pnpm test && pnpm build` — all clean.
- [x] 9.2 Manual smoke with realistic seed: load `/publications/{id}/assignments`, scan grid colors for gap visibility, click a partial cell, click-stage a candidate, observe added hint, drag an assigned chip onto another cell, observe two staged entries, click Submit, observe confirm dialog (or no dialog) per warnings, confirm assignments persist after refresh.
- [x] 9.3 `openspec validate assignment-board-grid-redesign --strict` — clean.
