## MODIFIED Requirements

### Requirement: Sidebar navigation grouping

The application sidebar SHALL render its navigation entries grouped by audience, not as a single flat list. Three groups are defined:

- **My schedule** — visible to every authenticated user. Contains, in order: `Dashboard` (`/`), `Roster` (`/roster`), `Availability` (`/availability`), `Requests` (`/requests`, with the unread-count badge per the existing `Pending-count badge excludes pool` requirement), `Leaves` (`/leaves`).
- **Manage** — visible only when `user.is_admin` is true. Contains, in order: `Users` (`/users`), `Positions` (`/positions`), `Templates` (`/templates`), `Publications` (`/publications`).
- **Account** — visible to every authenticated user, rendered after the "Manage" group when present. Contains: `Settings` (`/settings`). The avatar-dropdown footer SHALL continue to expose the `Settings` entry as well; the two entries are intentional duplicates so users who don't discover the avatar dropdown still have a navigation path.

Each group SHALL render with a `<SidebarGroupLabel>` heading. The "Manage" group SHALL NOT be rendered (no empty header) when the viewer is not an admin. Sidebar header (logo) and footer (avatar dropdown) sections are not affected by this requirement.

The localized labels for the `Availability` entry SHALL frame the entry as an action rather than an abstract attribute. Specifically, the Chinese localization for `sidebar.availability` SHALL read "提交空闲时间" (Submit free time), not "可用性" — the previous noun phrasing was unclear about what the page does. The English label SHALL remain `Availability` because that is the standard scheduling-domain term in English; renaming it would introduce friction without benefit. Other labels in this group remain noun phrases, since their actions are obvious from context.

#### Scenario: Employee sees the schedule and account sidebar groups

- **GIVEN** an authenticated user with `is_admin = false`
- **WHEN** the user loads any page in the authenticated layout
- **THEN** the sidebar renders two groups in order: "My schedule" first, "Account" second
- **AND** the "My schedule" group contains 5 items: Dashboard, Roster, Availability, Requests, Leaves
- **AND** the "Account" group contains 1 item: Settings linking to `/settings`
- **AND** no group labeled "Manage" is rendered

#### Scenario: Admin sees all three sidebar groups

- **GIVEN** an authenticated user with `is_admin = true`
- **WHEN** the user loads any page in the authenticated layout
- **THEN** the sidebar renders three groups in order: "My schedule", "Manage", "Account"
- **AND** the "My schedule" group contains 5 items
- **AND** the "Manage" group contains 4 items: Users, Positions, Templates, Publications
- **AND** the "Account" group contains 1 item: Settings linking to `/settings`

#### Scenario: Settings remains in the avatar dropdown

- **WHEN** any authenticated user opens the avatar-footer dropdown
- **THEN** the dropdown still includes a `Settings` item that navigates to `/settings`
- **AND** clicking either the dropdown entry or the sidebar Account-group entry navigates to the same `/settings` page

#### Scenario: Availability entry uses action phrasing in Chinese

- **GIVEN** the user's UI language is set to Chinese
- **WHEN** the user loads any page in the authenticated layout
- **THEN** the `Availability` sidebar entry renders the localized label "提交空闲时间", not "可用性"
- **AND** the localized label for `availability.title` on the `/availability` page also reads "提交空闲时间"
- **WHEN** the user's UI language is set to English
- **THEN** the same entry renders the literal label "Availability"

## ADDED Requirements

### Requirement: Availability page layout

The `/availability` page SHALL render the qualified-shift submission grid as a 2D layout: rows are the user's qualified time blocks (sorted ascending by `start_time`, then `end_time`); columns are weekdays Monday through Sunday. The leftmost column SHALL show the time-block label once per row; the top row SHALL show the weekday name once per column. The page SHALL NOT duplicate the time label inside individual cells.

Each `(time block, weekday)` cell SHALL render in one of two states:

