## Context

The first leave implementation already models leave as a leave-backed shift-change request: `give_pool` means public coverage, `give_direct` means a named counterpart must approve, and successful approval/claim writes the occurrence override through the existing shift-change apply path. The unfinished part is the product surface around that model. `/leaves` is currently a personal history page, `/requests` still speaks in generic shift-change language, and leave detail/list responses expose user IDs rather than names.

This change keeps the core data model intact and adds a dedicated leave workbench/read model. The workbench is optimized for one user question: "which leave requests need coverage, and what can I do about them?"

## Goals / Non-Goals

**Goals:**

- Make `/leaves` the main leave workbench with request CTA, leave pool table, status filters, pagination, urgency, and row actions.
- Add a backend leave-pool API that enforces visibility centrally and returns enough presentation data for the frontend to avoid guessing permissions from raw SCRT fields.
- Preserve the existing leave/SCRT lifecycle and approval paths.
- Make direct leave creation and leave resolution emails read as leave workflows and link to leave detail.
- Replace ID-only leave display with names for requester, counterpart, and successful substitute where available.

**Non-Goals:**

- No HR approval workflow, quotas, payroll/attendance logic, or new leave state machine.
- No admin intervention actions.
- No public-pool broadcast email.
- No notification center or real-time delivery.
- No database schema migration.
- No new frontend or backend dependency.

## Decisions

### D-1. Add a dedicated leave pool endpoint

**Decision:** add `GET /leaves/pool?state=pending|completed|cancelled|failed|all&page=1&page_size=20`, authenticated for every logged-in user. The response returns:

- `leaves`: visible leave pool rows.
- `page`, `page_size`, `total_count`.

Each row includes leave identity, derived state, category/reason, request type, occurrence date, occurrence start/end, slot time, position name, requester name, counterpart name for direct requests, substitute name for completed requests, urgency metadata, `share_url`, and an `actions` object.

The existing endpoints remain:

- `GET /users/me/leaves` for personal history and dashboard cards.
- `GET /publications/{id}/leaves` for admin publication-level API compatibility.
- `GET /leaves/{id}` for detail.

**Alternative rejected:** overload `GET /users/me/leaves`. The leave pool includes rows not owned by the viewer, including public requests and direct requests assigned to the viewer; calling that "my leaves" would make permission and sorting rules ambiguous.

### D-2. Visibility is service-owned, not frontend-owned

**Decision:** the pool API enforces visibility:

- Public coverage (`give_pool`) is visible to every authenticated user.
- Direct coverage (`give_direct`) is visible to the requester, the counterpart, and admins.
- Admins see all leave rows but receive view-only actions.

The frontend does not filter hidden direct leaves because it never receives them.

**Alternative rejected:** reuse the shift-change request list. It already includes pool rows for non-admins, but it is scoped to publication and shift-change semantics, not leave workbench semantics. It also does not return leave-specific display names, urgency, or action affordances.

### D-3. Row actions are returned by the backend

**Decision:** each pool/detail row returns action flags:

- `can_claim`
- `can_approve`
- `can_reject`
- `can_cancel`
- `disabled_reason`

Rules:

- Pending public leave owned by another user: `can_claim` only when the viewer is qualified for the position and is not an admin.
- Pending direct leave assigned to the viewer: `can_approve` and `can_reject`.
- Pending leave owned by the viewer: `can_cancel`.
- Admins: all action flags false, `disabled_reason = "admin_view_only"` where useful.
- Unqualified public leaves remain visible with `disabled_reason = "not_qualified"`.

The API does not precompute time conflicts. Shift-change approval/claim remains the final authority and may still reject with `SHIFT_CHANGE_TIME_CONFLICT`.

**Alternative rejected:** infer actions entirely in React from raw request fields. That would duplicate qualification/admin/self rules in the frontend and would still be incomplete without backend-only checks.

### D-4. Keep the existing state model

**Decision:** no new leave states are added. The workbench filters use existing derived leave states:

```text
SCRT pending      -> leave pending
SCRT approved     -> leave completed
SCRT expired      -> leave failed
SCRT rejected     -> leave failed
SCRT cancelled    -> leave cancelled
SCRT invalidated  -> leave cancelled
```

Responsibility rule:

```text
Requester owns original assignment
        |
        | submit leave
        v
Pending leave, requester still responsible
        |
        | counterpart approves OR pool helper claims
        v
Completed leave, single occurrence overridden to substitute

Pending leave, no coverage before occurrence start
        |
        v
Failed leave, original assignment was never transferred
```

**Alternative rejected:** add a separate leave-workbench state such as `needs_coverage`. It would duplicate `pending` without changing behavior.

### D-5. Sort for action first

**Decision:** pool sorting is server-side:

- `pending`: occurrence start ascending.
- `completed`, `cancelled`, `failed`: leave creation time descending.
- `all`: pending rows first by occurrence start ascending, then non-pending rows by creation time descending.

The frontend resets to page 1 when the filter changes.

**Alternative rejected:** pure created-at sorting. It hides urgent upcoming uncovered shifts behind recently submitted low-urgency leaves.

### D-6. Enrich leave responses with names and shift context

