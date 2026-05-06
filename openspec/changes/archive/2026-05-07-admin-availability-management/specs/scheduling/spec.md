## ADDED Requirements

### Requirement: Admin availability management endpoints

The system SHALL expose the following endpoints, all requiring `RequireAdmin`:

- `GET /publications/{id}/availability-board?page=1&page_size=10&search=...`
- `GET /publications/{id}/availability-submissions/{user_id}`
- `PUT /publications/{id}/availability-submissions/{user_id}`

`GET /publications/{id}/availability-board` SHALL be readable in every publication effective state. It SHALL return the publication summary, pagination metadata, and employees who satisfy all of these filters: `users.status = 'active'`, not the bootstrap admin user, and at least one current `user_positions` row overlaps a position used by the publication template. Employees with zero submitted availability rows SHALL be included. The endpoint SHALL support case-insensitive search by employee name or email, normalize invalid pagination as `INVALID_REQUEST`, use default `page_size = 10`, and cap `page_size` at `100`.

`GET /publications/{id}/availability-submissions/{user_id}` SHALL be readable in every publication effective state. It SHALL return the publication summary, target employee, target employee positions, template slot-weekday grid, the target employee's submitted `(slot_id, weekday)` pairs, and eligibility for each grid cell. The target user SHALL be active, not the bootstrap admin, and relevant to the publication template.

#### Scenario: Board includes active relevant employees with zero submissions

- **GIVEN** a publication whose template uses position `P`
- **AND** active employee Alice has position `P` and has no `availability_submissions` rows for the publication
- **WHEN** an admin calls `GET /publications/{id}/availability-board`
- **THEN** Alice appears in the paginated result with submitted count `0`

#### Scenario: Board excludes disabled and irrelevant users

- **GIVEN** a disabled employee Bob has a qualifying position for the publication template
- **AND** active employee Carol has no position used by the publication template
- **WHEN** an admin calls `GET /publications/{id}/availability-board`
- **THEN** neither Bob nor Carol appears in the paginated result

#### Scenario: Board search filters by name or email

- **GIVEN** Alice and Bob are both active relevant employees for a publication
- **WHEN** an admin calls `GET /publications/{id}/availability-board?search=alice`
- **THEN** the response includes Alice
- **AND** the response excludes Bob when Bob's name and email do not match the search term

#### Scenario: Detail marks ineligible submitted cells as removable exceptions

- **GIVEN** active employee Alice has a persisted submission for `(slot S, weekday 3)`
- **AND** Alice no longer has any position used by slot `S`
- **WHEN** an admin calls `GET /publications/{id}/availability-submissions/{alice_id}`
- **THEN** the response includes `(S, 3)` in the submitted set
- **AND** the response marks `(S, 3)` as not eligible for the final saved target set

### Requirement: Admin availability replacement is atomic and qualification-gated

`PUT /publications/{id}/availability-submissions/{user_id}` SHALL accept body `{ submissions: [{ slot_id, weekday }] }`. The `submissions` array SHALL represent the complete target availability set for the target employee under the publication. The server SHALL normalize duplicate pairs as a set. An empty array SHALL be valid and SHALL clear all of the target employee's submissions for the publication.

Admin availability replacement SHALL be allowed only while the publication effective state is `COLLECTING` or `ASSIGNING`. Requests in `DRAFT`, `PUBLISHED`, `ACTIVE`, or `ENDED` SHALL be rejected with HTTP 409 and error code `PUBLICATION_NOT_MUTABLE`. This admin write window SHALL NOT change the employee self-service submission window, which remains `COLLECTING` only.

The replacement SHALL run in one database transaction. Before mutating rows, the service SHALL validate that the publication exists, the target user exists and is active, the target user is not the bootstrap admin, the target user is relevant to the publication template, each target pair belongs to the publication template's slot-weekday grid, and each target pair overlaps the target user's current positions with that slot's composition. If any validation or write fails, the database SHALL remain unchanged for that employee's publication submissions.

Previously persisted rows that are no longer eligible MAY be removed by omitting them from the target set. Ineligible rows SHALL NOT remain in the final target set.

#### Scenario: Admin replaces one employee's availability in ASSIGNING

- **GIVEN** a publication whose effective state is `ASSIGNING`
- **AND** active employee Alice is qualified for slots `S1` and `S2`
- **AND** Alice currently has a persisted submission for `(S1, 1)`
- **WHEN** an admin calls `PUT /publications/{id}/availability-submissions/{alice_id}` with target set `[(S2, 2)]`
- **THEN** Alice's persisted set for that publication becomes exactly `[(S2, 2)]`
- **AND** no other employee's submissions are changed

#### Scenario: Empty replacement clears submissions

