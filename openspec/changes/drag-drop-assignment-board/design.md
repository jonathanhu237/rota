## Context

The assignment board today is a click-driven grid: each `(slot, position)` cell renders three lists (assigned users / candidates / non-candidate qualified, the last toggleable). Operations are atomic single-clicks against existing endpoints. This change layers a drag-and-drop UX on top with a draft model, while leaving the backend unchanged.

This is a frontend-heavy refactor. The decisions below cover (a) drag library + interaction semantics, (b) draft state shape and submit pipeline, (c) qualification-warning UX, (d) hours computation, and (e) accessibility / fallback path.

## Goals / Non-Goals

**Goals:**

- Single-drag operations for swap (cell↔cell) and replace/add (candidate→cell).
- Real-time hours-per-user displayed in cells, updating as drafts mutate.
- Admin can override the qualification check (already permitted by spec) but is forced to acknowledge each unqualified assignment at submit time.
- Existing API contract preserved: only the existing assign / unassign endpoints are called.
- Keyboard / non-drag fallback retained.

**Non-Goals:**

- Backend changes.
- Atomic / transactional submit.
- Concurrent multi-admin collaboration.
- Mobile / touch primary path.
- Workload caps or overflow validation.

## Decisions

### D-1. Drag library: `@dnd-kit/core`

**Decision:** use `@dnd-kit/core` (with `@dnd-kit/sortable` if needed for keyboard reorder).

**Alternatives considered:**

- *`react-dnd`*: long-established but the API has aged; touch-event handling needs separate backend; the sensors model in `@dnd-kit` is cleaner for our future-proofing.
- *Native HTML5 drag API*: no helper for overlay / collision / accessibility; reinventing wheels.

`@dnd-kit/core` is widely adopted, tree-shakeable, ~10KB gzipped, supports keyboard sensors (drag with `Tab` / arrow keys / `Enter`), and has good TypeScript types. No security or compliance surface.

### D-2. Drag source determines operation

**Decision:** the source of the drag, not the target, drives the operation type. Two sources:

| Source | Target | Operation |
|---|---|---|
| Cell A | empty area (outside any cell) | drag cancel (no-op) |
| Cell A | Cell A | no-op |
| Cell A (user U) | Cell B (has open headcount) | MOVE: U leaves A, joins B |
| Cell A (user U) | Cell B (full, dropped on user V) | SWAP: U → B, V → A |
| Cell A (user U) | Cell B (full, dropped on empty area of cell) | rejected (cell full, no place to land) |
| Candidate panel (user U) | Cell B (open slot in B) | ADD: U joins B |
| Candidate panel (user U) | Cell B (existing user V) | REPLACE: V is removed, U takes the spot |

This gives admins predictable mental shortcuts: "drag a candidate in to bring someone new"; "drag between cells to rearrange the existing roster". The dichotomy maps to the user's instinct.

**Alternative considered:**

- *Always treat as MOVE; SWAP requires modifier key (Shift+drag)*: clever but discoverability is poor. Rejected.

### D-3. Two-color drop targets, override at submit time

**Decision:** drop targets light up green (qualified) or red (not qualified). Drop is permitted on red — the resulting draft entry is marked with a `⚠` icon. Submit triggers a confirmation dialog if any drafts have warnings, listing them so the admin acknowledges each.

The system's rule is already "admin can bypass qualification" (per spec *Qualification gates employee actions*: "Admins bypass this check when creating assignments directly"). The UI just makes the bypass intentional rather than silent.

**Alternative considered:**

- *Hard-block drops on red*: contradicts existing spec — admins are allowed to override. Rejected.
- *Three colors (green / soft yellow override-able / hard red blocked)*: in this UI, structural constraints (slot composition, slot-conflict, disabled user) cannot be reached because:
  - Cells *are* template_slot_positions rows, so the composition trigger never trips.
  - Disabled users are filtered out of every draggable list (spec *Reject assignment of disabled users* at the assignment board read).
  - Per-slot uniqueness is naturally preserved by MOVE/SWAP semantics — the dragged user is removed from their old slot before being added to the new one.
  
  So two colors covers reality.

### D-4. Draft state lives client-side; submit replays operations

**Decision:** drag/drop mutations accumulate in client state (a Zustand store or `useReducer`, keyed per publication-board session). Each draft entry is a typed operation:

```ts
type DraftOp =
  | { kind: "assign"; slotID: number; positionID: number; userID: number; isUnqualified: boolean }
  | { kind: "unassign"; assignmentID: number; userID: number; slotID: number; positionID: number };

type DraftState = {
  ops: DraftOp[];
  // Derived: which (slot, position, user) triples are added vs removed vs unchanged
  // relative to the server snapshot loaded at session start.
};
```

A SWAP produces 4 ops (2 unassign + 2 assign); a REPLACE produces 2 ops (1 unassign + 1 assign); a MOVE produces 2 ops; an ADD produces 1 op.

