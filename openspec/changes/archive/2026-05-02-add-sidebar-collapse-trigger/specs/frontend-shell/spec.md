## ADDED Requirements

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