- **GIVEN** active employee Alice has two persisted submissions for a publication in `COLLECTING`
- **WHEN** an admin calls `PUT /publications/{id}/availability-submissions/{alice_id}` with `{ "submissions": [] }`
- **THEN** Alice has zero persisted submissions for that publication

#### Scenario: Ineligible final cell rejects the whole replacement

- **GIVEN** active employee Alice is not qualified for slot `S`
- **AND** Alice currently has a persisted submission for `(S1, 1)`
- **WHEN** an admin calls `PUT /publications/{id}/availability-submissions/{alice_id}` with target set `[(S, 3)]`
- **THEN** the response is HTTP 403 with error code `NOT_QUALIFIED`
- **AND** Alice's persisted set remains exactly `[(S1, 1)]`

#### Scenario: Invalid template cell rejects the whole replacement

- **GIVEN** slot `S` belongs to a different template than the publication's template
- **WHEN** an admin calls `PUT /publications/{id}/availability-submissions/{user_id}` with target set `[(S, 1)]`
- **THEN** the response is HTTP 400 with error code `INVALID_REQUEST`
- **AND** no availability rows are changed

#### Scenario: Replacement outside mutable states is rejected

- **GIVEN** a publication whose effective state is `PUBLISHED`
- **WHEN** an admin calls `PUT /publications/{id}/availability-submissions/{user_id}` with any target set
- **THEN** the response is HTTP 409 with error code `PUBLICATION_NOT_MUTABLE`
- **AND** no availability rows are changed

### Requirement: Admin availability management UI

The frontend SHALL expose availability management from the publication scheduling area. The publication detail page SHALL show an admin action for managing availability, and the admin assignment board SHALL show an action that navigates to the same availability board for that publication.

The publication route structure SHALL render publication subpages as standalone pages:

- `/publications/:publicationId` shows the publication detail page.
- `/publications/:publicationId/assignments` shows the assignment board without the publication detail card above it.
- `/publications/:publicationId/availability` shows the admin availability table.
- `/publications/:publicationId/availability/:userId` shows the single-employee availability editor.
- `/publications/:publicationId/shift-changes` shows shift-change administration without the publication detail card above it.

The availability table SHALL support pagination and search, resetting to page `1` when the search term changes. It SHALL show employee name, email, current qualification chips, submitted slot count, and an action to open that employee's editor.

The editor SHALL render the publication template grid and a local draft of the target employee's submitted cells. Eligible cells SHALL be editable. Ineligible cells that are not submitted SHALL be disabled. Ineligible cells that are already submitted SHALL be shown as exceptions that can be unchecked but cannot be rechecked after removal. Off-schedule cells SHALL be blank or disabled according to the existing template grid behavior.

The editor SHALL show a bottom save bar while there are unsaved changes, including unsaved change count, discard, and save actions. Browser refresh, back navigation, and route navigation SHALL prompt while the draft is dirty. On successful save, the editor SHALL remain on the same employee page, show a success notification, and refresh the employee availability data.

The editor SHALL be read-only when the publication effective state is not `COLLECTING` or `ASSIGNING`. It SHALL display a localized note that availability edits affect future auto-assign candidate pools only and do not automatically change existing assignments. The editor SHALL NOT display the current assignment schedule.

All new user-facing labels, helper text, button text, toasts, and errors SHALL be localized through the existing `zh` and `en` locale files.

#### Scenario: Assignment board availability action opens a standalone page

- **GIVEN** an admin is viewing `/publications/1/assignments`
- **WHEN** the admin clicks the availability management action
- **THEN** the app navigates to `/publications/1/availability`
- **AND** the publication detail card is not rendered above the availability table

#### Scenario: Search resets pagination

- **GIVEN** an admin is on page `3` of `/publications/1/availability`
- **WHEN** the admin changes the search term
- **THEN** the frontend requests page `1` for the new search term

#### Scenario: Editor prevents saving an ineligible checked cell

- **GIVEN** an employee is not qualified for a template cell
- **WHEN** an admin opens that employee's availability editor
- **THEN** the cell cannot be checked in the editor

#### Scenario: Ineligible submitted exception can be cleared

- **GIVEN** an employee has a submitted cell that is no longer eligible
- **WHEN** an admin opens that employee's availability editor
- **THEN** the cell is visible as submitted
- **AND** the admin can uncheck it before saving

#### Scenario: Read-only publication state disables save

- **GIVEN** a publication whose effective state is `ACTIVE`
- **WHEN** an admin opens an employee availability editor
- **THEN** availability cells are not editable
- **AND** the save action is not available

#### Scenario: Dirty draft prompts before leaving

- **GIVEN** an admin has unsaved availability changes in the editor
- **WHEN** the admin attempts to navigate away or refresh the page
- **THEN** the frontend prompts before losing the draft

## MODIFIED Requirements

### Requirement: Scheduling error code catalog

