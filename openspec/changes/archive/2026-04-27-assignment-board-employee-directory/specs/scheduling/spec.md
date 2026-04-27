## MODIFIED Requirements

### Requirement: Admin assignment board drag-drop and draft submission

The admin assignment-board UI SHALL render assignments as a 2D grid: rows are the publication's distinct time blocks (sorted by `start_time`, then `end_time`), columns are weekdays Monday-Sunday (column headers SHALL be center-aligned). Each grid cell representing an on-schedule `(slot, weekday)` pair SHALL render:

- A summary header showing `已分配 X / 需求 N` with a status color (`full` when `X = N`, `partial` when `0 < X < N`, `empty` when `X = 0`).
- One **seat block** per `(position, headcount-index)` unit derived from the slot's `template_slot_positions`. A position with `required_headcount = K` produces `K` seats. Each seat is identified by `{ slotID, weekday, positionID, headcountIndex }` where `headcountIndex` is a stable 0-based UI key per `(slot, weekday, position)`. Seats SHALL be rendered vertically stacked under the cell summary header, grouped by position.
- Each seat in one of two states: **filled** (rendering the assigned employee's chip with name, total hours, and an `×` affordance) or **empty** (rendering a placeholder for the position name).
- If the cell has more applied or draft-projected assignments than the position's `required_headcount`, the surplus SHALL render as overflow seats below the regular seat stack so the admin sees the imbalance.

Cells whose `(slot, weekday)` is not in the slot's weekday set SHALL render shaded with a `—` glyph and SHALL render no seats and SHALL NOT be drop targets.

The page SHALL be a two-pane layout: the grid on the left, a fixed-width employee directory on the right. The directory SHALL be **always visible** and independent of any cell selection (selection state is removed). The directory SHALL list every employee who is qualified for at least one position appearing in this publication's slots, deriving the qualification list from the `GET /publications/{id}/assignment-board` response's top-level `employees[]` array. Each directory row SHALL show the employee's name, current total hours (sum across applied + draft assignments), and the employee's qualified position names as small chips. Disabled (`status != 'active'`) and bootstrap-admin users SHALL be excluded server-side before the data reaches the frontend.

The directory SHALL provide a name search box (case-insensitive substring filter) and a sort toggle between **hours-ascending (default)** and **name-ascending**. A small banner above the directory SHALL show the count of cells where `X < N` (e.g., `仍缺 3 个 cell`); when all on-schedule cells are full, the banner SHALL show a "全部 cell 已满" indicator.

The admin SHALL stage assignment changes via two input modalities, both producing entries in a single deferred-submission draft:

- **Drag** an employee — either from a directory row or from a filled seat in the grid — onto a seat in the grid. During the drag, every on-schedule seat SHALL recolor based on whether the dragged employee's `user_positions` contains the seat's `position_id`: **green border** when qualified, **yellow border** when not qualified. Off-schedule cells render no seats and never highlight. Drop semantics:
  - Drop on an empty seat → stage `assign` to that `(slot, weekday, positionID, userID)`.
  - Drop on a filled seat held by a different user → stage `unassign` of the existing assignment + `assign` of the dragged user.
  - Drop on a filled seat held by the same user → no-op.
  - When the source is a filled seat in the grid (cross-cell or cross-position move), the source seat SHALL ALSO stage an `unassign` entry.
- **Click** the seat chip's `×` affordance to stage an `unassign` draft entry for that filled seat. Clicking a chip already staged for `unassign` (rendered with a strikethrough) SHALL cancel the pending `unassign` entry.

The draft SHALL persist across drag interactions and re-renders. Each draft entry that produces a request the admin is permitted but not "default" to make (specifically: assigning a user to a `(slot, position)` whose `position_id` is not in the user's `user_positions`) SHALL be marked with `isUnqualified: true` and SHALL trigger the existing confirmation dialog on Submit.

Clicking Submit SHALL replay the draft as a sequence of `POST /publications/{id}/assignments` and `DELETE /publications/{id}/assignments/{assignment_id}` calls. The UI SHALL display each filled seat chip's running total hours (sum of `slot.end_time − slot.start_time` over the user's currently-applied assignments in this publication, including pending drafts).

This requirement does not change any write API contract. The endpoints `POST /publications/{id}/assignments` and `DELETE /publications/{id}/assignments/{assignment_id}` are the only writes. The `GET /publications/{id}/assignment-board` read response is trimmed by the `Assignment board surfaces non-candidate qualified employees` requirement below: directory data comes from top-level `employees[]`, while per-pair candidate arrays are no longer returned.

#### Scenario: Cell renders explicit seats per position composition

- **GIVEN** a `(slot, weekday)` cell whose composition is `{前台负责人 × 1, 前台助理 × 2}`
- **WHEN** an admin loads the assignment board
- **THEN** the cell renders 3 seat blocks (1 lead seat + 2 assistant seats) stacked vertically under the cell summary
- **AND** filled seats show the assigned employee's chip with hours
- **AND** empty seats show a placeholder labeled with the position name

#### Scenario: Off-schedule cell renders no seats

- **GIVEN** a daytime time block whose slot's weekday set is `{1, 2, 3, 4, 5}` and weekday `6` (Saturday)
- **WHEN** an admin loads the assignment board
- **THEN** the `(time block, Saturday)` cell renders shaded with `—` and no seats
- **AND** dragging any chip over the cell does not produce a drop highlight

#### Scenario: Right-panel directory lists all qualified employees

- **GIVEN** a publication whose slots collectively cover positions `{前台负责人, 前台助理, 外勤负责人, 外勤助理}`
- **WHEN** an admin loads the assignment board
- **THEN** the directory lists every active employee qualified for at least one of those positions
- **AND** each row carries the employee's name, total hours, and qualified position chips
- **AND** the bootstrap admin and disabled users with no current assignments are excluded

#### Scenario: Directory search filters by name

- **WHEN** the admin types `员工 1` into the search box
- **THEN** the directory shows only rows whose name contains `员工 1` (case-insensitive)
- **AND** the rest of the page is unchanged

#### Scenario: Directory sort toggles between hours and name

- **WHEN** the admin selects the sort toggle's "hours" option
- **THEN** the directory orders rows by total hours ascending
- **WHEN** the admin selects the sort toggle's "name" option
- **THEN** the directory orders rows by name ascending via locale-aware comparison

#### Scenario: Drag from directory shows green border on qualified seats

- **GIVEN** an admin dragging `员工 38` whose `user_positions` is `{前台助理}`
- **WHEN** the drag is in progress
- **THEN** every empty or filled seat whose `positionID = 前台助理` renders with a green border
- **AND** every seat whose `positionID ≠ 前台助理` renders with a yellow border
- **AND** off-schedule cells render no seats and remain shaded

#### Scenario: Drop directory chip onto empty seat stages an assign

- **GIVEN** an admin viewing the assignment board
- **WHEN** the admin drags `员工 38` from the directory onto an empty `前台助理` seat at `(slot S, weekday W)`
- **THEN** a draft entry of kind `assign` for `(S, W, 前台助理, 员工 38)` is queued
- **AND** the seat re-renders showing the `员工 38` chip with an "added" hint
- **AND** no API call is made yet

#### Scenario: Drop on a filled seat held by a different user replaces

- **GIVEN** seat `(slot S, weekday W, 前台助理 #0)` is filled by `员工 22`
- **WHEN** the admin drops `员工 38` from the directory onto that seat
- **THEN** a draft entry of kind `unassign` for `员工 22` at that seat is queued
- **AND** a draft entry of kind `assign` for `员工 38` at that seat is queued
- **AND** the seat re-renders showing `员工 38` with an "added" hint and `员工 22` is removed from the cell

#### Scenario: Drop on a filled seat held by the same user is a no-op

- **GIVEN** seat `(slot S, weekday W, 前台助理 #0)` is filled by `员工 38`
- **WHEN** the admin drops `员工 38` (from the directory or from the same seat) back onto that seat
- **THEN** no draft entry is queued
- **AND** the board state is unchanged

#### Scenario: Cross-seat drag from a filled chip moves the assignment

- **GIVEN** seat A at `(slot S1, weekday W1, 前台助理 #0)` is filled by `员工 38`
- **AND** seat B at `(slot S2, weekday W2, 前台助理 #0)` is empty
- **WHEN** the admin drags `员工 38` from seat A onto seat B
- **THEN** a draft entry of kind `unassign` for seat A is queued
- **AND** a draft entry of kind `assign` for seat B with `员工 38` is queued
- **AND** no API call is made yet

#### Scenario: Drop on a seat whose position is unqualified

- **GIVEN** an admin dragging `员工 22` whose `user_positions` is `{前台助理}` only
- **WHEN** the admin drops `员工 22` on a `前台负责人` seat
- **THEN** the drop is accepted
- **AND** the resulting `assign` draft entry is marked with `isUnqualified: true`
- **AND** the chip renders with a warning indicator after the drop

#### Scenario: Click `×` on a filled seat chip stages an unassign

- **GIVEN** seat `(slot S, weekday W, 前台助理 #0)` is filled by `员工 38`
- **WHEN** the admin clicks the `×` affordance on the `员工 38` chip
- **THEN** a draft entry of kind `unassign` for that seat is queued
- **AND** the chip renders with a strikethrough and a "to-remove" hint
- **AND** no API call is made yet

#### Scenario: Click on a chip already staged for unassign cancels the entry

- **GIVEN** an admin who has just clicked the `×` on a filled seat chip, staging an `unassign` entry
- **WHEN** the admin clicks the same chip's body (now strikethrough)
- **THEN** the staged `unassign` entry is removed from the draft
- **AND** the chip returns to its plain filled state

#### Scenario: Gap banner reflects current coverage

- **GIVEN** a publication where 3 on-schedule cells have `X < N` and the rest are full
- **WHEN** the admin loads the assignment board
- **THEN** the gap banner above the directory reads `仍缺 3 个 cell`
- **AND** as the admin stages assigns that close gaps, the banner updates live

#### Scenario: Submit with no warnings fires immediately

- **GIVEN** a draft queue where every entry is on a seat the user is qualified for
- **WHEN** the admin clicks Submit
- **THEN** the system replays the draft as `POST` / `DELETE` calls in order, with no confirmation dialog

#### Scenario: Submit with warnings prompts a confirmation dialog

- **GIVEN** a draft queue containing at least one entry with `isUnqualified: true`
- **WHEN** the admin clicks Submit
- **THEN** a dialog opens listing each unqualified entry with user, cell, and reason
- **AND** the admin must click "Confirm and submit" before any API call fires
- **AND** clicking Cancel returns to the draft view with the queue intact

#### Scenario: Per-user hours update live as drafts mutate

- **GIVEN** `员工 1` currently assigned to one 3-hour slot (display: `员工 1 · 3h` in the directory and seat chip)
- **WHEN** the admin stages an additional `assign` draft for `员工 1` on a 2-hour slot
- **THEN** every chip and directory row where `员工 1` appears displays `员工 1 · 5h` in real time

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

### Requirement: Assignment board surfaces non-candidate qualified employees

`GET /publications/{id}/assignment-board` SHALL return:

- A `slots` array. Each slot carries its position composition. Per `(slot, position)` pair the response SHALL include `assignments` — the list of users currently assigned to that pair. Each assignment entry has shape `{ assignment_id, user_id, name, email }`. Per-pair `candidates` and `non_candidate_qualified` arrays SHALL NOT be returned.
- A top-level `employees` array listing every employee the admin may consider for assignment in this publication. Each entry has shape `{ user_id, name, email, position_ids: int[] }`. The array SHALL be sorted ascending by `user_id`.

Filter rules for `employees`:

- The bootstrap admin user SHALL be excluded.
- Users with `status != 'active'` SHALL be excluded.
- `position_ids` for each user SHALL be the intersection of the user's `user_positions` with the set of `position_id`s appearing in any `template_slot_positions` row of the publication's template. Users whose intersection is empty SHALL be excluded from the array.

The response shape MAY include other top-level fields (e.g., `publication`, summary metadata) without violating this requirement; only the per-pair `candidates` / `non_candidate_qualified` removal and the new top-level `employees` are normative.

The auto-assigner does NOT consume this HTTP response; it queries the underlying tables directly. The shape of this endpoint is therefore decoupled from the auto-assigner's correctness.

#### Scenario: Response carries top-level employees array

- **GIVEN** a publication whose template references positions `P1` and `P2`
- **AND** active users `Alice` qualified for `{P1}`, `Bob` qualified for `{P1, P2}`, and `Carol` qualified for `{P2, P3}` where `P3` does not appear in the template
- **WHEN** an admin fetches `GET /publications/{id}/assignment-board`
- **THEN** the response carries a top-level `employees` array
- **AND** the array contains `Alice` with `position_ids = [P1.id]`, `Bob` with `position_ids = [P1.id, P2.id]`, `Carol` with `position_ids = [P2.id]`
- **AND** the array is sorted ascending by `user_id`

#### Scenario: Bootstrap admin and disabled users are excluded

- **GIVEN** a publication whose `employees` array would otherwise include the bootstrap admin and a disabled user
- **WHEN** an admin fetches `GET /publications/{id}/assignment-board`
- **THEN** the bootstrap admin user does NOT appear in `employees`
- **AND** users with `status != 'active'` do NOT appear in `employees`

#### Scenario: Users with no qualifying intersection are excluded

- **GIVEN** an active user qualified only for positions that do not appear in this publication's template
- **WHEN** an admin fetches `GET /publications/{id}/assignment-board`
- **THEN** that user does NOT appear in `employees`

#### Scenario: Per-pair shape no longer carries candidates or non_candidate_qualified

- **WHEN** an admin fetches `GET /publications/{id}/assignment-board`
- **THEN** each entry under `slots[].positions[]` carries `assignments` and the position composition fields
- **AND** the entry does NOT carry `candidates`
- **AND** the entry does NOT carry `non_candidate_qualified`

#### Scenario: Per-pair assignments shape preserved

- **GIVEN** a `(slot, position)` pair with two currently-applied assignments for `Alice` and `Bob`
- **WHEN** an admin fetches `GET /publications/{id}/assignment-board`
- **THEN** the pair's `assignments` array contains exactly `Alice` and `Bob` with `{ assignment_id, user_id, name, email }` shape
