## ADDED Requirements

### Requirement: Roster page layout

The `/roster` page SHALL render the weekly roster as a 2D grid: rows are the publication's distinct time blocks (sorted ascending by `start_time`, then `end_time`); columns are weekdays Monday through Sunday. The leftmost column SHALL show the time-block label once per row; the top row SHALL show the weekday name once per column. The page SHALL NOT duplicate the time label inside individual cells.

Each `(time block, weekday)` cell SHALL render in one of two states:

- **Scheduled** — when the publication's payload contains a slot for this time block on this weekday. The cell SHALL render:
  - A summary header reading `已分配 X / 需求 N` (assigned X of required N) with a status color: green when `X = N`, amber when `0 < X < N`, red when `X = 0`.
  - A seat stack grouped by position. Each position group SHALL show the position name once at the top, followed by chips for each filled seat and dashed-border placeholder chips reading "空缺" / "Empty" for each unfilled seat (filled count + empty count = `required_headcount`).
  - The cell SHALL NOT repeat the time block's start/end times or per-position headcount inside the cell body.
- **Off-schedule** — when the time block does not run on this weekday in the publication's payload. The cell SHALL render a muted dashed-border container with the `—` glyph, mirroring the assignment-board off-schedule rendering, and SHALL carry an accessible label of "Off-schedule" / "排班外".

The header row SHALL highlight the column corresponding to today's weekday (Monday = 1 through Sunday = 7) with a primary-tinted background and a localized "Today" / "今天" badge. Each scheduled cell in today's column SHALL carry a primary-tinted ring so the highlight extends down the column.

Within each scheduled cell, the seat chip whose `user_id` matches the viewer's user_id SHALL be styled distinctly (border, background, and text using the primary token) to make self-assignments stand out across the week.

When the current publication's effective state is `PUBLISHED` and the viewer is a non-admin user, each self-chip SHALL include a dropdown trigger exposing three actions — `Propose swap`, `Give to direct`, and `Give to pool` — that surface the existing shift-change request flows. The dropdown SHALL NOT be rendered for assignments belonging to other users, and SHALL NOT be rendered when the publication's effective state is not `PUBLISHED`.

The roster grid SHALL be wrapped in `overflow-x-auto` so that on viewports narrower than ~1090px the grid scrolls horizontally rather than collapsing.

#### Scenario: Time and weekday labels appear once each

- **GIVEN** a publication with multiple slots covering distinct time blocks across multiple weekdays
- **WHEN** the user loads `/roster`
- **THEN** each distinct `(start_time, end_time)` time block appears exactly once in the leftmost column
- **AND** each weekday Monday through Sunday appears exactly once in the top row
- **AND** no individual cell body repeats the time label or per-position headcount

#### Scenario: Cell status color reflects coverage

- **GIVEN** a `(time block, weekday)` cell whose total `required_headcount` is 3 and total `assignments.length` is 2
- **WHEN** the user loads `/roster`
- **THEN** the cell renders with the partial-coverage (amber) status color
- **WHEN** the same cell has `assignments.length = 0`
- **THEN** the cell renders with the empty-coverage (red) status color
- **WHEN** the same cell has `assignments.length = 3`
- **THEN** the cell renders with the full-coverage (green) status color

#### Scenario: Off-schedule cell renders distinctly

- **GIVEN** a daytime time block whose slot's weekday set does not include Saturday
- **WHEN** the user loads `/roster`
- **THEN** the `(daytime block, Saturday)` cell renders with a muted dashed-border container and a `—` glyph
- **AND** the cell carries an accessible label "Off-schedule" / "排班外"
- **AND** the cell does not render any seat chips

#### Scenario: Today column is highlighted

- **GIVEN** the system clock indicates today's weekday is Wednesday
- **WHEN** the user loads `/roster`
- **THEN** the Wednesday column header carries a primary-tinted background and a "Today" / "今天" badge
- **AND** every scheduled cell in the Wednesday column carries a primary-tinted ring
- **AND** no other weekday column carries the primary-tinted ring

#### Scenario: Self assignment chip is visually distinct

- **GIVEN** the viewer's `user_id = U`
- **AND** at least one assignment in the displayed week with `user_id = U`
- **WHEN** the roster renders
- **THEN** every chip representing an assignment with `user_id = U` is styled with primary-token border, background, and text
- **AND** chips representing assignments belonging to other users use the default chip styling

#### Scenario: PUBLISHED self chip exposes swap and give actions

- **GIVEN** the current publication's effective state is `PUBLISHED`
- **AND** the viewer is a non-admin user with at least one self assignment in the displayed week
- **WHEN** the user clicks the menu trigger on a self-chip
- **THEN** the dropdown lists three options: `Propose swap`, `Give to direct`, and `Give to pool`
- **AND** clicking any option invokes the existing shift-change request flow with the correct `(slot, position, occurrence_date, assignment_id)` payload

#### Scenario: Non-PUBLISHED state hides the self-chip menu

- **GIVEN** the current publication's effective state is `ACTIVE` (or any state other than `PUBLISHED`)
- **WHEN** the user loads `/roster` and views their own assignment chip
- **THEN** the chip renders without a dropdown trigger

#### Scenario: Empty seats are visible inside scheduled cells

- **GIVEN** a position with `required_headcount = 2` and only one assignment
- **WHEN** the cell renders
- **THEN** the position group shows the position name once, the one filled chip, and one dashed-border placeholder chip reading "空缺" / "Empty"

#### Scenario: Narrow viewport scrolls horizontally

- **GIVEN** a viewport narrower than ~1090px
- **WHEN** the user loads `/roster`
- **THEN** the grid does not collapse or wrap rows; horizontal scrolling is enabled via the `overflow-x-auto` wrapper
