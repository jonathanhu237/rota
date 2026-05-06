## Context

Availability is currently an employee self-service flow: employees can list qualified cells, list their own submitted cells, and tick or untick one cell at a time during `COLLECTING`. Admins can see submitted availability indirectly on the assignment board, but they cannot inspect a paginated employee list or correct a specific employee's submissions without direct SQL.

The data model already supports the feature: `availability_submissions` stores `(publication_id, user_id, slot_id, weekday)` with no position column. The position is resolved later by auto-assign from current user qualifications and slot composition. No schema change is required.

The publication route also has a UI structural problem: the parent publication detail route renders its detail card and then renders child routes below it through `<Outlet />`. Assignment and shift-change management should be standalone publication subpages, and availability management should follow that structure.

## Goals / Non-Goals

**Goals:**

- Let admins find a publication-relevant employee, inspect their submitted availability, and replace that employee's submitted set in one save.
- Keep the replacement atomic: either every add/delete for that employee commits, or none of it does.
- Enforce current qualifications and template slot weekdays for the final saved target set.
- Make reads available in every publication state, but allow writes only while availability is still operationally mutable: `COLLECTING` and `ASSIGNING`.
- Keep availability edits isolated from assignments. Existing assignments remain as-is.
- Make publication detail, assignment board, availability management, and shift-change management standalone child pages.

**Non-Goals:**

- No assignment draft atomicity or published-roster history changes.
- No bulk editing of multiple employees.
- No availability export, heatmap, coverage analytics, or per-slot reverse lookup.
- No schema migration.
- No new external dependency.

## Decisions

### 1. Add dedicated admin availability APIs

The backend will add three admin-only endpoints:

- `GET /publications/{id}/availability-board?page=1&page_size=10&search=...`
- `GET /publications/{id}/availability-submissions/{user_id}`
- `PUT /publications/{id}/availability-submissions/{user_id}`

The board endpoint returns publication summary data, normalized pagination metadata, and active non-bootstrap employees who have at least one current qualification overlapping any position used by the publication template. It includes employees with zero submitted cells. `page_size` uses the existing list convention: default `10`, maximum `100`.

The detail endpoint returns the publication state, target employee, template slot grid, the employee's positions, submitted cells, and eligibility per `(slot_id, weekday)` cell. That lets the frontend render disabled cells without reimplementing qualification rules.

The `PUT` body is:

```json
{
  "submissions": [
    { "slot_id": 1, "weekday": 1 }
  ]
}
```

The service normalizes the target list as a set, validates it, diffs it against existing rows, and applies only the added and removed cells in a single transaction. An empty target list is valid and clears that employee's availability for the publication.

Rejected alternatives:

- Reuse the assignment-board payload. It already contains submitted slots, but it is not paginated and is shaped for drag-drop assignment work rather than targeted employee edits.
- Expose single-cell admin tick/untick endpoints. That would make the frontend responsible for sequencing changes and would not give a clean transaction boundary for a save.

### 2. Validate the final target set, not each UI click

The frontend may let admins draft changes locally, including removing previously submitted cells that are no longer eligible after a qualification change. The server validates only the final target set submitted to `PUT`.

Each final cell must:

- belong to a slot in the publication template;
- use a weekday claimed by that slot;
- overlap the target employee's current positions with the slot composition;
- target an active, non-bootstrap, publication-relevant employee.

If any final cell fails qualification, the whole request fails with `NOT_QUALIFIED` and the old rows remain unchanged. Previously persisted but now-ineligible cells can be removed because they are absent from the final target set.

Rejected alternative: reject loading or saving any employee who has legacy ineligible submissions. That would trap admins with no UI path to clean up those rows.

### 3. State gates are intentionally wider for admins than employees

Employee self-service availability writes remain `COLLECTING` only. Admin replacement writes are allowed in `COLLECTING` and `ASSIGNING`, because corrections are needed during scheduling after the public submission window has closed. Reads are allowed in every state so admins can inspect historical publications and diagnose schedule inputs.

Writes in `DRAFT`, `PUBLISHED`, `ACTIVE`, or `ENDED` return `PUBLICATION_NOT_MUTABLE` at HTTP 409. Read-only frontend states still show data but disable save controls.

Error codes used by the new endpoints:

- `INVALID_REQUEST` (400): malformed path, query, or body; invalid weekday; target cell not part of the publication template.
- `PUBLICATION_NOT_FOUND` (404): publication id does not exist.
- `USER_NOT_FOUND` (404): target user does not exist, is the bootstrap admin, or is not relevant to the publication template.
- `NOT_QUALIFIED` (403): at least one submitted target cell does not overlap the target user's current positions.
- `PUBLICATION_NOT_MUTABLE` (409): admin attempts replacement outside `COLLECTING` or `ASSIGNING`.
- `USER_DISABLED` (409): target user exists but is disabled.
- `FORBIDDEN` (403): authenticated non-admin calls an admin availability endpoint.
- `UNAUTHORIZED` (401): unauthenticated caller.

No new state transition is introduced. The existing state machine remains:

```text
DRAFT -> COLLECTING -> ASSIGNING -> PUBLISHED -> ACTIVE -> ENDED
```

### 4. Audit emits one event per changed cell

Admin replacements can touch multiple rows, so the audit taxonomy will include `availability.admin.create` and `availability.admin.delete`. The service records one event for each added or removed cell after the database transaction succeeds. Each event includes at least `publication_id`, `user_id`, `slot_id`, and `weekday`.

Rejected alternative: emit a single aggregate replacement event. That would obscure the exact cells changed and force future auditors to parse before/after arrays.

### 5. Frontend routes become standalone publication subpages

The current publication parent route will become a thin layout route. The existing publication detail card moves to an index route, and child pages render independently:

- `/publications/:publicationId`
- `/publications/:publicationId/assignments`
- `/publications/:publicationId/availability`
- `/publications/:publicationId/availability/:userId`
- `/publications/:publicationId/shift-changes`

The publication detail index and assignment board will link to availability management. The availability table supports search, pagination, and a single action to open a user's editor. The editor uses a local draft, a bottom save bar, discard/save actions, and a navigation/refresh prompt while dirty.

Rejected alternative: keep rendering availability under the publication detail card. That repeats the assignment-board nesting problem and makes the management page visually cramped.

## Risks / Trade-offs

- [Risk] Admin saves could race with an employee submission during `COLLECTING`. -> Mitigation: the replacement transaction reads the latest rows and commits a single final set for that employee; last writer wins at the per-employee set level.
- [Risk] Qualification changes can leave existing ineligible submissions. -> Mitigation: detail responses surface those cells as removable exceptions, and replacement validation only rejects ineligible cells that remain in the final target set.
- [Risk] Availability edits during `ASSIGNING` may make the current assignment board look inconsistent. -> Mitigation: assignments are not mutated automatically; the assignment board already treats missing availability as a warning and admins can rerun auto-assign or edit assignments manually.
- [Risk] Per-cell audit events can produce many rows for a large replacement. -> Mitigation: edits are scoped to one employee at a time and the expected row count is small for this project.

## Migration Plan

No database migration is required. Deployment is a normal backend and frontend rollout.

Rollback is also normal application rollback. Any availability rows already changed through the feature remain ordinary `availability_submissions` rows and require no special cleanup.

## Open Questions

None for the first implementation.
