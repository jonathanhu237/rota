## ADDED Requirements

### Requirement: Authenticated sidebar exposes desktop collapse control

The authenticated layout sidebar SHALL expose a visible desktop control for collapsing and expanding navigation chrome. Desktop collapse SHALL use the shadcn icon-collapse mode so primary navigation icons remain visible while labels and group headings are hidden by the sidebar primitive. Mobile behavior SHALL continue to use the existing mobile header trigger and sheet navigation.

#### Scenario: Desktop user collapses the sidebar through a visible trigger

- **GIVEN** an authenticated user is viewing the desktop layout
- **WHEN** the sidebar renders
- **THEN** a visible toggle control with an accessible name for collapsing or expanding navigation is rendered in the sidebar header
- **AND** activating the control collapses the sidebar to icon mode rather than removing the desktop sidebar entirely

#### Scenario: Mobile trigger remains the entry point

- **GIVEN** an authenticated user is viewing the mobile layout
- **WHEN** the authenticated header renders
- **THEN** the existing mobile navigation trigger remains available
- **AND** no duplicate desktop-only sidebar collapse control is required in the mobile sheet header
