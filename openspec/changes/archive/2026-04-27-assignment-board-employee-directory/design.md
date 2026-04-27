## Context

The grid + side-panel layout shipped a day ago solved scanning ("which cells are short") but lost in-place visibility of "*who* is in this cell" — admins had to click into the side panel to see the people. The side panel was also asymmetric: it showed candidates *for the selected cell only*, forcing the same select-then-edit modal loop the redesign was supposed to escape.

This change replaces the cell-as-badge + selection-driven editor with **cell-as-seat-stack** + **right-panel-as-global-directory**. Drag becomes the primary interaction; selection goes away. Drag color feedback (green = qualified, yellow = unqualified-warning) replaces the candidate vs non-candidate distinction in the UI. The `GET /publications/{id}/assignment-board` API is trimmed to match this UI: per-cell `candidates` and `non_candidate_qualified` arrays are removed, and a top-level `employees[]` array provides the directory + each user's `user_positions`.

## Goals / Non-Goals

**Goals:**

- Each on-schedule cell renders one seat block per `(position, headcount-index)` unit. Each seat is filled (with an employee chip) or empty (with a placeholder for that position).
- Right panel is a global, always-visible directory of all active employees in this publication. Search by name; sort by hours-asc (default) or name.
- Drag any employee chip (right panel or in-grid filled seat) → all seats across the grid recolor: green for qualified, yellow for unqualified-warning. Off-schedule cells render no seats.
- Drop on empty seat: stage assign. Drop on filled seat by another user: stage replace. Drop on filled seat by same user: no-op.
- Click `×` on a seat chip: stage unassign. Same draft + warning + confirm-dialog flow as today.
- Backend assignment-board response shape trimmed to expose top-level `employees[]` and stop returning per-pair candidate lists.

**Non-Goals:**

- Auto-assigner behavior; availability-submission collection.
- Per-occurrence overrides.
- Multi-cell or batch ops.
- Mobile / narrow-viewport (< 1280px).
- Re-introducing per-cell candidate lists anywhere in the UI.
- Cell-to-cell drag (cells now contain seats; the drag becomes seat-to-seat).
- Selection state.

## Decisions

### D-1. Cell layout — seat stack

Each on-schedule cell renders:

```
┌─ 09:00–10:00 ─────────────┐
│  [partial · 已分配 1/3]   │   ← cell summary header (color status)
│                           │
│  前台负责人               │   ← seat 1 (lead)
│  [员工 34 (4.7h)] [×]     │      filled chip + unassign affordance
│                           │
│  前台助理                 │   ← seat 2 (assistant, headcount index 1)
│  空缺                     │      empty placeholder
│                           │
│  前台助理                 │   ← seat 3 (assistant, headcount index 2)
│  [员工 38 (4.7h)] [×]     │
└───────────────────────────┘
```

Each seat is identified by `{ slotID, weekday, positionID, headcountIndex }`. The `headcountIndex` distinguishes the 1st `前台助理` seat from the 2nd, both of which carry `positionID = 前台助理`. This index is purely a UI-side stable key; it's not part of the API or the database.

How the frontend assigns employees to specific seat indexes:

```ts
// Given a position-cell with required_headcount = N and assignments = [a, b, ...]:
// Sort assignments by assignment.id ASC for stability.
// Render N seats in order:
//   seat[0] = filled with assignments[0] if exists, else empty
//   seat[1] = filled with assignments[1] if exists, else empty
//   ...
//   seat[N-1] = filled with assignments[N-1] if exists, else empty
// If assignments.length > N (over-assigned, possible after admin draft adds):
//   render the extras as overflow seats in a separate "超额" row, each with × to unassign.
```

The over-assignment edge case happens transiently when an admin stages multiple `assign` drafts on a position. The existing draft state model already tolerates this (`projectedAssignments[].assignments.length` can exceed `required_headcount`). Render the overflow visibly so the admin sees the imbalance.

