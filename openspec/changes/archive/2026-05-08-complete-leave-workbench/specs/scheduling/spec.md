## ADDED Requirements

### Requirement: Leave pool endpoint

`GET /leaves/pool` SHALL require `RequireAuth` and return a paginated leave pool for the current viewer. The endpoint SHALL accept `state ∈ {'pending','completed','cancelled','failed','all'}`, `page`, and `page_size`; omitted `state` defaults to `pending`, omitted `page` defaults to `1`, and omitted `page_size` defaults to `20`. Invalid state or pagination values SHALL be rejected with HTTP 400 and error code `INVALID_REQUEST`.

The response SHALL include `{ leaves, page, page_size, total_count }`. `total_count` SHALL count all rows matching the viewer's visibility and state filter before pagination.

Visibility SHALL be:

- `give_pool` leave rows are visible to every authenticated user.
- `give_direct` leave rows are visible only to the requester, the counterpart, and admins.
- Admins are visible to all leave rows but are view-only in the pool.

Sorting SHALL be:

- `pending`: occurrence start ascending.
- `completed`, `cancelled`, and `failed`: leave creation time descending.
- `all`: pending rows first by occurrence start ascending, then non-pending rows by leave creation time descending.

#### Scenario: Public leave is visible to another employee

- **GIVEN** Alice has a pending `give_pool` leave
- **WHEN** Bob calls `GET /leaves/pool?state=pending`
- **THEN** the response includes Alice's leave

#### Scenario: Direct leave is hidden from unrelated employees

- **GIVEN** Alice has a pending `give_direct` leave whose counterpart is Bob
- **WHEN** Carol calls `GET /leaves/pool?state=pending`
- **THEN** the response does not include Alice's leave

#### Scenario: Direct leave is visible to requester counterpart and admin

- **GIVEN** Alice has a pending `give_direct` leave whose counterpart is Bob
- **WHEN** Alice, Bob, or an admin calls `GET /leaves/pool?state=pending`
- **THEN** the response includes Alice's leave

#### Scenario: Pool response includes pagination metadata

- **WHEN** a viewer calls `GET /leaves/pool?state=all&page=2&page_size=20`
- **THEN** the response includes `page = 2`, `page_size = 20`, `total_count`, and the current page of `leaves`

#### Scenario: Pending pool sort prioritizes urgency

- **GIVEN** two visible pending leaves with occurrence starts on `2026-05-09T09:00:00Z` and `2026-05-08T09:00:00Z`
- **WHEN** the viewer calls `GET /leaves/pool?state=pending`
- **THEN** the `2026-05-08T09:00:00Z` row appears before the `2026-05-09T09:00:00Z` row

### Requirement: Leave pool row display and actions

Each leave pool row SHALL include human-readable scheduling context: leave id, derived leave state, request type, category, reason, occurrence date, occurrence start and end, slot start and end, position id and name, requester id and name, counterpart id and name when present, substitute id and name when the leave is completed, share URL, and created/updated timestamps.

Each row SHALL include urgency metadata for pending rows: `occurrence_start`, a duration or timestamp sufficient for the client to display time remaining, and a boolean equivalent to "starts within 24 hours" at the time of the response.

Each row SHALL include an action affordance object with `can_claim`, `can_approve`, `can_reject`, `can_cancel`, and `disabled_reason`. The service SHALL compute these flags from the viewer, row visibility, row state, request type, requester/counterpart identity, admin status, and position qualification. The write endpoints remain authoritative and SHALL re-check all constraints.

#### Scenario: Qualified employee can claim public leave

- **GIVEN** Alice has a pending `give_pool` leave for position `P`
- **AND** Bob is qualified for position `P`
- **WHEN** Bob calls `GET /leaves/pool?state=pending`
- **THEN** Alice's leave row has `actions.can_claim = true`

#### Scenario: Unqualified employee sees disabled public leave

- **GIVEN** Alice has a pending `give_pool` leave for position `P`
- **AND** Bob is not qualified for position `P`
- **WHEN** Bob calls `GET /leaves/pool?state=pending`
- **THEN** Alice's leave row is present
- **AND** `actions.can_claim = false`
- **AND** `actions.disabled_reason = "not_qualified"`

#### Scenario: Requester can cancel own pending leave

- **GIVEN** Alice has a pending leave
- **WHEN** Alice calls `GET /leaves/pool?state=pending`
- **THEN** the row has `actions.can_cancel = true`
- **AND** `actions.can_claim = false`

#### Scenario: Admin rows are view only

- **GIVEN** an admin views a pending leave pool row
- **WHEN** the row is returned
- **THEN** every action flag is false
- **AND** the row indicates view-only admin behavior through `disabled_reason`

#### Scenario: Completed leave identifies substitute

- **GIVEN** Alice's public leave was claimed by Bob
- **WHEN** any viewer who can see the row calls `GET /leaves/pool?state=completed`
- **THEN** the row identifies Bob as the substitute

