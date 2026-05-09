## MODIFIED Requirements

### Requirement: Sidebar navigation grouping

The application sidebar SHALL render its navigation entries grouped by audience, not as a single flat list. Three groups are defined:

- **My schedule** — visible to every authenticated user. Contains, in order: `Dashboard` (`/`), `Roster` (`/roster`), `Attendance` (`/attendance`), `Availability` (`/availability`), `Requests` (`/requests`, with the unread-count badge per the existing `Pending-count badge excludes pool` requirement), `Leaves` (`/leaves`).
- **Manage** — visible only when `user.is_admin` is true. Contains, in order: `Users` (`/users`), `Positions` (`/positions`), `Templates` (`/templates`), `Publications` (`/publications`).
- **Account** — visible to every authenticated user, rendered after the "Manage" group when present. Contains: `Settings` (`/settings`). The avatar-dropdown footer SHALL continue to expose the `Settings` entry as well; the two entries are intentional duplicates so users who don't discover the avatar dropdown still have a navigation path.

Each group SHALL render with a `<SidebarGroupLabel>` heading. The "Manage" group SHALL NOT be rendered (no empty header) when the viewer is not an admin. Sidebar header (logo) and footer (avatar dropdown) sections are not affected by this requirement.

The localized labels for the `Availability` entry SHALL frame the entry as an action rather than an abstract attribute. Specifically, the Chinese localization for `sidebar.availability` SHALL read "提交空闲时间" (Submit free time), not "可用性" — the previous noun phrasing was unclear about what the page does. The English label SHALL remain `Availability` because that is the standard scheduling-domain term in English; renaming it would introduce friction without benefit. Other labels in this group remain noun phrases, since their actions are obvious from context.

#### Scenario: Employee sees the schedule and account sidebar groups

- **GIVEN** an authenticated user with `is_admin = false`
- **WHEN** the user loads any page in the authenticated layout
- **THEN** the sidebar renders two groups in order: "My schedule" first, "Account" second
- **AND** the "My schedule" group contains 6 items: Dashboard, Roster, Attendance, Availability, Requests, Leaves
- **AND** the "Account" group contains 1 item: Settings linking to `/settings`
- **AND** no group labeled "Manage" is rendered

#### Scenario: Admin sees all three sidebar groups

- **GIVEN** an authenticated user with `is_admin = true`
- **WHEN** the user loads any page in the authenticated layout
- **THEN** the sidebar renders three groups in order: "My schedule", "Manage", "Account"
- **AND** the "My schedule" group contains 6 items
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

## ADDED Requirements

### Requirement: Attendance route namespace

Authenticated attendance UI SHALL expose:

- `/attendance` — leader attendance entry for current and overtime-window responsible shifts.
- `/publications/:publicationId/attendance` — administrator attendance management for a publication.

The `/attendance` page SHALL be available to all authenticated users but SHALL show actionable shift cards only when the viewer is the actual responsible user for an eligible shift. The administrator route SHALL require an admin user.

#### Scenario: Leader sees current attendance shift

- **GIVEN** Alice is the responsible user for a currently running shift
- **WHEN** Alice navigates to `/attendance`
- **THEN** the page renders that shift with roster arrival controls

#### Scenario: Non-leader sees attendance empty state

- **GIVEN** Bob is not responsible for any current or overtime-window shift
- **WHEN** Bob navigates to `/attendance`
- **THEN** the page renders an empty state with no arrival or overtime controls

#### Scenario: Non-admin cannot open publication attendance management

- **GIVEN** Bob is not an admin
- **WHEN** Bob navigates to `/publications/1/attendance`
- **THEN** the route denies access through the existing admin-gating behavior

### Requirement: Leader attendance page supports arrival and overtime workflows

The `/attendance` page SHALL list each eligible responsible shift with schedule context, actual roster users, derived arrival status, and existing overtime records. During the shift's arrival window, each roster user without an arrival SHALL expose an arrival action whose default time is the scheduled start and whose editable time cannot be earlier than scheduled start or later than the current time. During the overtime entry window, the page SHALL expose an overtime action that accepts active user, decimal hours, and required note.

#### Scenario: Arrival action defaults to scheduled start

- **GIVEN** Alice opens `/attendance` for a currently running responsible shift
- **WHEN** Alice opens Bob's arrival action
- **THEN** the arrival time control defaults to the shift's scheduled start

#### Scenario: Recorded arrival becomes locked in leader UI

- **GIVEN** Alice records Bob's arrival
- **WHEN** the `/attendance` page refreshes
- **THEN** Bob's arrival is displayed as recorded
- **AND** no leader edit or clear action is shown for Bob's arrival

#### Scenario: Overtime action requires note

- **GIVEN** Alice opens overtime entry for an eligible responsible shift
- **WHEN** Alice enters hours but leaves note blank
- **THEN** the client prevents submission or surfaces the backend `INVALID_REQUEST` error

### Requirement: Publication attendance management page supports corrections

The `/publications/:publicationId/attendance` page SHALL let administrators select an occurrence date, view the publication's shifts for that date, open a shift detail, correct arrivals, manage overtime records, update the publication overtime entry window, and see orphan arrival records that no longer match the current roster.

#### Scenario: Admin views shift attendance by date

- **GIVEN** an admin opens `/publications/1/attendance`
- **WHEN** the admin selects `2026-05-10`
- **THEN** the page lists the publication shifts whose occurrences fall on that date

#### Scenario: Admin correction controls are available

- **GIVEN** an admin opens a shift detail on the attendance management page
- **WHEN** a roster user's attendance row is shown
- **THEN** the admin can set, change, or clear that user's arrival time

#### Scenario: Admin manages overtime records

- **GIVEN** an admin opens a shift detail on the attendance management page
- **WHEN** overtime records are shown
- **THEN** the admin can create, edit, and delete overtime records

#### Scenario: Admin updates overtime window

- **GIVEN** an admin opens publication attendance management
- **WHEN** the admin sets the overtime entry window to `12.5` hours
- **THEN** the page persists the setting and subsequent leader overtime entry uses that value

#### Scenario: Orphan records are visually separated

- **GIVEN** an arrival record no longer matches the current actual roster after a schedule change
- **WHEN** an admin opens the affected shift detail
- **THEN** the page shows the record in a separate orphan records area
