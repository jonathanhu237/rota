## ADDED Requirements

### Requirement: Admin assignment board drag-drop and draft submission

The admin assignment-board UI SHALL support drag-and-drop manual adjustment with deferred submission. Drags accumulate as a draft locally; clicking Submit replays them as a sequence of `POST /publications/{id}/assignments` and `DELETE /publications/{id}/assignments/{assignment_id}` calls. Each draft entry that produces a request the admin is permitted but not "default" to make (specifically: assigning a user to a `(slot, position)` whose `position_id` is not in the user's `user_positions`) SHALL be marked with a warning indicator. Submit SHALL trigger a confirmation dialog if any pending entries carry warnings; if none do, Submit SHALL fire without confirmation.

The UI SHALL display each assigned user's running total hours (sum of `slot.end_time − slot.start_time` over the user's currently-applied assignments in this publication, including pending drafts) inside their cell badge.

The existing `+ / ×` per-user buttons on the board SHALL retain immediate-commit semantics for keyboard / accessibility / immediate-edit users; only the drag-drop interaction uses the draft model.

This requirement does not change any backend API contract. The endpoints `POST /publications/{id}/assignments` and `DELETE /publications/{id}/assignments/{assignment_id}` are the only writes.

#### Scenario: Drag a candidate to an open slot

- **GIVEN** an admin viewing the assignment board for a publication in `ASSIGNING`, `PUBLISHED`, or `ACTIVE`
- **WHEN** the admin drags a user from the candidate panel onto an empty slot in a cell
- **THEN** the cell renders the user with an "added" visual hint
- **AND** a draft entry of kind `assign` is queued
- **AND** no API call is made yet

#### Scenario: Drag from cell to cell swaps users

- **GIVEN** Alice currently assigned in cell A and Bob currently assigned in cell B
- **WHEN** the admin drags Alice onto Bob
- **THEN** the board projects Alice in cell B and Bob in cell A
- **AND** four draft entries are queued: unassign Alice from A, unassign Bob from B, assign Alice to B, assign Bob to A
- **AND** no API call is made yet

#### Scenario: Drop on a cell whose position the user is not qualified for

- **GIVEN** an admin dragging Alice (qualified for Cashier only)
- **WHEN** the admin drops Alice on a Cook cell
- **THEN** the drop is accepted
- **AND** the draft entry is marked with a warning (`isUnqualified: true`)
- **AND** the cell renders Alice with a warning indicator

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

- **GIVEN** Alice currently assigned to one 3-hour slot (display: `Alice (3h)`)
- **WHEN** the admin drags Alice to an additional 2-hour slot (creating an `assign` draft)
- **THEN** every cell where Alice appears displays `Alice (5h)` in real time
- **AND** the candidate panel (if Alice appears there) also reflects 5h

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

#### Scenario: Existing + / × buttons remain immediate-commit

- **GIVEN** the existing `+` (assign) and `×` (unassign) buttons on a cell
- **WHEN** the admin clicks `+` next to a candidate
- **THEN** a single `POST /publications/{id}/assignments` fires immediately (no draft, no warning dialog)
- **AND** the same applies to `×` for `DELETE`