**Decision:** leave pool and leave detail should stop displaying raw user IDs. The backend read model joins enough data to return requester, counterpart, and substitute names. For completed direct or public coverage, the substitute is the approving/claiming user. For cancelled leaves, `decided_by_user_id` is not labeled as substitute.

The read model also joins assignment, slot, and position data for the requester assignment so the UI can display date, time, position, and urgency without fetching the roster.

**Alternative rejected:** let the frontend stitch names from `/publications/{id}/members`. That endpoint only returns assigned publication members, not every relevant qualified user, and it forces every leave surface to repeat lookup logic.

### D-7. Add direct candidates to leave preview

**Decision:** extend `GET /users/me/leaves/preview` so each occurrence can include direct coverage candidates: active users other than the requester who are qualified for the occurrence's position. The frontend uses this list for the "specified colleague" dropdown.

The existing preview behavior remains: date range in, future assigned occurrences out. This change only adds qualified candidates to the occurrence payload.

**Alternative rejected:** keep using `/publications/{id}/members` for the dropdown. That list is assigned-members-only and not position-qualified for the selected occurrence.

### D-8. Leave-specific email copy and links

**Decision:** when a shift-change request has a `leave_id`, email generation uses leave language and links to `/leaves/{leave_id}`:

- Direct leave creation emails the counterpart.
- Direct approval/rejection emails the requester.
- Public leave creation sends no email.
- Public leave claim emails the requester.
- Cancellation/invalidation behavior continues to follow existing resolution email behavior, but links to leave detail when `leave_id` exists.

The outbox remains the delivery mechanism; no new notification storage is introduced.

**Alternative rejected:** create a separate notification subsystem. That would be larger than the leave workbench and is not needed for the agreed first version.

### D-9. Routing and pagination details

**Decision:** register `GET /leaves/pool` alongside existing leave routes. Go's `http.ServeMux` route specificity keeps the literal `/leaves/pool` distinct from `/leaves/{id}`. The pool handler uses leave-specific pagination defaults: page 1, page size 20, maximum 100.

**Alternative rejected:** pathing the endpoint under `/users/me/leaves/pool`. Admin visibility and public pool semantics are not "me" semantics.

### D-10. Error codes

No new error codes are required. The implementation reuses:

| Code | HTTP | Trigger |
|---|---:|---|
| `INVALID_REQUEST` | 400 | bad `state`, invalid pagination, malformed path/query/body |
| `INVALID_OCCURRENCE_DATE` | 400 | stale or invalid occurrence on create/approve paths |
| `SHIFT_CHANGE_INVALID_TYPE` | 400 | invalid leave type, including `swap` |
| `SHIFT_CHANGE_SELF` | 400 | user tries to target or claim self |
| `LEAVE_NOT_FOUND` | 404 | missing or hidden leave detail where applicable |
| `SHIFT_CHANGE_NOT_FOUND` | 404 | missing underlying request |
| `NOT_QUALIFIED` | 403 | requester or target lacks relevant position qualification |
| `SHIFT_CHANGE_NOT_QUALIFIED` | 403 | claim/approve qualification failure from SCRT path |
| `LEAVE_NOT_OWNER` | 403 | non-owner attempts leave cancel |
| `SHIFT_CHANGE_NOT_OWNER` | 403 | unauthorized approve/reject/cancel on underlying request |
| `PUBLICATION_NOT_ACTIVE` | 409 | leave create outside ACTIVE |
| `SHIFT_CHANGE_NOT_PENDING` | 409 | terminal request action |
| `SHIFT_CHANGE_EXPIRED` | 409 | action after occurrence start |
| `SHIFT_CHANGE_INVALIDATED` | 409 | referenced assignment changed |
| `SHIFT_CHANGE_TIME_CONFLICT` | 409 | substitute has conflicting assignment |
| `USER_DISABLED` | 409 | disabled counterpart or substitute observed by write path |

## Risks / Trade-offs

- **Risk:** The pool query joins several scheduling tables and can get expensive over long history. **Mitigation:** keep it paginated, index around existing leave/SCRT columns, and compute action flags only for returned rows.
- **Risk:** Backend action flags may drift from shift-change write validation. **Mitigation:** keep flags conservative and still rely on approve/reject/cancel endpoints for final validation.
- **Risk:** Leave-specific email copy could duplicate shift-change templates. **Mitigation:** reuse the existing email rendering pipeline and branch on `leave_id` for copy/link data.
- **Risk:** Extending preview with candidates increases payload size. **Mitigation:** preview date ranges are user-selected and small in normal use; candidates are compact `{user_id, name}` rows.

## Migration Plan

No database migration is required.

1. Add backend read models and handler methods while preserving all existing leave endpoints.
2. Extend preview responses and leave/detail responses in a backward-compatible way by adding fields.
3. Update frontend queries/types to consume the new fields.
4. Replace `/leaves` content with the workbench table while preserving the `/leaves/new` CTA and `/leaves/:leaveId` links.
5. Update email rendering for leave-bearing shift-change requests.

Rollback is code-only: revert the new endpoint/UI/email changes. Existing leave rows and shift-change requests remain valid because their schema and lifecycle do not change.

## Open Questions

None blocking. The user explicitly accepted the product rules around public visibility, direct visibility, admin view-only behavior, pending responsibility, notifications, categories, sorting, pagination, and keeping details out of this planning discussion unless needed for implementation.