**Rejected — render seats horizontally inside a cell:** with cells already in a 7-column weekday grid, horizontal seats inside each cell would explode the page width. Vertical stacking keeps total width at the existing 7 columns.

**Rejected — show seats as small badges only (no employee name in cell):** loses the information density that motivated this redesign. The whole point is "see who is in each cell at a glance."

### D-2. Seat as drop target

Each seat uses `useDroppable` with id `seat:${slotID}:${weekday}:${positionID}:${headcountIndex}`. The drop data carries the slot/weekday/position triple. Drop semantics:

| Source kind | Target seat state | Result |
|---|---|---|
| Right-panel employee | empty | stage `assign` |
| Right-panel employee | filled with same user | no-op |
| Right-panel employee | filled with different user | stage `unassign` of existing + `assign` of dragged user |
| Filled-seat chip | empty (different cell) | stage `unassign` of source + `assign` to target |
| Filled-seat chip | empty (same cell, different position) | same as above (cross-position move within cell) |
| Filled-seat chip | filled with different user | stage replace at target + `unassign` of source seat |
| Filled-seat chip | filled with same user (i.e., source seat) | no-op |

Off-schedule cells render no seats; they cannot be drop targets at all. Cell-level click selection is removed.

**Rejected — keep cell-level drop targets:** would force the admin to "drop on cell, then pick a position from a popup." Two clicks for what should be one drag. Per-seat drop is the whole point of seat-stack rendering.

### D-3. Right-panel global directory

```
┌─ Employee directory ──────────┐
│  仍缺 3 个 cell                │   ← gap banner
├───────────────────────────────┤
│  搜索 [_____________] [↑↓]    │   ← search + sort toggle
├───────────────────────────────┤
│  [员工 1 · 2h]  [前台助理]    │   ← row: drag handle, name+hours, position chips
│  [员工 22 · 5h] [前台助理]    │
│  [员工 26 · 2.8h] [前台助理]  │
│  ...                          │
└───────────────────────────────┘
```

Sort order:
- Default: hours ascending (employees with least load first → admin gravitates to under-utilized employees).
- Alternate: name ascending (alphabetical via `localeCompare`).

Search: substring match on employee name, case-insensitive. Empty input shows all.