The scheduling subsystem SHALL emit the following JSON `error.code` values with the HTTP statuses given:

- `INVALID_REQUEST` (400) - malformed body/path/query or generic `ErrInvalidInput`.
- `INVALID_PUBLICATION_WINDOW` (400) - window does not satisfy `start < end <= planned_active_from < planned_active_until`.
- `INVALID_OCCURRENCE_DATE` (400) - `occurrence_date` outside the publication's active window, weekday mismatch with the slot, occurrence start time `<= NOW()` at request creation, or roster `?week` parameter outside the window or not a Monday.
- `SHIFT_CHANGE_INVALID_TYPE` (400) - unknown request type, or wrong counterpart fields for the type, or `type = 'swap'` on a leave creation.
- `SHIFT_CHANGE_SELF` (400) - counterpart or claimer is the requester themselves.
- `PUBLICATION_NOT_FOUND` (404) - no row, or effective-state resolution requested for a missing publication.
- `TEMPLATE_NOT_FOUND` (404) - referenced template missing.
- `TEMPLATE_SLOT_NOT_FOUND` (404) - slot not found for the given template.
- `TEMPLATE_SLOT_POSITION_NOT_FOUND` (404) - position composition row not found for the given slot.
- `USER_NOT_FOUND` (404) - referenced user missing, hidden from the target operation, or not relevant to the target publication template.
- `SHIFT_CHANGE_NOT_FOUND` (404) - request missing or hidden from the viewer.
- `LEAVE_NOT_FOUND` (404) - leave row missing.
- `NOT_QUALIFIED` (403) - employee attempts a submission or approval for a `(slot, position)` they lack, or an admin availability replacement leaves a target cell that does not overlap the target user's current positions.
- `SHIFT_CHANGE_NOT_OWNER` (403) - caller is not the request's requester, counterpart, or eligible claimer.
- `SHIFT_CHANGE_NOT_QUALIFIED` (403) - swap or give counterpart is not mutually qualified.
- `LEAVE_NOT_OWNER` (403) - caller is not the leave's `user_id` on a cancel attempt.
- `PUBLICATION_ALREADY_EXISTS` (409) - create request violates the single-non-ENDED invariant.
- `PUBLICATION_NOT_DELETABLE` (409) - delete request on a non-`DRAFT` publication.
- `PUBLICATION_NOT_COLLECTING` (409) - employee self-service submission write outside `COLLECTING`.
- `PUBLICATION_NOT_MUTABLE` (409) - assignment create/delete outside `{ASSIGNING, PUBLISHED, ACTIVE}`, or admin availability replacement outside `{COLLECTING, ASSIGNING}`.
- `PUBLICATION_NOT_ASSIGNING` (409) - auto-assign or publish outside `ASSIGNING`.
- `PUBLICATION_NOT_PUBLISHED` (409) - activate outside `PUBLISHED`, or shift-change write outside `PUBLISHED`.
- `PUBLICATION_NOT_ACTIVE` (409) - end outside `ACTIVE`, leave create outside `ACTIVE`, or roster fetched for a publication that is not viewable.
- `USER_DISABLED` (409) - admin tries to assign a disabled user, admin tries to replace availability for a disabled user, or shift-change apply observes a disabled user under `FOR UPDATE`.
- `ASSIGNMENT_USER_ALREADY_IN_SLOT` (409) - admin `CreateAssignment` for a `(publication, user, slot)` triple that already has an assignment row.
- `TEMPLATE_SLOT_OVERLAP` (409) - admin `CreateSlot` / `UpdateSlot` that would violate the GIST exclusion constraint.
- `SHIFT_CHANGE_NOT_PENDING` (409) - approve/reject/cancel on a terminal request.
- `SHIFT_CHANGE_EXPIRED` (409) - approve/reject/cancel on a request past `expires_at`.
- `SHIFT_CHANGE_INVALIDATED` (409) - approve surfaces that the captured baseline assignment row is gone or no longer belongs to the captured user.
- `INTERNAL_ERROR` (500) - anything else.

#### Scenario: Malformed body yields INVALID_REQUEST

- **WHEN** any scheduling endpoint receives a malformed body, path, or query
- **THEN** the response is HTTP 400 with error code `INVALID_REQUEST`

#### Scenario: Missing publication yields PUBLICATION_NOT_FOUND

- **WHEN** any scheduling endpoint is called with an `{id}` that does not match any publication row
- **THEN** the response is HTTP 404 with error code `PUBLICATION_NOT_FOUND`

#### Scenario: Bad occurrence date yields INVALID_OCCURRENCE_DATE

- **WHEN** any endpoint accepting `occurrence_date` receives a value that fails `IsValidOccurrence`
- **THEN** the response is HTTP 400 with error code `INVALID_OCCURRENCE_DATE`