Submit walks `ops` in order, issuing the matching API call. On failure, processing stops, the failed op stays in the queue with an error annotation, the operations not yet attempted stay queued, and the user is shown the failure with a "retry from here" option. Successful ops are removed from the queue as they apply.

After all ops succeed, the queue is cleared and the board is re-fetched from the server (canonical state).

**Alternative considered:**

- *Backend batch endpoint that takes all ops in one transaction*: cleaner failure semantics but adds backend work and complicates error reporting (admin needs to see which specific op caused the rollback, which means the backend response must enumerate per-op errors anyway). Deferred per non-goal.

### D-5. Hours computation

**Decision:** for each user U, hours = sum over all of U's *current draft-applied* assignments in the publication of:

```
slot.end_time − slot.start_time   (in minutes, then converted to "Xh Ym" or "X.Yh" for display)
```

`current draft-applied` = the set you'd get if the queued ops were committed. This is computed from `(server snapshot) ⊕ (queued ops)`. Frontend has all needed data — slot times come from the assignment-board response, which already includes slot weekday/start/end on each cell.

Display format: `Alice (12h)` with single decimal precision when fractional (e.g., `Alice (10.5h)`). Hours displayed alongside the user name in each cell where they're assigned, plus in the candidate panel (showing what they'd accumulate if dragged in).

### D-6. Qualification check is `user_positions`-based

**Decision:** qualification = does the user have the cell's `position_id` in their `user_positions`? The backend already exposes this via the assignment-board response: each cell has `candidates` (qualified+submitted) and `non_candidate_qualified` (qualified+not-submitted) lists. Together these are the qualified universe for that cell. Anyone NOT in those two lists for a cell is unqualified for that cell's position — drag → red.

The frontend has all data to compute this client-side from the initial board fetch; no extra request per drag.

### D-7. Submit confirmation dialog

**Decision:** dialog appears only when `ops.some(op => op.kind === "assign" && op.isUnqualified)`. Lists each such op as a row:

```
⚠ Alice → Mon 09:00–10:00 / Cashier
   Alice is not qualified for Cashier in this publication.
   
⚠ Bob → Wed 14:00–17:00 / Cook
   Bob is not qualified for Cook in this publication.

[Cancel]   [Confirm and submit]
```

If no unqualified ops, submit fires immediately (no dialog). Cancel keeps drafts in the queue.

### D-8. Failure handling on submit

**Decision:** sequential apply with stop-on-error:

```
for op in ops:
    try POST/DELETE based on op
    if 4xx: stop loop, mark op as failed, surface error
    if 5xx: stop loop, mark op as transient, allow retry
    on success: remove op from queue, render new server state
re-fetch board state on completion
```

Errors render inline next to the failing draft entry. The board re-renders the partial-applied state (some ops may have succeeded). User can fix the offending op (e.g., delete the draft) and click submit again to continue with remaining ops.

### D-9. Accessibility / fallback

**Decision:** the existing `+ / ×` buttons stay. They retain their current immediate-commit semantics (no draft, no warning dialog — fast path for keyboard / accessibility users / admins who prefer click-by-click). The drag-and-drop layer is purely additive.

`@dnd-kit/core` also supports keyboard drag (focus a cell, press Space to pick up, arrow to move, Space to drop). This may be wired up in v1 but isn't required for the change to land.

### D-10. State scope

**Decision:** draft state is per-page-session (not persisted to localStorage). Refreshing the page or navigating away discards drafts. This is intentional — drafts are an in-progress edit, not data the system should remember.

Optional follow-up (not in scope): an "undo last submit" within the same session could restore the queued state; deferred.

## Risks / Trade-offs

- **Risk:** scope creep — "while we're rewriting, let's also add bulk operations / templates / per-user shift caps". → Mitigation: non-goals are explicit; design.md hops over each tempting extension.
- **Risk:** the new dependency adds maintenance surface. → Mitigation: `@dnd-kit/core` is widely adopted and tree-shakeable; risk is comparable to any standard React library we already have.
- **Risk:** users habituated to immediate-commit are surprised by the draft model. → Mitigation: the `+ / ×` buttons keep immediate semantics for those users; the draft model is opt-in via dragging.
- **Trade-off:** SWAP runs 4 sequential API calls. Between them, the slot is briefly inconsistent server-side. → For single-admin usage, the inconsistency window is sub-second. Acceptable per non-goal "atomic swap".
- **Trade-off:** hours are computed entirely client-side. If a future feature needs server-side hours (e.g., reports), the computation logic will need to live in two places. → Acceptable for now; no near-term reporting feature.

## Migration Plan

Single shipping unit on `change/drag-drop-assignment-board`. No database changes, no API changes. Rollback = revert the feature branch's commits.

Frontend installation step: `pnpm add @dnd-kit/core` runs as part of the apply.

## Open Questions

None.
