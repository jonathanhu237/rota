# frontend-shell Specification

## Purpose
TBD - created by archiving change dashboard-and-sidebar-restructure. Update Purpose after archive.
## Requirements
### Requirement: Authenticated landing page widgets

The authenticated landing page at route `/` SHALL render the following content blocks, in order, top-to-bottom:

1. **Welcome heading** — an `<h1>` displaying a localized greeting that interpolates the current user's name, plus a one-line description paragraph.
2. **Current publication card** — surfaces the single non-`ENDED` publication (if any) with its name, effective-state badge (the existing `<PublicationStateBadge>` component), planned-active window, and exactly one primary call-to-action whose label and target depend on the `(effective state, viewer is_admin)` matrix below. When no non-`ENDED` publication exists, the card body SHALL show a localized "no active publication" message; the admin variant SHALL include a "Create publication" CTA linking to `/publications`, and the employee variant SHALL include no CTA.
3. **To-do card** — rendered only when the viewer's unread shift-change count (per the existing `GET /users/me/notifications/unread-count` endpoint) is greater than zero. The card SHALL show "You have N pending shift changes" (localized) with a CTA linking to `/requests`.
4. **Recent leaves card** — rendered only when the viewer has at least one leave row OR there is an `ACTIVE` publication (so leave is permitted). The card SHALL list up to 3 most-recent leaves with their derived state badges and a "View all" link to `/leaves`.
5. **Manage shortcuts card** — rendered only when the viewer is an admin. The card SHALL display four chip-style links to `/users`, `/positions`, `/templates`, and `/publications`.

The current-publication card's CTA SHALL follow this `(effective state × is_admin)` matrix:

| Effective state | Employee CTA | Admin CTA |
|---|---|---|
| stored `DRAFT` | (no CTA, "preparing" copy) | "View publication" → `/publications/:id` |
| `COLLECTING` | "Submit availability" → `/availability` | "Open publication" → `/publications/:id` |
| `ASSIGNING` | (no CTA, "awaiting assignment" copy) | "Open assignment board" → `/publications/:id/assignments` |
| `PUBLISHED` | "View roster" → `/roster` | "View roster" → `/roster` |
| `ACTIVE` | "View roster" → `/roster` | "View roster" → `/roster` |

The page SHALL load each card's data from independent TanStack queries so that a slow query for one card does not block rendering of the others. Each card MAY render its own skeleton placeholder while loading.

#### Scenario: Employee with active publication sees roster CTA

- **GIVEN** a non-admin user
- **AND** a publication whose effective state is `ACTIVE`
- **WHEN** the user loads `/`
- **THEN** the current-publication card renders the publication's name and an `ACTIVE` badge
- **AND** the primary CTA is "View roster" linking to `/roster`

#### Scenario: Admin during ASSIGNING sees assignment-board CTA

- **GIVEN** an admin user
- **AND** a publication whose effective state is `ASSIGNING`
- **WHEN** the admin loads `/`
- **THEN** the current-publication card's CTA is "Open assignment board" linking to `/publications/:id/assignments`

#### Scenario: No active publication shows empty state

- **GIVEN** no publication exists in any state other than `ENDED`
- **WHEN** an employee loads `/`
- **THEN** the current-publication card renders a "no active publication" message with no CTA
- **WHEN** an admin loads `/`
- **THEN** the current-publication card renders the same message with a "Create publication" CTA linking to `/publications`

#### Scenario: To-do card hidden when no unread requests

- **GIVEN** the viewer's unread shift-change count is 0
- **WHEN** the user loads `/`
- **THEN** the to-do card is not rendered

#### Scenario: Recent leaves card hidden when no leaves and no ACTIVE publication

- **GIVEN** the viewer has zero leave rows
- **AND** no publication is in effective state `ACTIVE`
- **WHEN** the user loads `/`
- **THEN** the recent-leaves card is not rendered

#### Scenario: Manage shortcuts card admin-gated

- **WHEN** a non-admin user loads `/`
- **THEN** the manage-shortcuts card is not rendered
- **WHEN** an admin loads `/`
- **THEN** the manage-shortcuts card renders four chips linking to `/users`, `/positions`, `/templates`, `/publications`

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

#### Scenario: Leaves entry replaces the legacy Leave/My leaves pair

- **WHEN** the sidebar renders for any authenticated user
- **THEN** there is exactly one navigation entry under "My schedule" with the label "Leaves" and target `/leaves`
- **AND** there is no navigation entry with target `/leave` or `/my-leaves`

### Requirement: Leave route namespace

All leave-related authenticated UI routes SHALL live under the `/leaves` URL prefix:

- `/leaves` — the viewer's leave history list, paginated. Top of page SHALL include a primary "Request leave" CTA linking to `/leaves/new`.
- `/leaves/new` — the request flow with multi-row drafts and slot previews.
- `/leaves/:leaveId` — leave detail.

The legacy paths `/leave` (request) and `/my-leaves` (history) SHALL NOT be served. Internal links and tests in the codebase SHALL reference only the `/leaves` prefix.

#### Scenario: Leave history exposes Request CTA

- **GIVEN** an authenticated user
- **WHEN** the user navigates to `/leaves`
- **THEN** the page renders a "Request leave" CTA at the top
- **AND** the CTA links to `/leaves/new`

#### Scenario: Legacy leave paths are not served

- **WHEN** any internal link, sidebar item, or test in the frontend references `/leave` or `/my-leaves`
- **THEN** the reference is updated to `/leaves` or `/leaves/new` as appropriate
- **AND** no production route definitions register `/leave` or `/my-leaves`

#### Scenario: Leave detail remains a child of /leaves

- **GIVEN** a leave with id `K`
- **WHEN** the user clicks a row in the `/leaves` history list
- **THEN** the user is navigated to `/leaves/K`
- **AND** the URL pattern `/leaves/:leaveId` is the only path that renders leave detail

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