The directory excludes:
- The bootstrap admin user (admins don't get scheduled).
- Disabled users (`status !== 'active'`). Displayed as grey tomb-row at the bottom *only if* they currently appear in any assignment (so the admin sees they need to fix a stale assignment). Otherwise hidden.

Each employee row shows:
- Drag handle (the entire row is draggable via `useDraggable` with id `directory:${userID}`).
- Name + total hours (sum across applied + draft assignments, computed via the existing `computeUserHours`).
- `user_positions` rendered as small position-name chips (e.g., `前台助理`, `外勤负责人`).

**Rejected — render the directory as a side drawer:** drawer hide/show animation interferes with drag flow. Always-visible static panel is simpler.

### D-4. Directory comes from the API's top-level `employees` array

The assignment-board API now returns a flat top-level `employees` array (see D-12 for shape). The frontend's `deriveEmployeeDirectory` collapses to a trivial `Map` builder:

```ts
function deriveEmployeeDirectory(employees: AssignmentBoardEmployee[]): Map<number, Employee> {
  return new Map(
    employees.map((e) => [e.user_id, {
      user_id: e.user_id,
      name: e.name,
      email: e.email,
      position_ids: new Set(e.position_ids),
    }]),
  )
}
```

The backend filters the array (excludes bootstrap admin, excludes disabled users, restricts `position_ids` to positions appearing in the publication's slots), so the frontend doesn't re-filter.

**Rejected — keep the per-cell aggregation as a fallback:** would require keeping `candidates` / `non_candidate_qualified` in the response, defeating the API trim's purpose. Frontend trusts the API.

**Rejected — fetch `GET /users` separately:** the assignment-board response is already the contextualized cut (active + qualified for some position in this publication). `GET /users` returns the universal user list including disabled and admin users; would need re-filtering on the frontend.

### D-5. Drag color feedback

The page-level state owns `draggingUserID: number | null`, set on `onDragStart`, cleared on `onDragEnd` / `onDragCancel`.

Each seat's render decides its border color based on the seat's `positionID` and the dragged user:

```tsx
function seatBorderClass({
  draggingUserID,
  directory,
  positionID,
  isOffSchedule,
}: SeatBorderProps): string {
  if (isOffSchedule || draggingUserID === null) return "border-default"
  const employee = directory.get(draggingUserID)
  if (!employee) return "border-default"
  return employee.position_ids.has(positionID)
    ? "border-green-500 bg-green-50"
    : "border-amber-500 bg-amber-50"
}
```

Off-schedule cells stay shaded; never colored.

**Rejected — three-color scheme (green / yellow / gray for "completely doesn't belong"):** every seat has exactly one position and the user is either qualified for that position or not. There's no third state.

### D-6. Click-`×` on a chip

The seat chip's `×` icon is a click target. Click stages a `unassign` draft entry for that `(slotID, weekday, positionID, assignmentID)`. The seat then re-renders with the chip strikethrough + a `+` to undo (clicking strike-through chip's body cancels the `unassign` draft).

This is the only click-based mutation. Right-panel directory rows are not click-to-stage — clicking a directory row does nothing (drag is the only mutation path from the directory).

**Rejected — click-to-stage from directory row:** ambiguous. "Click which person → assign to which cell?" needs a target, which is what drag provides naturally. Forcing a click-then-target flow re-introduces the modal pattern we're removing.

### D-7. Cross-cell drag = seat-to-seat

Drag from a filled seat to another seat behaves as today's "cross-cell move": the source seat's user is unassigned, the target seat is assigned. The same `isUnqualified` flag fires when the target seat's position isn't in the source user's `user_positions`. Confirmation dialog continues to handle warnings on Submit.

Drag from a filled seat to its own seat (i.e., the drag overlay returns to where it started) is a no-op.

### D-8. Gap banner

A small banner above the directory:

```
仍缺 3 个 cell
```

Or, if all cells are full:

```
全部 cell 已满 ✓
```

The previous shipped design's "click-to-jump gap list" is removed. Admins scan the grid (color status badges still render at the cell level) for gap location; the banner is a count-only summary.

**Rejected — keep the click-to-jump gap list:** without selection state there's nothing to "jump to." The grid is always fully visible.

### D-9. Component shape

```
assignment-board.tsx (page, ~200 lines)
├── grid section (left, ~70%)
│   └── assignment-board-grid.tsx (~120 lines)
│       └── assignment-board-cell.tsx (~150 lines, seat-stacked)
│           └── assignment-board-seat.tsx (~100 lines, useDroppable + render)
└── right-panel section (right, fixed width)
    └── assignment-board-side-panel.tsx (~80 lines)
        ├── gap banner (~20 lines inline)
        ├── search + sort controls (~30 lines inline)
        └── assignment-board-employee-row.tsx (~80 lines, useDraggable + render)
assignment-board-directory.ts derives the global employee directory from the existing assignment-board payload.
```

Files **deleted**:
- `assignment-board-cell-editor.tsx` (selection-driven editor goes away)
- `assignment-board-summary-view.tsx` (banner moves into the side panel)
- `assignment-board-candidate-chip.tsx` (no candidate concept)
- `assignment-board-assigned-chip.tsx` (seat owns the filled-chip rendering)

Files **renamed** (consider): `assignment-board-side-panel.tsx` no longer routes between editor and summary; it IS the directory. Name stays for continuity.

### D-10. Draft state continuity

Existing `draft-state.ts` stays. Entry shape unchanged: `{ slotID, weekday, positionID, userID, isUnqualified, ... }`. The new seat-stacked rendering reads draft entries the same way the old per-position block did. `applyDraftToBoard` produces the same projected board the new code consumes.

The "click candidate to stage assign" flow goes away (no candidate chips); the only mutation entry points are now:
- Drag from directory row → `assign` (or replace on filled target).
- Drag from filled seat → `unassign` source + `assign` target.
- Click `×` on filled seat chip → `unassign`.
- Click strike-through (re-engaged) chip → cancel pending `unassign`.

### D-12. Backend response shape

`GET /publications/{id}/assignment-board` SHALL return:

```json
{
  "publication": { "id": ..., "state": ..., ... },
  "slots": [
    {
      "slot": { "id": 12, "weekday": 1, "start_time": "09:00", "end_time": "10:00" },
      "positions": [
        {
          "position": { "id": 1, "name": "前台负责人" },
          "required_headcount": 1,
          "assignments": [
            { "assignment_id": 100, "user_id": 34, "name": "员工 34", "email": "..." }
          ]
        }
      ]
    }
  ],
  "employees": [
    { "user_id": 1, "name": "员工 1", "email": "...", "position_ids": [2] },
    { "user_id": 22, "name": "员工 22", "email": "...", "position_ids": [2] },
    ...
  ]
}
```

Compared to the previous shape:

- **Removed** per-`positions[]` entry: `candidates`, `non_candidate_qualified`. (Kept: `position`, `required_headcount`, `assignments`.)
- **Added** top-level: `employees[]`.

`employees[]` rules (enforced server-side):

- Each entry's `position_ids` is the user's `user_positions ∩ { positions appearing in this publication's slots }`. Qualifications outside the publication's universe are excluded as noise.
- Excludes the bootstrap admin user (`is_admin = true`).
- Excludes users with `status != 'active'`.
- Excludes users with no qualifying intersection (i.e., `position_ids = []`).
- Sorted ascending by `user_id` for stability.

The auto-assigner does NOT consume this endpoint — it queries `availability_submissions` join `template_slot_positions` join `user_positions` directly inside the service layer. The trim is purely about the HTTP-facing response shape.

**Rejected — split into two endpoints (`/assignment-board` for cells, `/employees-for-publication` for directory):** doubles the round trip on page load. The two pieces of data co-render on the same page and rarely live independently.

**Rejected — keep `candidates` per-pair as an opt-in flag:** flag-driven response shapes are debt. The new UI doesn't use it; the auto-assigner doesn't use it; nothing else uses it.

### D-13. Backend implementation sketch

Service layer:

```go
type AssignmentBoardEmployee struct {
    UserID      int64   `json:"user_id"`
    Name        string  `json:"name"`
    Email       string  `json:"email"`
    PositionIDs []int64 `json:"position_ids"`
}

type AssignmentBoardResponse struct {
    Publication ...
    Slots       []AssignmentBoardSlot
    Employees   []AssignmentBoardEmployee  // NEW
}

type AssignmentBoardPositionEntry struct {
    Position          Position
    RequiredHeadcount int
    Assignments       []AssignmentBoardAssignment
    // candidates and non_candidate_qualified removed
}
```

Repository: a single query (or two queries) builds `Employees`:

```sql
SELECT u.id, u.name, u.email, ARRAY_AGG(DISTINCT up.position_id ORDER BY up.position_id) AS position_ids
FROM users u
JOIN user_positions up ON up.user_id = u.id
WHERE u.status = 'active'
  AND u.is_admin = false
  AND up.position_id IN (
    SELECT DISTINCT tsp.position_id
    FROM template_slot_positions tsp
    JOIN template_slots ts ON ts.id = tsp.slot_id
    WHERE ts.template_id = $1  -- the publication's template_id
  )
GROUP BY u.id, u.name, u.email
ORDER BY u.id;
```

Existing per-pair `assignments` query stays (one row per current assignment). Per-pair `candidates` and `non_candidate_qualified` queries get deleted.

### D-14. Tests

Frontend unit tests at the component layer:

- Cell seat rendering: composition `{lead × 1, assistant × 2}` produces 3 seats; assignments stack-fill in `assignment.id` order; over-assignment renders an overflow row.
- Seat drop targets: drop directory row on empty seat → `assign` entry; drop on filled seat (different user) → `unassign` + `assign`; drop filled-seat chip on another seat → `unassign` source + `assign` target; off-schedule cells reject drops.
- Drag color feedback: during drag, seats whose `positionID ∈ employee.position_ids` render green; others render yellow; off-schedule cells stay shaded.
- Directory: search filters by name (case-insensitive substring); sort toggles between hours-asc and name-asc.
- Gap banner: shows correct count; updates as drafts add/remove assignments.
- Click `×` flow: chip × stages unassign; clicking strike-through chip cancels.
- Submit / Discard / failure handling: existing draft-state tests carry forward; submit handler is unchanged.

Backend tests:

- Repository / handler test: `GET /publications/{id}/assignment-board` response now carries `employees[]` at top level; per-position entries no longer carry `candidates` or `non_candidate_qualified`.
- `employees[]` filter rules: bootstrap admin excluded; disabled users excluded; users with no qualifying position in this publication excluded; `position_ids` restricted to publication-relevant positions.
- Existing per-`(slot, position)` `assignments` array contents unchanged — that surface is preserved.

## Risks / Trade-offs

- **Risk:** the grid grows taller because each cell stacks 2-3 seats instead of one badge. → Mitigation: each seat is one line; daytime cells become 3 lines, evening 2 lines. Total grid height ≈ 2-3× before. 1280px wide × ~600px tall fits a typical desktop viewport without scroll, but a long publication might require vertical scroll (acceptable).
- **Risk:** dragging an employee row redraws border styles on up to ~80 seats every animation frame. → Mitigation: parent owns a single `draggingUserID: number | null` state; per-seat boolean is `O(1)` (set lookup). React only re-renders changed cells.
- **Trade-off:** removing the per-cell candidate list hides "who submitted availability" information from admins entirely. → Acceptable: admins still see who's in the position via assignment chips, and the auto-assigner respects submissions on its own. If an admin needs to know "did Alice submit availability" they can check the availability-submission page separately.
- **Risk:** filled-seat-to-filled-seat replace (vs the old swap) might confuse admins who try to "swap" by dragging A onto B and expect B to land in A's seat. → Mitigation: the deferred-submission draft model means a wrong drag is one click away from undo (Discard, or click the cancel icon on each draft entry). Document the behavior in the spec scenarios.
- **Risk:** shipping a UX rewrite a day after the previous ships churns the spec history (the `Admin assignment board drag-drop and draft submission` requirement is now on its third version in three days). → Acceptable: each version is fully self-contained; the archive folder retains predecessors for posterity.
- **Risk:** dropping `candidates` and `non_candidate_qualified` from the API is a breaking change for any caller that was reading them. → Mitigation: the only caller is the frontend page being rewritten in this same change; no external consumers exist. Backend unit tests assert on the new shape only.
- **Trade-off:** the new `employees[]` query joins `users` × `user_positions` × the publication's slot positions in one shot. Bigger query than the old per-pair `candidates` / `non_candidate_qualified` builders, but only one trip and the indexes already exist (`user_positions(user_id, position_id)`). Negligible at expected fleet sizes.

## Migration Plan

Single shipping unit, backend + frontend together:

1. Apply per `tasks.md`.
2. CI runs the full backend gate (`go build`, `go vet`, `go test`, `go test -tags=integration`, `govulncheck`) plus the frontend gate (`pnpm lint && pnpm test && pnpm build`).
3. Manual smoke: with realistic seed loaded, walk a publication's assignment page — confirm seat-stacked cells, directory on the right populated from the API's `employees[]`, drag from directory shows green/amber highlights, drop on empty seat assigns, drop on filled seat replaces, click × unassigns, Submit replays draft, Discard clears.
4. Rollback = revert; no schema migration to undo. The previous API response shape ships back with the revert.

## Open Questions

None.
