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

### Requirement: Ordinary record lists use shared DataTable primitives

The Users, Positions, Templates, and Publications top-level list screens SHALL render their ordinary record tables through a shared DataTable renderer backed by TanStack Table and shadcn-compatible Table primitives.

Each migrated list screen SHALL preserve its existing record columns, localized labels, row actions, loading state, empty state, pagination controls, admin gating, mutation side effects, and backend list API contract. The list screens SHALL continue to own their data queries, mutations, dialogs, route navigation, and page state; the shared DataTable renderer SHALL only render a caller-owned TanStack table instance and table-level states.

The shared DataTable foundation SHALL be configured for server/manual table state, including manual pagination in this change and extension points for future manual sorting and filtering. This change SHALL NOT render new sorting controls, filtering controls, search boxes, column-visibility controls, row selection, export controls, or bulk-action controls.

The assignment board grid, roster grid, availability grid, and publication shift-change review table SHALL NOT be migrated to the shared DataTable in this change.

#### Scenario: Users list preserves behavior on the shared table

- **GIVEN** an admin user views `/users`
- **WHEN** the users query returns a page of records
- **THEN** the screen renders the users through the shared DataTable renderer
- **AND** the current user columns, status display, qualification display, edit action, active-status action, loading state, empty state, and pagination behavior remain available
- **AND** changing page requests the corresponding server page rather than paginating only the current client-side rows

#### Scenario: Positions list preserves behavior on the shared table

- **GIVEN** an admin user views `/positions`
- **WHEN** the positions query returns a page of records
- **THEN** the screen renders the positions through the shared DataTable renderer
- **AND** the current position columns, edit action, delete action, loading state, empty state, and pagination behavior remain available
- **AND** changing page requests the corresponding server page rather than paginating only the current client-side rows

#### Scenario: Templates list preserves behavior on the shared table

- **GIVEN** an admin user views `/templates`
- **WHEN** the templates query returns a page of records
- **THEN** the screen renders the templates through the shared DataTable renderer
- **AND** the current template columns, detail navigation, loading state, empty state, and pagination behavior remain available
- **AND** changing page requests the corresponding server page rather than paginating only the current client-side rows

#### Scenario: Publications list preserves behavior on the shared table

- **GIVEN** an admin user views `/publications`
- **WHEN** the publications query returns a page of records
- **THEN** the screen renders the publications through the shared DataTable renderer
- **AND** the current publication columns, state badge, detail navigation, publish action, activate action, end action, loading state, empty state, and pagination behavior remain available
- **AND** changing page requests the corresponding server page rather than paginating only the current client-side rows

#### Scenario: No deferred table controls are rendered

- **WHEN** any migrated list table renders
- **THEN** it does not render sorting controls, filtering controls, search boxes, column-visibility controls, row-selection controls, export controls, or bulk-action controls

#### Scenario: Schedule matrices stay custom

- **WHEN** the assignment board, roster, availability, or publication shift-change review screens render
- **THEN** they keep their existing custom grid or table implementations
- **AND** they are not rendered through the shared DataTable renderer

### Requirement: Date/time form controls use shared shadcn-style wrappers

The frontend SHALL provide shared `DatePicker`, `TimePicker`, and `DateTimePicker` wrappers for user-facing date and time entry. `DatePicker` and `DateTimePicker` SHALL compose shadcn-compatible Popover and Calendar primitives; `TimePicker` SHALL use a shadcn-styled time input rather than a custom time menu.

The wrappers SHALL accept and emit the same external string formats already used by existing forms:

- `DatePicker`: `YYYY-MM-DD`
- `TimePicker`: `HH:MM`
- `DateTimePicker`: `YYYY-MM-DDTHH:mm`

The create-publication dialog, publication planned-active edit form, leave request date-range form, and template-slot time form SHALL use these wrappers. The migration SHALL preserve existing validation behavior, empty-value handling before validation, query parameter formats, and API payload formats.

#### Scenario: Publication datetime values preserve local form format and API conversion

- **GIVEN** an admin enters publication window datetimes in the create-publication dialog
- **WHEN** the form submits
- **THEN** the datetime controls emit `YYYY-MM-DDTHH:mm` form values
- **AND** the publication API request still converts those local datetime values to ISO/RFC3339 strings as it did before this change

#### Scenario: Publication planned-active edit preserves API conversion

- **GIVEN** an admin edits `planned_active_until` on a publication detail page
- **WHEN** the edit form submits
- **THEN** the datetime control emits a `YYYY-MM-DDTHH:mm` form value
- **AND** the publication API request still converts that local datetime value to the backend format used before this change

#### Scenario: Leave request dates preserve preview query parameters

- **GIVEN** a user enters a leave `from` date and `to` date
- **WHEN** the leave preview request is made
- **THEN** the date controls emit `YYYY-MM-DD` form values
- **AND** the preview request still sends `from=YYYY-MM-DD` and `to=YYYY-MM-DD` query parameters

#### Scenario: Template slot times preserve payload format

- **GIVEN** an admin enters a template slot start time and end time
- **WHEN** the slot form submits
- **THEN** the time controls emit `HH:MM` form values
- **AND** the template slot payload still sends `start_time` and `end_time` as `HH:MM`

#### Scenario: Empty date and time values remain valid intermediate form state

- **GIVEN** a form using one of the shared date/time wrappers has not yet passed validation
- **WHEN** the user clears the control or opens the form with no value
- **THEN** the wrapper represents the value as an empty string at the form boundary
- **AND** existing form validation remains responsible for accepting or rejecting submission