### Requirement: Pending leave preserves original responsibility

A pending leave SHALL NOT by itself remove the requester's responsibility for the original assignment occurrence. The assignment override SHALL be written only when the underlying shift-change request is approved or claimed. If the request expires before approval or claim, the leave state SHALL derive as `failed` and the original assignment SHALL remain unchanged.

#### Scenario: Pending leave does not transfer the occurrence

- **GIVEN** Alice submits a public leave for assignment `A` on `2026-05-08`
- **AND** no one has claimed it
- **WHEN** the roster for that occurrence is read
- **THEN** Alice remains the assigned user for that occurrence

#### Scenario: Claimed leave transfers one occurrence

- **GIVEN** Alice submits a public leave for assignment `A` on `2026-05-08`
- **WHEN** Bob successfully claims it
- **THEN** the underlying shift-change request becomes `approved`
- **AND** an occurrence override assigns `A` on `2026-05-08` to Bob
- **AND** Alice's derived leave state is `completed`

#### Scenario: Unclaimed leave fails after expiry

- **GIVEN** Alice has a pending leave whose occurrence start has passed
- **WHEN** the leave is read through any leave endpoint
- **THEN** the underlying request is expired on read
- **AND** Alice's derived leave state is `failed`

## MODIFIED Requirements

### Requirement: Leave preview endpoint

`GET /users/me/leaves/preview?from=YYYY-MM-DD&to=YYYY-MM-DD` SHALL require `RequireAuth`. It SHALL return the viewer's future occurrences in the current ACTIVE publication that fall within `[from, to]`.

The response SHALL list each occurrence with `{ assignment_id, occurrence_date, slot: {id, weekday, start_time, end_time}, position: {id, name}, occurrence_start, occurrence_end, direct_candidates }`, sorted by `occurrence_start` ascending. `direct_candidates` SHALL contain active users other than the requester who are qualified for the occurrence's position, with `{ user_id, name }` for each candidate. Occurrences whose `occurrence_start <= NOW()` SHALL be excluded.

If no ACTIVE publication exists, the response SHALL be HTTP 200 with an empty `occurrences` array. If `from > to`, the response SHALL be HTTP 400 with error code `INVALID_REQUEST`.

#### Scenario: Future occurrences in the requested range

- **GIVEN** Alice has assignments in the current ACTIVE publication with multiple future occurrences in `[2026-05-01, 2026-05-31]`
- **WHEN** Alice calls `GET /users/me/leaves/preview?from=2026-05-01&to=2026-05-31`
- **THEN** the response includes those occurrences sorted by `occurrence_start`

#### Scenario: Direct candidates are qualified colleagues

- **GIVEN** Alice previews a future occurrence for position `P`
- **AND** Bob is active and qualified for `P`
- **AND** Carol is active but not qualified for `P`
- **WHEN** Alice calls the preview endpoint
- **THEN** Bob appears in the occurrence's `direct_candidates`
- **AND** Carol does not appear
- **AND** Alice does not appear

#### Scenario: Past occurrences are filtered out

- **GIVEN** an occurrence on `2026-04-26` whose `occurrence_start` is in the past at request time
- **WHEN** Alice calls preview with `from = 2026-04-01`
- **THEN** the response does NOT include the past occurrence

#### Scenario: No active publication returns empty list

- **GIVEN** no publication has stored or effective state `ACTIVE`
- **WHEN** Alice calls the preview endpoint
- **THEN** the response is HTTP 200 with `{ occurrences: [] }`

#### Scenario: Inverted range is rejected

- **WHEN** Alice calls preview with `from = 2026-05-10&to = 2026-05-01`
- **THEN** the response is HTTP 400 with error code `INVALID_REQUEST`

### Requirement: Leave detail and listing endpoints

`GET /leaves/{id}` SHALL require `RequireAuth` only — any logged-in user SHALL be permitted to read leave details. The response SHALL include the leave row and the underlying SCRT in a single payload, plus derived `state`, requester name, counterpart name when present, substitute name when the leave is completed, and scheduling context for the referenced occurrence. A missing leave SHALL be rejected with HTTP 404 and error code `LEAVE_NOT_FOUND`. The frontend SHALL use the leave response action affordances and the SCRT layer's existing authorization rules to decide which action buttons (approve, reject, cancel, claim) to render.

`GET /users/me/leaves` SHALL require `RequireAuth` and return the viewer's leaves, sorted by `created_at DESC`, paginated.

`GET /publications/{id}/leaves` SHALL require `RequireAdmin` and return all leaves in the named publication, sorted by `created_at DESC`, paginated.

#### Scenario: Any logged-in user can read a leave

- **GIVEN** Alice's leave `L`
- **WHEN** any authenticated employee or admin calls `GET /leaves/{L}`
- **THEN** the response is HTTP 200 with the leave detail
- **AND** the response includes Alice's display name as requester

