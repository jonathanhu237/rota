## 1. Directory derivation helper

- [x] 1.1 Add `frontend/src/components/assignments/assignment-board-directory.ts` exporting `deriveEmployeeDirectory(slots: AssignmentBoardSlot[]): Employee[]` per design D-4. `Employee` is `{ user_id: number; name: string; email: string; position_ids: Set<number> }`. Aggregate users from per-cell `candidates`, `assignments`, `non_candidate_qualified` arrays; dedupe by `user_id`; union position_ids. Verify by `pnpm tsc --noEmit`.
- [x] 1.2 Unit-test `deriveEmployeeDirectory`: same user appearing in multiple cells produces one entry; position_ids accumulate; no duplicates. Verify by `pnpm test assignment-board-directory`.
- [x] 1.3 **(API trim)** Once the backend exposes `employees[]` at the top of the assignment-board response (see group 11), simplify `deriveEmployeeDirectory` per design D-4 (revised): the function takes the API's `employees` array and returns a `Map<number, Employee>` directly — no more aggregation across per-cell arrays. Update [assignment-board-directory.ts](frontend/src/components/assignments/assignment-board-directory.ts) and its test. Verify by `pnpm test assignment-board-directory && pnpm tsc --noEmit`.

## 2. Seat component

- [x] 2.1 Add `frontend/src/components/assignments/assignment-board-seat.tsx` per design D-1, D-2, D-5. Props: `{ slotID, weekday, positionID, headcountIndex, positionName, filledBy: ProjectedAssignment | null, draggingUserID: number | null, directory: Map<number, Employee>, disabled: boolean, isReadOnly: boolean, onUnassignClick, onCancelDraft }`. Render filled (chip + ×) or empty (placeholder) state; use `useDroppable` with id `seat:${slotID}:${weekday}:${positionID}:${headcountIndex}`; compute border color based on dragging-user qualification per D-5. Verify by `pnpm tsc --noEmit`.
- [x] 2.2 Unit-test `assignment-board-seat`: filled state renders chip + ×; empty state renders placeholder; border color flips green/yellow during drag based on `directory.get(draggingUserID)?.position_ids.has(positionID)`; off-schedule cells never render seats (no test on the seat itself, but on the cell-level renderer that owns this).

## 3. Cell renders seat stack

- [x] 3.1 Refactor `frontend/src/components/assignments/assignment-board-cell.tsx` to render the seat stack per design D-1: cell summary header (color status + `已分配 X / 需求 N`) on top, then `<Seat>` blocks grouped by position, with overflow seats (when assignments exceed `required_headcount`) rendered below the regular stack. Off-schedule cells render `—` and no seats. Drop the cell-level click-to-select handler (selection is gone). Verify by `pnpm tsc --noEmit`.
- [x] 3.2 Unit-test cell rendering: composition `{lead × 1, assistant × 2}` produces 1 + 2 = 3 seats in stable order; over-assignment produces overflow; off-schedule cell renders no seats.

## 4. Right-panel directory

- [x] 4.1 Refactor `frontend/src/components/assignments/assignment-board-side-panel.tsx` into the directory shell per design D-3. State owns: search string, sort mode (`'hours' | 'name'`). Renders gap banner on top, search + sort controls, then the employee row list filtered by search and sorted by sort mode. Drop selection branching entirely (no more `selection: AssignmentBoardSelection | null` prop). Verify by `pnpm tsc --noEmit`.
- [x] 4.2 Add `frontend/src/components/assignments/assignment-board-employee-row.tsx` per design D-3. Props: `{ employee, totalHours, positionNames, disabled }`. Renders draggable row via `useDraggable` with id `directory:${userID}` and drag data carrying `{ kind: "directory-employee", employee }`. Verify by `pnpm tsc --noEmit`.
- [x] 4.3 Unit-test directory: search filters by name (case-insensitive substring); sort toggles between hours-asc and name-asc; disabled and bootstrap-admin users excluded.
- [x] 4.4 Implement gap banner at the top of the directory: `仍缺 N 个 cell` when `N > 0`; `全部 cell 已满` when `N == 0`. Pure projection from `pivotIntoGridCells` results.

## 5. Drag handler topology

- [x] 5.1 Rewrite `frontend/src/components/assignments/assignment-board-dnd.ts` per design D-2 + D-7. Drag sources: directory rows (kind `"directory-employee"`) and filled-seat chips (kind `"assigned"`). Drop targets: seats only. Drop handler routes:
  - source = directory + target = empty seat → `enqueueAdd` (one entry).
  - source = directory + target = filled seat (different user) → `enqueueRemove` of existing + `enqueueAdd` of dragged user (two entries).
  - source = directory + target = filled seat (same user) → no-op.
  - source = filled seat (cross-seat) + target = empty seat → `enqueueRemove` of source + `enqueueAdd` of target (two entries).
  - source = filled seat (cross-seat) + target = filled seat (different user) → three entries: `unassign` source seat, `unassign` target seat (existing user), `assign` target seat (dragged user).
  - source = filled seat + target = same seat → no-op.
- [x] 5.2 Drop on a seat whose `positionID ∉ user_positions(draggedUser)` flips `isUnqualified: true` on the resulting `assign` entry. Verify by adding a draft-state test for "drop unqualified user on seat → entry has isUnqualified: true".

## 6. Page-level state

- [x] 6.1 Update `frontend/src/components/assignments/assignment-board.tsx`: remove `selection` state; add `draggingUserID: number | null` state set on `onDragStart` / cleared on `onDragEnd`/`onDragCancel`. Memoize `directory: Map<number, Employee>` from `slots` via `deriveEmployeeDirectory`. Pass `directory` and `draggingUserID` down to grid. Verify by `pnpm tsc --noEmit`.

