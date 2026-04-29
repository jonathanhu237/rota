## ADDED Requirements

### Requirement: Assignment board admin feedback

The `/publications/:id/assignments` page (the assignment board) SHALL communicate two orthogonal dimensions of every chip and drag-target separately:

1. **Draft state** — whether an edit has been persisted to the backend or is still a client-side draft awaiting submit.
2. **Override severity** — whether placing the chip required the admin to bypass either the qualification gate (severe) or the availability-submission gate (mild).

The visual contract SHALL be:

- **Routine pending-add chip** (qualified + the employee submitted availability for this `(slot, weekday)`) renders identically to a saved assignment chip except for a small filled circle dot rendered before the employee name (sized `size-1.5`, colored `bg-primary`). No inline `[新增]` text badge, no left-border accent, no ring outline.
- **Pending-remove chip** (admin removed an existing assignment, not yet submitted) keeps the chip visible with `line-through` text in a muted color and the chip container at `opacity-70` so the removal is unmistakable visually, replacing the `<X>` icon with a `<Undo2>` icon. Clicking the icon (or the chip body) cancels the removal in one click. No inline `[移除]` text badge.
- **Override chip** carries `<AlertTriangle>` icons sized `size-3.5`. Color encodes severity: `text-red-500` when `isUnqualified`, `text-amber-500` when `isUnsubmitted` and not `isUnqualified`. When both flags are true the chip shows only the red icon.
- **Drag-target highlight** colorizes the cell border when the admin starts dragging an employee from the directory. The color SHALL key off the dragged employee's relationship to the target `(slot, weekday, position)`:
  - Qualified for the position AND submitted availability for `(slot, weekday)` → green border + green-tinted background.
  - Qualified for the position AND did NOT submit availability for `(slot, weekday)` → amber border + amber-tinted background.
  - NOT qualified for the position → red border + red-tinted background (this rule wins over the submission check when both apply).
- **Submit confirmation dialog** SHALL gate the submit action whenever the pending draft set contains at least one override (unqualified or unsubmitted). The dialog SHALL render up to two stacked sections in a single dialog: red "资格不匹配" section listing each unqualified draft, then amber "未提交空闲时间" section listing each unsubmitted draft. A single confirm button commits all drafts. When only one severity is present, only that section renders and the title adjusts. When no overrides exist, no dialog renders and submit proceeds directly.
- **Employee directory** (the right-hand side panel) SHALL split into two stacked sections:
  - "已提交空闲时间 (X)" — employees whose `submitted_slots` is non-empty. This section displays per-employee hours, supports the existing search and sort affordances, and feeds the directory stats (mean, range, stddev).
  - "未提交空闲时间 (Y)" — employees with empty `submitted_slots`. This section displays employees alphabetically by name, omits per-employee hours, and is rendered with a muted background so it visually recedes. Drag from this section is permitted (admin override).