#### Scenario: Time entry does not introduce a custom menu

- **WHEN** a user edits a `TimePicker` value
- **THEN** the control uses a styled time input for time entry
- **AND** no custom listbox, popover menu, or natural-language time parser is introduced

### Requirement: Authenticated shell uses floating sidebar and breadcrumbs

The authenticated layout SHALL render a shadcn floating sidebar and a persistent main-content header. The header SHALL contain a visible sidebar trigger, a vertical separator, and breadcrumbs for the current authenticated route. The sidebar SHALL use icon-collapse behavior on desktop so primary navigation icons remain visible when collapsed. Existing sidebar navigation groups, role-based visibility, unread badge behavior, avatar dropdown actions, theme toggle, and logout behavior SHALL remain unchanged.

#### Scenario: Desktop user collapses the floating sidebar from the main header

- **GIVEN** an authenticated user is viewing the desktop layout
- **WHEN** the authenticated shell renders
- **THEN** the sidebar is configured as a floating sidebar with icon-collapse behavior
- **AND** a visible sidebar trigger is rendered in the main content header
- **WHEN** the user activates the trigger
- **THEN** the sidebar collapses to icon mode rather than disappearing entirely

#### Scenario: Breadcrumbs render on top-level authenticated pages

- **WHEN** an authenticated user visits any top-level authenticated route
- **THEN** the main content header renders a breadcrumb page item for the current route
- **AND** the breadcrumb does not prepend `Dashboard` except on the dashboard route itself

#### Scenario: Breadcrumbs render nested route hierarchy

- **WHEN** an authenticated user visits `/leaves/new`
- **THEN** the breadcrumbs render `Leaves` as a link to `/leaves`
- **AND** the current page crumb renders `Request leave`
- **WHEN** an admin visits `/publications/{id}/assignments`
- **THEN** the breadcrumbs render `Publications` as a link to `/publications`
- **AND** the publication detail crumb links to `/publications/{id}`
- **AND** the current page crumb renders `Assignments`

#### Scenario: Breadcrumb detail labels use loaded record names with fallbacks

- **WHEN** an authenticated user visits a publication, template, or leave detail route
- **THEN** the breadcrumb fetches or reuses the current route's detail query
- **AND** the breadcrumb uses the record name or descriptive label when available
- **AND** the breadcrumb renders a stable fallback while the detail query is unavailable

#### Scenario: Breadcrumbs replace duplicate back links

- **WHEN** an authenticated user visits nested pages with breadcrumb parent links
- **THEN** page-level links whose only purpose is returning to the breadcrumb parent are not rendered
- **AND** form cancel controls, dialog close controls, and pagination previous/next controls are unaffected

### Requirement: Application branding appears in UI and settings

The frontend SHALL load application branding through the branding API and use `product_name` instead of hard-coded `Rota` in public entry pages and authenticated shell chrome. While branding is loading or unavailable, the frontend SHALL fall back to `product_name = "Rota"` and `organization_name = ""` so pages remain usable. Frontend product identity rendering SHALL NOT rewrite business data such as template names, publication names, seed data labels, or historical records that happen to contain the word `Rota`.

The authenticated Settings page SHALL render an admin-only Branding section. The section SHALL use the existing shadcn form style and SHALL let administrators edit `product_name` and `organization_name`. Non-admin users SHALL NOT see the Branding section. The form SHALL preserve optimistic concurrency by submitting the current branding version and surfacing a localized conflict error when the backend returns `VERSION_CONFLICT`. Outside this admin Settings section, the frontend SHALL NOT render `organization_name` in the sidebar or shell header in this change.

#### Scenario: Public entry pages use configured product name

- **GIVEN** branding has `product_name = "排班系统"`
- **WHEN** a visitor loads the login, forgot-password, or setup-password page
- **THEN** the page renders `排班系统` in the product identity text
- **AND** no hard-coded `Rota` product identity is shown

#### Scenario: Authenticated shell uses configured product name

- **GIVEN** branding has `product_name = "Scheduling Portal"` and `organization_name = "Operations"`
- **WHEN** an authenticated user loads any authenticated page
- **THEN** the sidebar header renders `Scheduling Portal`
- **AND** shell product identity text does not render hard-coded `Rota`
- **AND** the sidebar header does not render `Operations`

#### Scenario: Business data containing Rota is not rewritten

- **GIVEN** branding has `product_name = "Scheduling Portal"`
- **AND** a template is named `Default Rota`
- **WHEN** the template appears in breadcrumbs, tables, or detail views
- **THEN** the template name still renders as `Default Rota`
- **AND** it is not rewritten to `Default Scheduling Portal`

#### Scenario: Admin edits branding from Settings

- **GIVEN** an authenticated administrator opens `/settings`
- **WHEN** the Branding section loads
- **THEN** the section displays current `product_name` and `organization_name`
- **WHEN** the administrator saves valid updated values
- **THEN** the branding mutation sends both names and the current version
- **AND** the Settings page and shell update to the returned branding values

#### Scenario: Non-admin settings hide branding form

- **GIVEN** an authenticated non-admin user opens `/settings`
- **THEN** the Branding section is not rendered

#### Scenario: Branding validation errors are localized

- **GIVEN** an authenticated administrator opens `/settings`
- **WHEN** the administrator submits a blank product name
- **THEN** the form shows a localized validation error and does not call the update API
- **WHEN** the update API returns `VERSION_CONFLICT`
- **THEN** the form shows a localized conflict error and refetches current branding