#### Scenario: Completed leave detail identifies substitute

- **GIVEN** Alice's leave was claimed by Bob
- **WHEN** Alice calls `GET /leaves/{id}`
- **THEN** the response identifies Bob as the substitute

#### Scenario: Non-admin cannot list publication-wide leaves

- **WHEN** an employee calls `GET /publications/{id}/leaves`
- **THEN** the request is rejected by the `RequireAdmin` middleware

#### Scenario: Self listing returns own leaves only

- **WHEN** Alice calls `GET /users/me/leaves`
- **THEN** the response contains only leaves whose `user_id = Alice.id`

### Requirement: Shift-change emails render localized HTML and text bodies

Shift-change request-received and shift-change resolution emails SHALL be rendered from embedded external template files into both a text/plain body and an HTML body. Each shift-change email subject SHALL be prefixed with `[<product_name>]`, where `<product_name>` is the current normalized branding product name. For regular shift-change requests where `leave_id IS NULL`, the email body SHALL contain a CTA to view requests and the complete requests URL in both HTML and text bodies. For leave-bearing requests where `leave_id IS NOT NULL`, the email body SHALL use leave-specific copy and contain a CTA to view the leave detail URL `/leaves/{leave_id}` in both HTML and text bodies. Shift-change email layout headers and footers SHALL use the configured product name instead of hard-coded `Rota`. When `organization_name` is configured, shift-change email footers SHALL include it; main shift-change body copy SHALL NOT add organization-specific scheduling facts outside the footer.

Shift-change email language SHALL resolve in this order for user-request-triggered emails: recipient user's `language_preference`, triggering request `Accept-Language`, then `en`. Shift-change emails triggered by system cascade work without a current request, such as assignment deletion invalidation, SHALL resolve in this order: recipient user's `language_preference`, then `en`.

#### Scenario: Direct request email contains localized CTA and fallback URL

- **GIVEN** branding has `product_name = "排班系统"`
- **WHEN** an employee creates a regular `swap` or `give_direct` request where `leave_id IS NULL`
- **THEN** an email is enqueued to the counterpart with `kind = 'shift_change_request_received'`
- **AND** the subject starts with `[排班系统]`
- **AND** the text body contains the requests URL
- **AND** the HTML body contains a CTA to view the request and the complete requests URL

#### Scenario: Direct leave email links to leave detail

- **GIVEN** branding has `product_name = "排班系统"`
- **WHEN** an employee creates a `give_direct` leave whose underlying request has `leave_id = L`
- **THEN** an email is enqueued to the counterpart with `kind = 'shift_change_request_received'`
- **AND** the subject starts with `[排班系统]`
- **AND** the text and HTML bodies contain `/leaves/L`
- **AND** the body copy describes a leave coverage request

#### Scenario: Pool creation still sends no email

- **WHEN** an employee creates a regular `give_pool` request or a public `give_pool` leave
- **THEN** no email is sent at creation

#### Scenario: Resolution email contains localized outcome and branded footer

- **GIVEN** branding has `product_name = "Scheduling Portal"` and `organization_name = "Operations"`
- **WHEN** a regular shift-change request is approved, rejected, claimed, cancelled, or invalidated
- **THEN** an email is enqueued to the requester with `kind = 'shift_change_resolved'`
- **AND** the subject starts with `[Scheduling Portal]`
- **AND** the subject and body contain the localized outcome label
- **AND** the text and HTML bodies contain the requests URL
- **AND** the footer identifies `Operations`

#### Scenario: Leave resolution email links to leave detail

- **GIVEN** Alice has a leave with id `L`
- **WHEN** the leave's underlying shift-change request is approved, rejected, claimed, cancelled, or invalidated
- **THEN** an email is enqueued to Alice with `kind = 'shift_change_resolved'`
- **AND** the text and HTML bodies contain `/leaves/L`
- **AND** the body copy describes the leave coverage outcome

#### Scenario: Persisted recipient language beats request language

- **GIVEN** a shift-change email recipient has `language_preference = 'en'`
- **AND** the triggering request has `Accept-Language: zh-CN,zh;q=0.9`
- **WHEN** the shift-change email is enqueued
- **THEN** the rendered email language is `en`

#### Scenario: Request language is used when recipient has no preference

- **GIVEN** a shift-change email recipient has `language_preference IS NULL`
- **AND** the triggering request has `Accept-Language: zh-CN,zh;q=0.9`
- **WHEN** the shift-change email is enqueued
- **THEN** the rendered email language is `zh`

#### Scenario: System cascade falls back to English

- **GIVEN** an assignment deletion invalidates a pending shift-change request
- **AND** the requester has `language_preference IS NULL`
- **WHEN** the invalidation email is enqueued
- **THEN** the rendered email language is `en`
