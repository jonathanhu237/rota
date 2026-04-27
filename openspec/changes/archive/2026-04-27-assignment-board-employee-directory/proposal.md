## Why

The grid + side-panel layout we just shipped solves the "where are the gaps" question (color-coded `X / N` cells) but loses the "who is in this cell" information until you click into the side panel. Admins consistently want both at once: scan the week and see *which specific people* are filling each shift, not just whether the cell is full.

The recently-shipped right-panel editor is also asymmetric: it shows candidates *for the currently-selected cell only*, and the action loop is "click cell → click person → click cell → click person." That's the same modal-edit pattern this whole redesign was supposed to escape from. The right answer is to make the right panel a **stable global employee directory** and let admins drag people directly onto specific seats in the grid, with on-drag color feedback that says "this person fits here / doesn't fit here."

This change reshapes the page accordingly **and trims the assignment-board API to match the new UI's data needs**: the per-cell `candidates` and `non_candidate_qualified` arrays — useful when the right panel showed cell-scoped candidate lists, dead surface now — are dropped from `GET /publications/{id}/assignment-board`. A single top-level `employees` array replaces them, listing each active qualified employee once with their `user_positions`.

## What Changes

### Grid cell — explicit seats, not a single badge

- Each on-schedule cell renders one **seat block per `(position, headcount-index)`** unit. A daytime cell with composition `{前台负责人 × 1, 前台助理 × 2}` renders 3 seats stacked vertically; an evening cell with `{外勤负责人 × 1, 外勤助理 × 1}` renders 2.
- Each seat is one of two states:
  - **Filled** — shows the assigned employee's chip (`员工 38 (4.7h)`) with an `×` affordance for direct unassign.
  - **Empty** — shows a placeholder slot for that position (e.g., `空缺 · 前台助理`), visually distinct from filled.
- Each seat is its own drop target. Position is implied by the seat — admins don't pick a position separately.
- Cell-level summary (color status badge `已分配 X / 需求 N`) remains as a small header above the seat stack, but the seats themselves are the primary visual.
- Off-schedule cells render shaded `—` exactly as they do today.
- **Column headers** (`时间 / 星期一 / 星期二 / …`) are centered (cosmetic fix).

### Right panel — global employee directory

- Right panel becomes a **flat, always-visible directory of all active employees** for this publication. Independent of any cell selection (the selection concept is removed).
- Each row shows: employee name, current running total hours (sum across applied + draft assignments), and the employee's `user_positions` as small position-name chips.
- A search box at the top filters by name (substring match). A sort toggle switches between "by hours ascending" (default — surfaces low-load employees first for load-balance) and "by name."
- Above the directory, a small **gap banner** lists how many cells are still under-staffed (e.g., `仍缺 3 个 cell`). The previous summary-view's click-to-jump list is replaced by relying on the visible grid colors — admins scan the grid, no jump needed.
- Availability submissions are NOT surfaced in this panel. The data still exists in the database and the auto-assigner still uses it; it's simply invisible to the admin's manual-edit flow.

### Drag interaction — color-coded validity

- Drag source is any chip: a filled-seat chip in the grid, OR an employee row in the right panel directory.
- During a drag, every seat across the grid is repainted:
  - **Green border** if the dragged employee is qualified for that seat's position (`seat.position_id ∈ user_positions(employee)`).
  - **Yellow border** if the dragged employee is NOT qualified for that seat's position. Drop is still permitted; the resulting draft entry carries `isUnqualified: true` and triggers the existing confirmation dialog on Submit.
  - Off-schedule cells render no seats and never highlight.
- Drop targets:
  - **Empty seat**: stage `assign` to that `(slot, weekday, position)`.
  - **Filled seat (different user)**: stage `unassign` of the existing user + `assign` of the dragged user. Same-cell replacement.
  - **Filled seat (same user)**: no-op.
  - **Cross-cell drag from a filled seat**: also stage `unassign` from the source seat.

### Click — direct unassign on chip

The seat chip's `×` button stages an `unassign` draft entry. This is the only click-based mutation; no more "click candidate → stage assign" because there's no per-cell candidate concept in the right panel.

### Backend — assignment-board response shape

- **BREAKING** — `GET /publications/{id}/assignment-board` per `(slot, position)` pair drops the `candidates` and `non_candidate_qualified` arrays. Per-pair the response keeps only `assignments` (the currently-assigned users for that pair) and the position composition fields.
- **BREAKING** — adds a top-level `employees` array. Each entry: `{ user_id, name, email, position_ids: number[] }`. `position_ids` is restricted to positions appearing in the publication's slots (qualifications outside the publication's universe are noise and excluded). The bootstrap admin and `status != 'active'` users SHALL be excluded.
- The internal candidate-pool query the auto-assigner uses is unaffected — it queries `availability_submissions` directly via the service layer, never via this HTTP endpoint.

The frontend `deriveEmployeeDirectory` helper collapses to "consume the API's `employees` array and turn it into a `Map`" — no more aggregation across per-cell arrays.

### Capabilities

#### New Capabilities

None.

#### Modified Capabilities