- **Directory stats** SHALL describe only the submitter subset. The "X 人无班" warning is replaced with "X 人未排上" rendered in `text-amber-700 dark:text-amber-300` (consistent with the system's amber = "mild warning" gradient), and renders only when at least one submitter has zero assigned hours; non-submitters do not contribute to this count.
- **Seat order stability** SHALL preserve the visual order of saved assignments while pending-add drafts are present. Saved assignments stay in their original seat positions (the backend's `(slot, weekday, position)`-grouped order) regardless of any pending-add drafts in the same cell. Pending-add chips SHALL render after all saved chips, in the order the admin queued them, so a new pending-add never displaces an existing chip from its seat. Pending-remove chips render at the end of the cell (after pending-adds), since the underlying assignment is being filtered out of the projection — preserving their exact original seat position would require a deeper projection rewrite and is deferred.
- **Pending-changes counter** SHALL display the count of unsaved draft operations alongside discard and submit buttons. The label is "未提交的更改：N 项".
- **Beforeunload guard** SHALL register a native browser confirmation prompt whenever the draft set is non-empty, so refresh / tab close / cross-origin navigation does not silently drop unsubmitted edits.

#### Scenario: Routine pending-add chip uses a small dot indicator

- **GIVEN** a draft assignment placing a qualified employee who submitted availability into a previously-empty seat
- **WHEN** the chip renders before submit
- **THEN** the chip displays the employee's name and an X icon, with no inline "新增" text badge
- **AND** the chip renders a small filled dot (`size-1.5`, `bg-primary`) immediately before the employee name
- **AND** the chip container carries no left-border accent and no ring outline
- **AND** no `<AlertTriangle>` icon is rendered on the chip

#### Scenario: Pending-remove chip uses strikethrough and undo icon

- **GIVEN** an existing assignment that the admin marked for removal
- **WHEN** the chip renders before submit
- **THEN** the employee name carries `line-through` styling in a muted color
- **AND** the chip container carries `opacity-70` so the chip visibly fades
- **AND** the trailing icon is `<Undo2>` instead of `<X>`
- **AND** clicking the icon or chip body cancels the removal and restores the chip to its saved appearance

#### Scenario: Drag-highlight is green when qualification and submission both match

- **GIVEN** an admin starts dragging an employee from the directory
- **AND** the employee is qualified for some position `P` AND has a `submitted_slots` entry for `(slot S, weekday W)`
- **WHEN** the admin hovers a target seat in `(S, W, P)`
- **THEN** the cell border is green and the background is green-tinted

#### Scenario: Drag-highlight is amber when qualified but did not submit

- **GIVEN** an admin starts dragging an employee from the directory
- **AND** the employee is qualified for some position `P` AND does NOT have a `submitted_slots` entry for `(slot S, weekday W)`
- **WHEN** the admin hovers a target seat in `(S, W, P)`
- **THEN** the cell border is amber and the background is amber-tinted

#### Scenario: Drag-highlight is red when not qualified

- **GIVEN** an admin starts dragging an employee from the directory
- **AND** the employee is NOT qualified for some position `P`, regardless of submission state
- **WHEN** the admin hovers a target seat in `(_, _, P)`
- **THEN** the cell border is red and the background is red-tinted

#### Scenario: Dropped chip carries the matching severity icon

- **GIVEN** the admin drops an unqualified employee onto a seat
- **WHEN** the resulting draft chip renders
- **THEN** the chip carries an `<AlertTriangle>` icon styled `text-red-500`
- **WHEN** the admin instead drops a qualified-but-unsubmitted employee
- **THEN** the chip carries an `<AlertTriangle>` icon styled `text-amber-500`
- **WHEN** the admin drops an employee who is both unqualified AND unsubmitted
- **THEN** the chip carries the red icon only — the amber icon is suppressed

#### Scenario: Submit dialog merges both override categories

- **GIVEN** the admin's draft set contains 2 unqualified drafts and 3 unsubmitted drafts
- **WHEN** the admin clicks the submit button
- **THEN** a single confirmation dialog opens with two stacked sections: a red "资格不匹配 (2)" section listing the 2 unqualified drafts, and below it an amber "未提交可用性 (3)" section listing the 3 unsubmitted drafts
- **AND** the dialog's confirm button commits all 5 drafts when clicked

#### Scenario: Submit dialog title adapts when only one severity is present

- **GIVEN** the admin's draft set contains only unsubmitted drafts (no unqualified)
- **WHEN** the admin clicks submit
- **THEN** the dialog opens with title "确认可用性破例"
- **AND** only the amber section renders
- **WHEN** the admin's draft set contains only unqualified drafts (no unsubmitted)
- **THEN** the dialog opens with the existing "确认资格覆盖" title and only the red section
- **WHEN** the admin's draft set contains neither severity
- **THEN** no dialog opens and submit proceeds directly

#### Scenario: Employee directory splits into submitter and non-submitter sections

- **GIVEN** the publication has 28 active qualified employees with at least one submission and 7 with none
- **WHEN** the admin opens the assignment board
- **THEN** the directory shows a "已提交空闲时间 (28)" section followed by a "未提交空闲时间 (7)" section
- **AND** the upper section is sortable by hours or name
- **AND** the lower section is sorted alphabetically by name and renders against a muted background
- **AND** the lower section's rows omit hours and are still draggable

#### Scenario: Directory stats describe submitters only

- **GIVEN** 28 submitters whose hours have stddev = 1.6 and 7 non-submitters at 0 hours
- **WHEN** the directory stats line renders
- **THEN** the displayed mean, range, and stddev are computed over the 28 submitters
- **AND** the "X 人未排上" indicator only renders if at least one submitter has zero assigned hours
- **AND** non-submitters do not contribute to "X 人未排上"

#### Scenario: Beforeunload prompt fires only when drafts exist

- **GIVEN** the admin has at least one pending draft operation
- **WHEN** the admin attempts to refresh the page, close the tab, or navigate to a different origin
- **THEN** the browser displays its native "leave site? changes you made may not be saved" prompt
- **WHEN** the admin's draft set is empty
- **THEN** no prompt fires

#### Scenario: Pending-add drafts append after saved chips and never displace them

- **GIVEN** a `(slot, weekday, position)` cell with two saved assignments rendered in seats 1 and 2 (sorted ascending by `assignment_id`)
- **WHEN** the admin drags a directory employee onto the empty seat 3 of that cell
- **THEN** the saved assignments remain in seats 1 and 2 in their original order
- **AND** the new draft chip renders in seat 3 (after the two saved chips)
- **AND** the saved chips do not visually shift position even though the draft is unsaved
- **WHEN** the admin then queues a second pending-add draft against the same cell
- **THEN** the second draft chip renders after the first draft chip (drafts retain queue order, both render after all saved chips)
