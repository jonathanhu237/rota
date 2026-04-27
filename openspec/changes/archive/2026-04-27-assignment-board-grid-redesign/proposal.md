## Why

Today's `/publications/{id}/assignments` page is a single vertical scroll: weekdays nested as headers, time blocks nested under those, and each `(slot, weekday, position)` cell rendered inline with `候选人 + 已分配 + 已分配进度` stacked. With the realistic seed (5 time blocks × Mon-Fri daytime + Mon-Sun evening = 27 cells × up to 2 positions each ≈ 50 inline blocks) the page is hundreds of lines of repeated structure. The admin cannot scan for "which slot is short" without reading every cell. The two visually-co-mingled lists per cell (candidates vs assigned) further confuse "what's the answer here" with "what could the answer be."

The data model already gives us a clean 2D plane: time block × weekday is a primary key under the new schema (every assignment is exactly one cell), slots within a weekday don't overlap (DB-enforced), and weekday membership is decided per-slot. The natural visualisation is a 2D grid; everything else is shoehorned. The redesign aligns the UI with that data shape and splits the page into two zones — grid on the left for *the answer*, side panel on the right for *the editor* — so scanning, focusing, and editing each get their own visual budget.

## What Changes

### Page layout

- **2-pane layout** at `/publications/{id}/assignments`. Left pane (≈ 70% width) is a 2D grid: rows are the publication's time blocks (sorted by `start_time`), columns are weekdays Mon-Sun. Right pane (≈ 30% width) is the selected-cell editor. Resizable not in scope.
- **Grid cells** carry: a single combined progress badge `已分配 X / 需求 N` plus a color state (`full = X==N`, `partial = 0<X<N`, `empty = X==0`). No per-position breakdown, no chips. Compact: each cell is one line tall.
- **Off-schedule cells** (a (time block, weekday) pair where the slot doesn't run, e.g., Saturday daytime) render shaded with a `—` and are not selectable.
- **Cell selection** is a single-click toggle. Selected cell highlights; the right pane reflects the selection.
- **Side panel** when a cell is selected shows: cell context header (`周一 09:00-10:00`, occurrence date if relevant), per-position blocks, each block listing assigned chips (with running-hours badge per user, kept from the existing requirement) and candidate chips, plus the existing `显示所有具备资格的员工` toggle scoped to the panel.
- **Side panel without selection** shows a publication summary: total demand, total assigned, list of cells still missing coverage as a clickable "go to cell" list.

### Editing model — clicks for in-cell, drag for cross-cell

Two input modalities, each owning the case it's good at:

- **Click in the side panel** for in-cell adjustments (the common case). Click a candidate chip to stage an `assign` draft entry; the chip moves visually into the assigned column with an "added" hint. Click an assigned chip's `×` (or the chip itself) to stage an `unassign` draft; the chip stays visible with a "to-remove" hint.
- **Drag side-panel chip → grid cell** for cross-cell adjustments. Drag an assigned-in-the-current-cell chip and drop it on another grid cell to stage `unassign-from-current` + `assign-to-target` in one gesture (the "swap into another cell" workflow). Drag a candidate chip onto a grid cell other than the current one to stage `assign-to-target` directly without re-selecting.
- **Drag-and-drop between grid cells** is NOT supported — grid cells render only the `X / N` badge with no per-user chips, so there's no source chip to drag from a non-selected cell. Cross-cell moves always start from the side panel.
- The page-level Submit button (top right, same position as today) flushes the entire draft via `POST /publications/{id}/assignments` and `DELETE /publications/{id}/assignments/{assignment_id}`. Same warning + confirmation rules as today (unqualified-position drafts trigger the confirm dialog). The deferred-submission via draft model is preserved end-to-end.
- The existing per-user `+ / ×` immediate-commit buttons collapse into the click-staging model: clicks always stage, never immediate-commit. Accessibility relies on keyboard nav + Enter (Tab to a chip, Enter to stage; Tab to a grid cell, Enter to select it) rather than special-case buttons.

### Capabilities

#### New Capabilities

None.

#### Modified Capabilities

- `scheduling`: the *Admin assignment board drag-drop and draft submission* requirement is reworded to reflect the new mixed click + side-panel-to-grid-cell drag interaction. The deferred-submission, per-cell warning, confirmation-dialog, and running-hours-badge clauses are preserved. The cell-to-cell drag scenario from the old requirement (drag a chip from grid cell A onto grid cell B) is removed — grid cells no longer render per-user chips, so they cannot be drag sources. The legacy `+ / ×` immediate-commit clause is removed (clicks now stage instead).

### Frontend

- `frontend/src/routes/_authenticated/publications/$publicationId/assignments.tsx` — re-architected around the grid + side panel.
- `frontend/src/components/assignments/assignment-board.tsx` (1072 lines today) — split into `assignment-board-grid.tsx` (the 2D grid), `assignment-board-side-panel.tsx` (the editor for a selected cell), and a thin parent `assignment-board.tsx` that wires the grid → side-panel selection state.
- `frontend/src/components/assignments/assignment-board-dnd.ts` — rewritten. The drag source set shrinks to "side-panel chip"; the drop target set shrinks to "grid cell." `@dnd-kit` stays.
- `frontend/src/components/assignments/draft-state.ts` (and its tests) — preserved largely as-is; the draft entries are unchanged in shape. Call sites that produce entries split into click handlers (in-cell stage) and drag handlers (cross-cell stage).
- `frontend/src/components/assignments/group-assignment-board-shifts.ts` — replaced by a new `assignment-board-grid-cells.ts` helper that pivots the API response into `cells[time_block_index][weekday]` form.
- Tests for the new components: cell selection, click-to-stage, click-to-unstage, off-schedule cell behavior, summary view when nothing is selected.
- i18n strings under `frontend/src/i18n/locales/{en,zh}.json` updated for new labels (`仍缺人手 / Coverage gaps`, etc.) and old DnD strings removed.

## Non-goals

- **Backend / API changes.** `GET /publications/{id}/assignment-board`, `POST /publications/{id}/assignments`, `DELETE /publications/{id}/assignments/{assignment_id}` — all unchanged in request and response shape.
- **Auto-assign algorithm or coverage objective changes.** Same MCMF, same cells, same fairness-out-of-scope stance.
- **Per-occurrence-date editing.** The board still operates on the recurrence-pattern model (`(publication, user, slot, weekday)`); per-week overrides remain shift-change territory.
- **Multi-cell selection or batch operations.** One cell selected at a time. No "select all empty Tuesday cells and assign Alice to each."
- **Resizable / collapsible side panel.** Fixed proportions — keep the design centered on a single primary screen size.
- **Drag-and-drop within the side panel.** Click only inside the side panel; drag is exclusively the side-panel-chip → grid-cell interaction.
- **Drag-and-drop between grid cells.** Grid cells render no chips; there's nothing to drag from one cell to another.
- **Mobile / narrow-viewport layout.** Desktop-first; behavior at viewports < 1024px is undefined for now.

## Impact

- **Frontend code**: significant.
  - One route file (`assignments.tsx`) and one component (`assignment-board.tsx`) re-architected.
  - One module (`assignment-board-dnd.ts`) deleted; one (`group-assignment-board-shifts.ts`) replaced.
  - Tests: existing `assignment-board.test.tsx` rewritten; new tests for grid + side panel.
  - One Zod schema unchanged (response shape preserved).
- **Spec**: one `scheduling` requirement renamed + reworded; one removed scenario (cell-to-cell swap).
- **Backend code**: no changes. No migrations. No new endpoints.
- **No new third-party dependencies.** `@dnd-kit` stays — it now serves a narrower, well-scoped interaction.
- **No infra / config changes.**

## Risks / safeguards

- **Risk:** drag from side-panel chip to grid cell is a longer drag distance than today's same-region drag, and the grid cell drop target is small. **Mitigation:** drop target highlights on hover; a generous drop padding around each cell; click-based fallback always works for users who find the drag awkward.
- **Risk:** the side panel's draft state needs to persist across cell-selection changes (admin stages changes in cell A, clicks cell B, must not lose the cell-A drafts). **Mitigation:** the existing draft-state machine already keys entries by `(slot_id, weekday, position_id, user_id)`; selection is a UI-only concept on top. Test coverage for "switch cells, return, drafts intact."
- **Risk:** the spec change to scheduling capability re-touches a recently-archived requirement (drag-drop was added by the drag-drop-assignment-board change). **Mitigation:** clearly mark the new requirement as superseding the drag-drop UX while keeping the deferred-submission semantics.
- **Risk:** the empty / non-selection state of the side panel might be visually awkward. **Mitigation:** that state shows the publication-level summary (gap list), which is itself useful — admins land on the page, see what's missing, click their way through. The state is meaningful, not "blank."
