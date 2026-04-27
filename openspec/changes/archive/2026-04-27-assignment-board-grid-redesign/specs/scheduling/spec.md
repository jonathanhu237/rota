## MODIFIED Requirements

### Requirement: Admin assignment board drag-drop and draft submission

The admin assignment-board UI SHALL render assignments as a 2D grid: rows are the publication's distinct time blocks (sorted by `start_time`, then `end_time`), columns are weekdays Mon-Sun. Each grid cell representing an on-schedule `(slot, weekday)` pair SHALL show a single combined `已分配 X / 需求 N` progress badge plus a status color (`full` when `X = N`, `partial` when `0 < X < N`, `empty` when `X = 0`) and SHALL be selectable by click. Cells whose `(slot, weekday)` is not in the slot's weekday set SHALL render shaded with a `—` glyph and SHALL NOT be selectable or droppable.

The page SHALL be a two-pane layout: the grid on the left, a side-panel editor on the right. The side panel SHALL display the publication summary (total demand vs assigned, plus a click-to-jump list of cells where `X < N` sorted by `(weekday, start_time)`) when no cell is selected, and the cell editor (cell context header, per-position blocks with assigned chips and candidate chips, the existing per-position "show all qualified employees" toggle) when a cell is selected. Selection SHALL be single (one cell at a time); clicking the selected cell again or the side-panel close affordance SHALL clear the selection.

The admin SHALL stage assignment changes via two input modalities, both of which produce entries in a single deferred-submission draft:

- **Click in the side panel** — clicking a candidate chip stages an `assign` draft entry; clicking an assigned chip (or its `×` affordance) stages an `unassign` draft entry. Clicking a chip that already has an inverse staged entry cancels that entry.
- **Drag a side-panel chip onto a grid cell** — dropping an assigned chip on a different cell stages `unassign` from the source cell plus `assign` to the target cell; dropping a candidate chip on a different cell stages `assign` to the target cell. Dropping onto the currently-selected cell behaves as a click on the chip. Drops on off-schedule cells SHALL be rejected.

The draft SHALL persist across cell-selection changes (a user staging changes in cell A then selecting cell B SHALL find the cell-A drafts still recorded). Clicking Submit SHALL replay the draft as a sequence of `POST /publications/{id}/assignments` and `DELETE /publications/{id}/assignments/{assignment_id}` calls. Each draft entry that produces a request the admin is permitted but not "default" to make (specifically: assigning a user to a `(slot, position)` whose `position_id` is not in the user's `user_positions`) SHALL be marked with a warning indicator. Submit SHALL trigger a confirmation dialog if any pending entries carry warnings; if none do, Submit SHALL fire without confirmation.

The UI SHALL display each assigned user's running total hours (sum of `slot.end_time − slot.start_time` over the user's currently-applied assignments in this publication, including pending drafts) inside their chip.

This requirement does not change any backend API contract. The endpoints `POST /publications/{id}/assignments` and `DELETE /publications/{id}/assignments/{assignment_id}` are the only writes.

#### Scenario: Grid cell renders summary state

- **GIVEN** a `(slot, weekday)` cell with `required_headcount` totalling 3 and 2 assignments currently applied
- **WHEN** an admin loads the assignment board
- **THEN** the cell renders `已分配 2 / 需求 3` with the `partial` status color
- **AND** the cell is clickable

#### Scenario: Off-schedule cell is non-interactive

- **GIVEN** a daytime time block whose slot's weekday set is `{1, 2, 3, 4, 5}` and weekday `6` (Saturday)
- **WHEN** an admin loads the assignment board
- **THEN** the `(time block, Saturday)` cell renders shaded with `—` and `aria-disabled`
- **AND** clicking the cell does not change selection
- **AND** dragging a chip over the cell does not produce a drop highlight

#### Scenario: Selecting a cell opens the editor in the side panel

- **GIVEN** an admin viewing the assignment board with no cell selected
- **WHEN** the admin clicks a grid cell
- **THEN** the side panel transitions from the summary view to the cell editor view for that cell
- **AND** the cell renders with a selection highlight

#### Scenario: Click a candidate chip to stage an assign

- **GIVEN** an admin viewing the cell editor for cell `(slot S, weekday W)` with `员工 22` in the candidates list
- **WHEN** the admin clicks the `员工 22` chip
- **THEN** the chip moves visually into the assigned column with an "added" hint
- **AND** a draft entry of kind `assign` for `(S, W, position, 员工 22)` is queued
- **AND** no API call is made yet

#### Scenario: Click an assigned chip to stage an unassign