- **Qualified** — when the user has a qualified shift entry for this time block on this weekday. The cell SHALL render a single checkbox horizontally centered, with no inline composition text. The checkbox SHALL reflect the user's current submission state (checked when the `(slot_id, weekday)` tuple is in the user's submitted set, unchecked otherwise) and SHALL fire the existing `onToggle(slot_id, weekday, checked)` callback when changed. The cell SHALL surface position composition (`{position_name} × {required_headcount}` per position, joined by " / ") via a tooltip on hover or focus, mirroring the existing `availability.shift.composition` / `availability.shift.compositionEntry` i18n format. The cell's accessible name SHALL include the weekday, time block, and full composition summary so screen readers receive the same information without needing the tooltip.
- **Off-schedule** — when the time block does not appear in this user's qualified-shift list for this weekday (either because no shift runs at that time on that weekday, or because the user is not qualified for any position in that slot). The cell SHALL render a muted dashed-border container with the `—` glyph and SHALL carry an accessible label of "Off-schedule" / "排班外" via the new `availability.offSchedule` i18n key.

The header row SHALL highlight the column corresponding to today's weekday (Monday = 1 through Sunday = 7) with a primary-tinted background and the localized "Today" / "今天" badge, identical to the roster page.

The grid SHALL be wrapped in `overflow-x-auto` so that on viewports narrower than ~1090px the grid scrolls horizontally rather than collapsing.

When the publication's effective state is not `COLLECTING`, the page-level wrapper continues to render the existing state-specific message instead of the grid; this requirement governs only the grid that renders during `COLLECTING`.

#### Scenario: Time and weekday labels appear once each

- **GIVEN** a user with qualified shifts spanning multiple time blocks across multiple weekdays
- **WHEN** the user loads `/availability` during `COLLECTING`
- **THEN** each distinct `(start_time, end_time)` time block appears exactly once in the leftmost column
- **AND** each weekday Monday through Sunday appears exactly once in the top row
- **AND** no individual cell body repeats the time label or position composition text

#### Scenario: Qualified cell renders only a checkbox

- **GIVEN** a `(time block, weekday)` cell where the user has a qualified shift
- **WHEN** the cell renders
- **THEN** the cell body contains exactly one checkbox and no other interactive controls
- **AND** the checkbox is checked when the `(slot_id, weekday)` is in the user's submitted set, unchecked otherwise
- **WHEN** the user toggles the checkbox
- **THEN** the existing `onToggle(slot_id, weekday, checked)` callback is invoked with the matching values

#### Scenario: Composition is reachable via accessible name and tooltip

- **GIVEN** a qualified cell whose underlying shift requires `Front desk × 2` and `Cashier × 1`
- **WHEN** the cell renders
- **THEN** the cell's accessible name (per the rendered DOM, regardless of tooltip visibility) includes the weekday, time block, and a composition summary equivalent to "Front desk × 2 / Cashier × 1" (localized)
- **AND** a tooltip element with the same composition summary is wired to the cell so sighted users can surface it on hover or focus

#### Scenario: Off-schedule cell renders distinctly

- **GIVEN** a daytime time block that does not appear in this user's qualified-shift list for Saturday
- **WHEN** the user loads `/availability` during `COLLECTING`
- **THEN** the `(daytime block, Saturday)` cell renders with a muted dashed-border container and the `—` glyph
- **AND** the cell carries an accessible label "Off-schedule" / "排班外"
- **AND** the cell does not render any checkbox

#### Scenario: Today column header is highlighted

- **GIVEN** the system clock indicates today's weekday is Wednesday
- **WHEN** the user loads `/availability` during `COLLECTING`
- **THEN** the Wednesday column header carries a primary-tinted background and a "Today" / "今天" badge
- **AND** no other weekday column header carries the primary-tinted highlight

#### Scenario: Empty qualified shifts shows the existing fallback

- **GIVEN** a user who has zero qualified shifts in the current publication
- **WHEN** the user loads `/availability` during `COLLECTING`
- **THEN** the page renders the existing `availability.noQualifiedShifts` fallback message instead of an empty grid

#### Scenario: Narrow viewport scrolls horizontally

- **GIVEN** a viewport narrower than ~1090px
- **WHEN** the user loads `/availability` during `COLLECTING`
- **THEN** the grid does not collapse or wrap rows; horizontal scrolling is enabled via the `overflow-x-auto` wrapper