## 7. Cleanup deletions

- [x] 7.1 Delete `frontend/src/components/assignments/assignment-board-cell-editor.tsx` (selection editor goes away). Verify by grep for stray imports.
- [x] 7.2 Delete `frontend/src/components/assignments/assignment-board-summary-view.tsx` (gap banner moves into the directory). Verify by grep.
- [x] 7.3 Delete `frontend/src/components/assignments/assignment-board-candidate-chip.tsx` (no candidates concept). Verify by grep.
- [x] 7.4 Delete `frontend/src/components/assignments/assignment-board-side-panel-utils.ts` if unused after the directory shell rewrite (the `weekdayKeys` helper may still be referenced elsewhere — keep only what's used). Verify by grep.

## 8. Tests + i18n

- [x] 8.1 Rewrite `frontend/src/components/assignments/assignment-board.test.tsx` for the new topology. Cover: cell renders seat stack; off-schedule cells empty; directory lists employees; search/sort works; drag from directory shows green/yellow border; drop on empty seat assigns; drop on filled seat replaces; click × unassigns; click strike-through cancels. Verify by `pnpm test assignment-board`.
- [x] 8.2 Update `frontend/src/i18n/locales/{en,zh}.json`: add directory strings (`仍缺 N 个 cell / N coverage gaps`, `全部 cell 已满 / All cells filled`, `搜索员工 / Search employee`, `按工时 / by hours`, `按姓名 / by name`, `空缺 / Empty`); drop unused strings from the previous shipped layout (selection-related, candidate-related). Verify by `pnpm lint && pnpm tsc --noEmit`.

## 9. Spec sync

- [x] 9.1 Confirm the change-folder spec delta at `openspec/changes/assignment-board-employee-directory/specs/scheduling/spec.md` matches the implemented behavior (new requirement text + 17 scenarios). Do not edit `openspec/specs/scheduling/spec.md` directly — `/opsx:archive` syncs it.
- [x] 9.2 **(API trim)** Confirm the same spec delta also carries the rewritten `Assignment board surfaces non-candidate qualified employees` requirement (response now has top-level `employees[]`; per-pair `candidates` and `non_candidate_qualified` removed). The delta already includes this block (added during the scope expansion); just verify it matches the implemented backend response shape.

## 10. Final gates

- [x] 10.1 `cd frontend && pnpm lint && pnpm test && pnpm build` — all clean.
- [x] 10.2 Manual smoke with realistic seed: load `/publications/{id}/assignments`, observe seat-stacked grid; observe right-panel directory with 42 employees; type a name in search → filtered; toggle sort → reorders; drag an employee → seats glow green/yellow; drop on empty seat → assigned with hint; drop on filled seat → replace; click × → unassign; Submit clears draft and persists.
- [x] 10.3 `openspec validate assignment-board-employee-directory --strict` — clean.
- [x] 10.4 **(API trim)** Re-run the full backend gate after group 11 lands: `cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./... && govulncheck ./...`. All clean.
- [x] 10.5 **(API trim)** Re-run the frontend gate after task 1.3 lands: `cd frontend && pnpm lint && pnpm test && pnpm build`. All clean.
- [x] 10.6 **(API trim)** Re-run the realistic smoke from 10.2 after the API trim, plus DevTools network inspection: confirm the assignment-board response carries `employees[]` and per-pair entries no longer carry `candidates` / `non_candidate_qualified`.

## 11. Backend assignment-board response trim

- [x] 11.1 Update the response DTO in `backend/internal/handler/publication.go` (or wherever the assignment-board handler lives): add a top-level `employees: []AssignmentBoardEmployee` field; drop `Candidates` and `NonCandidateQualified` from the per-position entry struct. Verify by `cd backend && go build ./...`.
- [x] 11.2 Update the service layer in `backend/internal/service/` (the assignment-board builder — likely `publication.go` or `publication_pr*.go`): remove the per-pair candidates / non-candidates aggregation; add a single query that returns the publication-scoped `employees[]` per design D-13's SQL sketch. The bootstrap admin filter, the `status = 'active'` filter, and the `position_ids` intersection filter all run in SQL. Verify by `cd backend && go vet ./...`.
- [x] 11.3 Update `backend/internal/repository/publication.go` (or the relevant repo file) with the new `employees[]` query. Existing per-pair `assignments` query stays untouched. Verify by integration tests in 11.5.
- [x] 11.4 Update existing service / handler tests to match the new response shape: assertions that read `Candidates` or `NonCandidateQualified` re-target `Employees[]` or are removed. New tests cover the four scenarios in the spec delta (response carries top-level employees array; bootstrap admin and disabled users excluded; users with no qualifying intersection excluded; per-pair shape no longer carries candidates / non_candidate_qualified). Verify by `cd backend && go test ./internal/handler/... && go test ./internal/service/...`.
- [x] 11.5 Run integration tests against the migrated DB: `cd backend && go test -count=1 -tags=integration ./internal/repository/ ./internal/service/`. Clean.
- [x] 11.6 Update `frontend/src/lib/types.ts` (or the assignment-board Zod schema): drop `candidates` and `non_candidate_qualified` from per-position; add top-level `employees: AssignmentBoardEmployee[]`. Verify by `pnpm tsc --noEmit`.
- [x] 11.7 Update any frontend consumer of the dropped fields. After this task lands, [assignment-board.test.tsx](frontend/src/components/assignments/assignment-board.test.tsx) fixtures + [draft-state.test.ts](frontend/src/components/assignments/draft-state.test.ts) fixtures stop referencing the dropped fields. Verify by `pnpm test`.