- **GIVEN** an admin viewing the cell editor for cell `(slot S, weekday W)` with `员工 34` in the assigned list
- **WHEN** the admin clicks the `员工 34` chip's `×` affordance
- **THEN** the chip stays in the assigned column with a strikethrough or "to-remove" hint
- **AND** a draft entry of kind `unassign` for `(S, W, position, 员工 34)` is queued
- **AND** no API call is made yet

#### Scenario: Click a chip that already has an inverse staged entry cancels it

- **GIVEN** an admin who has just clicked a candidate chip, staging an `assign` entry
- **WHEN** the admin clicks the same chip again (now showing the "added" hint in the assigned column)
- **THEN** the staged `assign` entry is removed from the draft
- **AND** the chip returns to the candidate column

#### Scenario: Drag an assigned chip onto a different on-schedule cell

- **GIVEN** an admin viewing the cell editor for cell A with `员工 34` assigned
- **WHEN** the admin drags `员工 34` from the side-panel assigned chip onto cell B (a different on-schedule cell)
- **THEN** two draft entries are queued: `unassign` from cell A's `(slot, weekday, position, 员工 34)`, and `assign` to cell B's `(slot, weekday, position, 员工 34)`
- **AND** no API call is made yet

#### Scenario: Drag a candidate chip onto a different on-schedule cell

- **GIVEN** an admin viewing the cell editor for cell A with `员工 22` in the candidates list
- **WHEN** the admin drags `员工 22` from the side-panel candidate chip onto cell B (a different on-schedule cell)
- **THEN** one draft entry of kind `assign` for cell B's `(slot, weekday, position, 员工 22)` is queued
- **AND** no API call is made yet

#### Scenario: Drop on a cell whose composition does not include the position

- **GIVEN** an admin dragging an assigned chip whose `position_id` is `Cashier`
- **WHEN** the admin drops the chip on a cell whose composition is `{Cook}` only
- **THEN** the drop is accepted
- **AND** the draft entry is marked with a warning (`isUnqualified: true`)
- **AND** the chip renders with a warning indicator in the side panel after the user re-selects the target cell

#### Scenario: Drop on an off-schedule cell is rejected

- **GIVEN** an admin dragging any side-panel chip
- **WHEN** the admin drops the chip on an off-schedule cell
- **THEN** the drop is rejected
- **AND** no draft entry is queued

#### Scenario: Drafts persist across cell-selection changes

- **GIVEN** an admin who has staged an `assign` entry in cell A's editor
- **WHEN** the admin clicks cell B (selection changes)
- **AND** the admin then clicks back to cell A
- **THEN** cell A's editor renders the staged `assign` entry as still pending

#### Scenario: Summary view lists cells with coverage gaps

- **GIVEN** an admin viewing the assignment board with no cell selected
- **WHEN** the side panel renders the summary view
- **THEN** the summary lists every on-schedule cell where `X < N`
- **AND** the list is sorted ascending by `(weekday, start_time)`
- **AND** each entry is clickable; clicking sets selection to that cell

#### Scenario: Submit with no warnings fires immediately

- **GIVEN** a draft queue where every entry is on a cell the user is qualified for
- **WHEN** the admin clicks Submit
- **THEN** the system replays the draft as `POST` / `DELETE` calls in order, with no confirmation dialog

#### Scenario: Submit with warnings prompts a confirmation dialog

- **GIVEN** a draft queue containing at least one entry with `isUnqualified: true`
- **WHEN** the admin clicks Submit
- **THEN** a dialog opens listing each unqualified entry with user, cell, and reason
- **AND** the admin must click "Confirm and submit" before any API call fires
- **AND** clicking Cancel returns to the draft view with the queue intact

#### Scenario: Per-user hours update live as drafts mutate

- **GIVEN** `员工 1` currently assigned to one 3-hour slot (display: `员工 1 (3h)`)
- **WHEN** the admin stages an additional `assign` draft for `员工 1` on a 2-hour slot
- **THEN** every chip where `员工 1` appears displays `员工 1 (5h)` in real time
- **AND** the summary view's totals reflect the in-flight draft as well

#### Scenario: Submit failure stops the queue and surfaces the failed op

- **GIVEN** a draft queue with three pending operations
- **WHEN** Submit is clicked and the second operation returns a non-2xx response
- **THEN** the first operation is removed from the queue (it succeeded)
- **AND** the second operation remains in the queue with an inline error annotation
- **AND** the third operation remains in the queue (not yet attempted)
- **AND** the admin sees a notification with the failed operation's details
- **AND** the board re-renders to reflect the partial state on the server

#### Scenario: Discard drafts clears the local queue

- **GIVEN** a non-empty draft queue
- **WHEN** the admin clicks "Discard drafts"
- **THEN** the queue is emptied
- **AND** the board re-renders the server state (no projected drafts)
- **AND** no API call is made
