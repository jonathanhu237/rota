## Why

The current leave feature can create leave-backed shift-change requests, but the product experience is still fragmented: employees do not have a clear leave pool where they can see requests that need coverage, decide whether they can help, or track whether their own leave has actually been covered. For a department-scale schedule, leave needs to read as "this shift still needs a qualified substitute" rather than as a generic shift-change row.

## What Changes

- Turn `/leaves` into a leave workbench instead of a simple personal history page.
  - The page SHALL keep a primary "Request leave" CTA to `/leaves/new`.
  - The main surface SHALL be a paginated leave pool table with status filters.
  - The default view SHALL show pending leave requests first.
- Define leave pool visibility around the two accepted leave modes:
  - Public coverage (`give_pool`): visible to all logged-in users.
  - Direct coverage (`give_direct`): visible only to the requester, the named counterpart, and admins.
  - Admins may view all leaves but SHALL NOT act on behalf of employees in this change.
- Add a dedicated leave pool API, separate from "my leave history", that returns visible leave rows with display names, shift context, urgency information, pagination metadata, and row-level action affordances.
- Keep `/leaves/new` as the request flow:
  - Employees choose a date range.
  - The app lists future assigned occurrences in that range.
  - Each occurrence is submitted independently as either public coverage or direct coverage.
  - Direct coverage candidates are limited to qualified colleagues other than the requester.
- Preserve leave categories (`sick`, `personal`, `bereavement`) and optional reason text for display and records only; category does not affect eligibility or approval behavior.
- Keep `/leaves/:leaveId` as the detail page and make leave-related emails link to the detail page when there is a specific leave.
- Improve leave notifications:
  - Direct coverage creation emails the named counterpart.
  - Direct approval/rejection emails the requester.
  - Public coverage creation does not email every possible helper.
  - Public coverage claim emails the requester.
- Clarify responsibility semantics:
  - While a leave is pending, the requester remains responsible for the original shift.
  - Only approval/claim transfers that single occurrence to the substitute.
  - If no one covers the leave before the occurrence starts, the leave fails.

## Capabilities

### New Capabilities

- None. Leave remains part of the scheduling and authenticated frontend-shell capabilities.

### Modified Capabilities

- `scheduling`: Adds a leave pool read model/API, visibility rules, display-name enriched leave responses, row action affordances, leave-specific notification requirements, and direct-coverage candidate constraints.
- `frontend-shell`: Changes `/leaves` from personal history into a leave workbench with a pool table, status filters, urgency display, actions, pagination, and links to the existing request/detail routes.

## Non-goals

- No HR-style leave approval, leave balances, quota accounting, payroll/attendance computation, or manager approval workflow.
- No admin intervention actions such as assigning a substitute, approving on behalf of a counterpart, rejecting on behalf of a counterpart, or force-cancelling an employee's leave.
- No broadcast email for public leave requests.
- No full notification center with read/unread message rows, archived notifications, push, SMS, or WebSocket delivery.
- No batch submit transaction for multiple leave occurrences.
- No category management UI or database-backed custom leave categories.

## Impact

- Backend API: add a leave pool listing endpoint and expand leave response data with human-readable names, shift context, pagination metadata, urgency/action fields, and explicit visibility/action rules.
- Backend service/repository: add a leave pool read model that joins leaves, shift-change requests, assignments, slots, positions, users, qualifications, and decision metadata without changing the existing leave/SCRT state machine.
- Backend email: adjust leave-bearing shift-change email rendering/link targets so leave users see leave language and land on leave detail when appropriate.
- Frontend: rebuild `/leaves` as a workbench table, update `/leaves/new` direct-candidate filtering, improve `/leaves/:leaveId` display names and action presentation, and keep navigation/breadcrumbs consistent.
- Tests: add backend service/handler/repository coverage for pool visibility/action rules and frontend coverage for table filtering, actions, urgency display, candidate filtering, and detail rendering.