- `scheduling`: two requirements modified.
  - *Admin assignment board drag-drop and draft submission* (last touched by `assignment-board-grid-redesign`) is reworded to describe the seat-level rendering, the global directory right panel, and the drag-color-feedback model. The Submit / Discard / running-hours / failure-handling sub-clauses are preserved verbatim. The `select cell to open editor` and `click candidate to stage assign` scenarios are removed. The cross-cell-drag scenarios re-target seats instead of cells.
  - *Assignment board surfaces non-candidate qualified employees* (the requirement defining the API response shape) is rewritten: per-pair `candidates` and `non_candidate_qualified` arrays drop out, a top-level `employees` array enters. The requirement keeps its current title for spec-delta hygiene; the literal "non-candidate qualified" framing is no longer accurate but renaming requires a separate cleanup.

### Frontend

- `frontend/src/components/assignments/assignment-board-cell.tsx` — re-architected. Was a single badge; becomes a stack of seat blocks with per-seat drop targets.
- `frontend/src/components/assignments/assignment-board-cell-editor.tsx` — **deleted** (selection-driven editor goes away).
- `frontend/src/components/assignments/assignment-board-summary-view.tsx` — **deleted** (gap-banner moves to right panel header).
- `frontend/src/components/assignments/assignment-board-side-panel.tsx` — re-architected as the directory shell (was the router). Owns search/sort state.
- `frontend/src/components/assignments/assignment-board-employee-row.tsx` (new) — one row per employee, draggable.
- `frontend/src/components/assignments/assignment-board-seat.tsx` (new) — one seat: filled or empty, droppable, click-x for unassign.
- `frontend/src/components/assignments/assignment-board-directory.ts` (new) — derives the global employee directory from the assignment-board payload's top-level `employees[]`.
- `frontend/src/components/assignments/assignment-board-dnd.ts` — drop-target topology updates: targets are seats (carrying `position_id`), not cells.
- `frontend/src/components/assignments/assignment-board.tsx` — drops the `selection` state (no longer used). Owns the dragging-employee-id state instead, so cells can color seats during drag.
- `frontend/src/components/assignments/assignment-board-side-panel-utils.ts` — **deleted** (helpers moved inline or into `assignment-board-directory.ts`).
- `frontend/src/components/assignments/assignment-board-assigned-chip.tsx` — **deleted** (seat owns the filled-chip rendering).
- `frontend/src/components/assignments/assignment-board-candidate-chip.tsx` — **deleted** (no candidates concept in right panel).
- Tests: rewrite `assignment-board.test.tsx` for the new topology; add tests for employee-directory rendering, search, sort, drag color states.
- i18n strings updated.

## Non-goals

- **Removing availability submissions or changing auto-assigner behavior.** Submissions still exist, employees still tick them, the auto-assigner still uses them. Admins simply don't see submission status in the directory. The auto-assigner's internal candidate-pool query continues to work via direct DB access — it does not consume the assignment-board HTTP endpoint and is unaffected by the response trim.
- **Showing per-cell candidate lists anywhere.** Intentionally dropped — the directory is the unified view.
- **Multi-cell selection / batch operations.** Same as before.
- **Per-occurrence-date overrides.** Recurrence-pattern model only.
- **Resizable / mobile layouts.** Desktop-first; viewports < 1280px are out of scope (the new layout is wider than before).
- **Removing the `+ / ×` buttons or the cross-cell DnD shortcut.** Click-x stays for unassign; cross-cell DnD becomes seat-to-seat instead of cell-to-cell.

## Impact

- **Frontend code**: significant. ~10 component files touched (some new, some deleted, the rest re-shaped). The shipped grid + side-panel layout was only on `main` for hours; this rewrite supersedes it before the dust settled. `deriveEmployeeDirectory` simplifies once the API exposes `employees[]` directly.
- **Backend code**: assignment-board handler / service / response DTO updated to emit the new shape. The repository query may simplify (no more building per-cell candidate / non-candidate arrays). Tests updated to match.
- **Spec**: two requirements in `scheduling` modified — the assignment board UI requirement and the assignment-board response-shape requirement.
- **No new third-party dependencies.** `@dnd-kit` continues to serve drag.
- **No infra / config / migration changes.**

## Risks / safeguards

- **Risk:** the grid gets visually wider because each cell stacks 2-3 seats vertically. → Mitigation: each seat is one line tall and roughly fits the existing cell width; total grid height grows by ~2× but width is unchanged. 1280px+ viewports are still fine.
- **Risk:** the right panel directory grows long (~42-50 employees with the realistic seed). → Mitigation: search box + sort by hours; admins use search to find the person they want to drop.
- **Risk:** dragging from a filled seat onto a different filled seat ("replace") might feel surprising vs the old "swap" semantic. → Mitigation: replace is the simpler default; admins who actually want to swap do it as two passes. The deferred-submission draft means nothing is committed until Submit, so a wrong drag is easy to undo.
- **Risk:** the drag-highlight repaint touches every seat in the grid (up to ~80 seats with the realistic seed). → Mitigation: a single dragged-employee-id state at the parent + memoized per-seat boolean computation; not a perf concern at this scale.
- **Risk:** shipping a UX rewrite a day after the previous one churns the spec history. → Mitigation: the spec change is a focused reword of a single requirement; the change folder under `archive/` will retain both designs side-by-side for posterity.
