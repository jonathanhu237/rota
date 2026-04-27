## ADDED Requirements

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

The application sidebar SHALL render its navigation entries grouped by audience, not as a single flat list. Two groups are defined:

- **My schedule** — visible to every authenticated user. Contains, in order: `Dashboard` (`/`), `Roster` (`/roster`), `Availability` (`/availability`), `Requests` (`/requests`, with the unread-count badge per the existing `Pending-count badge excludes pool` requirement), `Leaves` (`/leaves`).
- **Manage** — visible only when `user.is_admin` is true. Contains, in order: `Users` (`/users`), `Positions` (`/positions`), `Templates` (`/templates`), `Publications` (`/publications`).

Each group SHALL render with a `<SidebarGroupLabel>` heading. The "Manage" group SHALL NOT be rendered (no empty header) when the viewer is not an admin. Sidebar header (logo) and footer (avatar dropdown) sections are not affected by this requirement.

#### Scenario: Employee sees one sidebar group

- **GIVEN** an authenticated user with `is_admin = false`
- **WHEN** the user loads any page in the authenticated layout
- **THEN** the sidebar renders exactly one group labeled "My schedule"
- **AND** the group contains 5 items: Dashboard, Roster, Availability, Requests, Leaves
- **AND** no group labeled "Manage" is rendered

#### Scenario: Admin sees both sidebar groups

- **GIVEN** an authenticated user with `is_admin = true`
- **WHEN** the user loads any page in the authenticated layout
- **THEN** the sidebar renders two groups: "My schedule" first, "Manage" second
- **AND** the "My schedule" group contains 5 items
- **AND** the "Manage" group contains 4 items: Users, Positions, Templates, Publications

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
