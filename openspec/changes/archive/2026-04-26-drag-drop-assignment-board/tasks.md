## 1. Add drag-drop dependency

- [x] 1.1 `cd frontend && pnpm add @dnd-kit/core @dnd-kit/utilities` (and `@dnd-kit/sortable` only if needed for keyboard reorder; otherwise skip). Verify: `pnpm build` runs clean.

## 2. Draft state module

- [x] 2.1 Create `frontend/src/components/assignments/draft-state.ts` exporting:
    - The `DraftOp` discriminated-union type per design D-4 (`{ kind: "assign", ... }` | `{ kind: "unassign", ... }`).
    - A `DraftState` shape: `{ ops: DraftOp[] }`.
    - Reducer functions: `enqueueAssign`, `enqueueUnassign`, `enqueueMove(fromOp, toOp)`, `enqueueSwap(...)`, `enqueueReplace(...)`, `enqueueAdd(...)`. Each takes the current state and the relevant inputs and returns the new state.
    - A pure helper `applyDraftToBoard(serverSnapshot, draftState)` that returns the projected `(slot, position) → user[]` map after applying queued ops to the snapshot. This drives rendering.
    - A pure helper `computeUserHours(snapshot, draftState, userID)` returning total hours for that user in the projected state.
    - A pure helper `discardDrafts()` returning the empty state.
  Verify: unit tests cover each reducer (success cases) plus the projection helper for representative scenarios (move, swap, replace, add).

## 3. Drag interaction wiring on AssignmentBoard

- [x] 3.1 In `frontend/src/components/assignments/assignment-board.tsx`:
    - Wrap the board in `<DndContext>` with `PointerSensor` (and `KeyboardSensor` if accessible drag is in scope).
    - Make each assigned-user badge a `useDraggable`. Make each cell area a `useDroppable`.
    - Make each candidate-panel user a `useDraggable` with a separate "source" tag.
    - On `onDragOver`: compute drop-target color (green/red) per design D-3 and apply via CSS class.
    - On `onDragEnd`: dispatch the appropriate `enqueueXxx` reducer per the source/target table in design D-2.
  Verify: component tests exercise the drag → drop → state-mutation flow for each operation type (use `@testing-library/react` and the `@dnd-kit` test helpers; or simulate the dispatcher directly if the testing library route is too noisy).
- [x] 3.2 Render the projected state from `applyDraftToBoard(...)` (not the raw server snapshot) so admins see their drafts live. Add visual diff hints — a small badge or color tint on cells whose contents differ from the server snapshot.
- [x] 3.3 Display per-user hours in cell badges and in the candidate panel via `computeUserHours`. Format: `Alice (5h)` with single-decimal precision when fractional. Verify: visual regression / snapshot test confirms the suffix appears.

## 4. Submit flow

- [x] 4.1 Add a footer panel to the assignment board showing pending draft op count and two buttons: "Submit" and "Discard drafts". Disabled when ops list is empty.
- [x] 4.2 Create `frontend/src/components/assignments/draft-confirm-dialog.tsx`:
    - Opens iff the queue has any `assign` op with `isUnqualified === true`.
    - Lists each unqualified entry with user name, target cell (weekday + start_time + position name), and a brief reason ("X is not qualified for Y in this publication").
    - Buttons: "Cancel" (returns to draft view, queue intact) and "Confirm and submit" (proceeds to actual replay).
  Verify: component test for both branches (with-warnings → dialog opens; no-warnings → dialog skipped).
- [x] 4.3 Implement the submit replay: walk the queue in order, awaiting each `POST` / `DELETE`. On success, remove the op from the queue and re-render. On failure (4xx or 5xx), stop, annotate the failing op with the error, and surface a toast / inline error. Successful ops stay applied; failed and untried ops remain in queue.
- [x] 4.4 After all ops succeed, refetch the assignment-board from the server (canonical state) and clear the draft queue. Verify: integration-style test mocks the API with one failure mid-queue and asserts the partial-state behavior.

## 5. Internationalization

- [x] 5.1 Add new i18n keys to `frontend/src/i18n/locales/en.json` and `zh.json` for:
    - "Drafts: N pending"
    - "Submit"
    - "Discard drafts"
    - "Confirm and submit"
    - "Cancel"
    - The dialog header and per-row warning template.
    - Hours suffix formatting (e.g., `{{user}} ({{hours}}h)`).
    - Failed-submit toast.
  Verify: lint passes, no missing-key warnings.

## 6. Tests

- [x] 6.1 Unit tests for the draft reducer in `frontend/src/components/assignments/draft-state.test.ts`. Cover at minimum: enqueue MOVE, enqueue SWAP, enqueue REPLACE, enqueue ADD, applying drafts to a snapshot, computing user hours with and without drafts, discard. Verify: `pnpm test` includes these.
- [x] 6.2 Component test for `AssignmentBoard` exercising drag → drop interaction for each of the 4 operation types via the `@dnd-kit` test helpers. At least one test per type. Verify: same.
- [x] 6.3 Component test for `DraftConfirmDialog` covering with-warnings and no-warnings paths. Verify: same.
- [x] 6.4 Submit-flow test mocking the `POST` / `DELETE` mutations and verifying queue advance, partial-failure stop, and post-success refetch.

## 7. Final verification

- [x] 7.1 Frontend clean: `cd frontend && pnpm lint && pnpm test && pnpm build` — every step exits 0.
- [x] 7.2 Backend untouched: `cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./... && govulncheck ./...` — every step exits 0 (regression check; no backend code is modified by this change).
- [x] 7.3 Smoke test (manual):
    - (a) `make migrate-down && make migrate-up && make seed SCENARIO=full`. Login as admin, navigate to the publication's assignment board.
    - (b) Drag an assigned user from one cell to another empty cell — confirm visual MOVE projection (no API call yet).
    - (c) Drag from cell to a full cell — confirm SWAP projection.
    - (d) Drag a candidate onto a user — confirm REPLACE projection.
    - (e) Drag a candidate to an open slot — confirm ADD projection.
    - (f) Drag a user onto a cell whose position they're not qualified for — confirm warning indicator appears.
    - (g) Verify each cell shows the assigned user's running hours; numbers update live as drafts change.
    - (h) Click Submit with no warnings — verify all ops apply, board refetches.
    - (i) Click Submit with at least one warning — verify confirmation dialog lists it, allows Confirm or Cancel.
    - (j) Click Discard drafts — verify queue clears and projection reverts to server state.
    - (k) Click `+` and `×` buttons in the existing fallback path — confirm immediate-commit semantics still work (no draft).
- [ ] 7.4 Confirm CI is green on `change/drag-drop-assignment-board`: `backend-test`, `frontend-test`, `migrations-roundtrip`, `docker-build`, `govulncheck`.
