## ADDED Requirements

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
