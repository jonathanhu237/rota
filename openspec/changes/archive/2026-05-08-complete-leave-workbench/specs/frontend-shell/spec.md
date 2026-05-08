## ADDED Requirements

### Requirement: Leave workbench pool table

The `/leaves` route SHALL render a leave workbench. The top of the page SHALL include a primary "Request leave" CTA linking to `/leaves/new`. The main content SHALL render a paginated leave pool table backed by `GET /leaves/pool`.

The table SHALL support status filters: `pending`, `all`, `completed`, `cancelled`, and `failed`. The default filter SHALL be `pending`. Changing the filter SHALL reset pagination to page `1`. The table SHALL use page size `20` and provide simple previous/next pagination using the backend's `total_count`.

Each row SHALL display at least: requester name, occurrence date, time range, position name, category, request type, derived state, urgency for pending rows, counterpart or substitute when applicable, and available actions. Pending rows that start within 24 hours SHALL be visually emphasized without hiding any row data.

#### Scenario: Leaves route renders workbench CTA and pending pool

- **GIVEN** an authenticated user
- **WHEN** the user navigates to `/leaves`
- **THEN** the page renders a "Request leave" CTA linking to `/leaves/new`
- **AND** the page requests the leave pool with `state = pending`, `page = 1`, and `page_size = 20`
- **AND** the leave pool table is rendered

#### Scenario: Status filter resets pagination

- **GIVEN** the user is on page `3` of the pending leave pool
- **WHEN** the user switches the filter to `completed`
- **THEN** the frontend requests page `1` of the completed leave pool

#### Scenario: Urgent pending leave is highlighted

- **GIVEN** the leave pool returns a pending row marked as starting within 24 hours
- **WHEN** the table renders
- **THEN** the row displays its urgency state visibly

### Requirement: Leave workbench row actions

The leave workbench table SHALL render actions from each row's backend-provided action affordance object. The frontend SHALL NOT infer claim/approve/reject/cancel permission solely from raw IDs or request type.

The table SHALL map actions as:

- `can_claim` → "Help cover" / "帮他上".
- `can_approve` → "Approve" / "同意".
- `can_reject` → "Reject" / "拒绝".
- `can_cancel` → "Cancel leave" / "取消请假".
- Rows without a permitted action SHALL still offer a detail link.

Disabled reasons such as `not_qualified` and `admin_view_only` SHALL be rendered as non-blocking explanatory text or disabled-action copy.

#### Scenario: Public leave claim action

- **GIVEN** the pool returns a row with `actions.can_claim = true`
- **WHEN** the row renders
- **THEN** the row includes a claim action
- **WHEN** the user clicks the action
- **THEN** the frontend calls the existing approve/claim shift-change endpoint for the underlying request

#### Scenario: Direct leave approval actions

- **GIVEN** the pool returns a row with `actions.can_approve = true` and `actions.can_reject = true`
- **WHEN** the row renders
- **THEN** the row includes approve and reject actions

#### Scenario: Own pending leave cancel action

- **GIVEN** the pool returns a row with `actions.can_cancel = true`
- **WHEN** the row renders
- **THEN** the row includes a cancel-leave action

#### Scenario: View-only row still links to detail

- **GIVEN** the pool returns a row with no action flags set
- **WHEN** the row renders
- **THEN** the row still includes a link to `/leaves/:leaveId`

### Requirement: Leave request direct-candidate picker

The `/leaves/new` request flow SHALL use each preview occurrence's direct candidate list for the "specified colleague" picker. The picker SHALL NOT include the requester and SHALL NOT include users who are not qualified for the occurrence's position. Each occurrence remains independently submitted; no batch-submit transaction is introduced.

#### Scenario: Direct picker uses occurrence candidates

- **GIVEN** the leave preview response for an occurrence includes Bob but not Carol in `direct_candidates`
- **WHEN** the user switches that occurrence to specified-colleague leave
- **THEN** Bob is available in the picker
- **AND** Carol is not available in the picker

#### Scenario: Occurrence submissions remain independent

- **GIVEN** the preview returns two future occurrences
- **WHEN** the user submits leave for one occurrence
- **THEN** the frontend sends one `POST /leaves` request for that occurrence
- **AND** the other occurrence is not submitted automatically

### Requirement: Leave detail uses display names and leave language

The `/leaves/:leaveId` detail route SHALL display requester, counterpart, and substitute names when provided by the backend. It SHALL describe actions and outcomes as leave coverage, not generic shift-change operations. Email links that target a specific leave SHALL land on this route.

#### Scenario: Detail shows names instead of raw IDs

- **GIVEN** the leave detail API returns requester name Alice and substitute name Bob
- **WHEN** the detail page renders
- **THEN** the page displays Alice and Bob by name
- **AND** it does not rely on `#<id>` as the primary user label

#### Scenario: Leave email detail link opens leave detail

- **GIVEN** an email links to `/leaves/42`
- **WHEN** the authenticated recipient opens the link
- **THEN** the leave detail route renders leave `42`

## MODIFIED Requirements

### Requirement: Leave route namespace

All leave-related authenticated UI routes SHALL live under the `/leaves` URL prefix:

- `/leaves` — the leave workbench, including the leave pool table, status filters, pagination, row actions, and a primary "Request leave" CTA linking to `/leaves/new`.
- `/leaves/new` — the request flow with multi-row drafts and slot previews.
- `/leaves/:leaveId` — leave detail.

The legacy paths `/leave` (request) and `/my-leaves` (history) SHALL NOT be served. Internal links and tests in the codebase SHALL reference only the `/leaves` prefix.

#### Scenario: Leave workbench exposes Request CTA

- **GIVEN** an authenticated user
- **WHEN** the user navigates to `/leaves`
- **THEN** the page renders a "Request leave" CTA at the top
- **AND** the CTA links to `/leaves/new`
- **AND** the page renders the leave workbench rather than a personal-history-only page

#### Scenario: Legacy leave paths are not served

- **WHEN** any internal link, sidebar item, or test in the frontend references `/leave` or `/my-leaves`
- **THEN** the reference is updated to `/leaves` or `/leaves/new` as appropriate
- **AND** no production route definitions register `/leave` or `/my-leaves`

#### Scenario: Leave detail remains a child of /leaves

- **GIVEN** a leave with id `K`
- **WHEN** the user clicks a row in the `/leaves` workbench table
- **THEN** the user is navigated to `/leaves/K`
- **AND** the URL pattern `/leaves/:leaveId` is the only path that renders leave detail
