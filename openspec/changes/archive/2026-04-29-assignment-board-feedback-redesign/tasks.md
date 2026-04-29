## 1. Backend — extend `AssignmentBoardEmployee` with submitted slots

- [x] 1.1 Add a `SubmittedSlot` model + extend `AssignmentBoardEmployee` in [model/assignment.go](backend/internal/model/assignment.go) with `SubmittedSlots []SubmittedSlot`. Verify by `cd backend && go build ./...`.
- [x] 1.2 Update [`PublicationRepository.ListAssignmentBoardEmployees`](backend/internal/repository/assignment.go#L776) to also fetch each employee's `(slot_id, weekday)` submissions for this publication. Either second query keyed by user_ids or a `LEFT JOIN` — pick the one with cleaner SQL given the existing `GROUP BY` shape. Verify by `cd backend && go build ./...`.
- [x] 1.3 Add a repository integration test (in `assignment_test.go` under `//go:build integration`) that seeds 3 employees: one with 2 submissions for this publication, one with 0, one with submissions for a different publication. Assert that the returned `AssignmentBoardEmployee` slice carries `SubmittedSlots` matching the seeded rows for the target publication only. Verify by `cd backend && go test -tags=integration ./internal/repository/...`.
- [x] 1.4 Update the response shape in [handler/response.go](backend/internal/handler/response.go) `assignmentBoardEmployeeResponse` to expose `submitted_slots` as `[{ slot_id, weekday }, ...]`. Verify by `cd backend && go test ./internal/handler/...`.
- [x] 1.5 Update the clone helper `cloneAssignmentBoardEmployees` in [service/publication_pr4.go](backend/internal/service/publication_pr4.go) to deep-copy the new slice. Verify by `cd backend && go test ./internal/service/...`.
- [x] 1.6 Update existing service-level handler tests (`backend/internal/handler/publication_test.go` around line 951 — `Employees: []*model.AssignmentBoardEmployee{...}`) to include the new field in their expectations. Verify by `cd backend && go test ./internal/handler/...`.

## 2. Frontend types — extend `Employee`

- [x] 2.1 Update `frontend/src/lib/types.ts` `AssignmentBoardEmployee` (or whatever type carries the directory employees) to include `submitted_slots: { slot_id: number; weekday: number }[]`. Verify by `cd frontend && pnpm tsc --noEmit`.
- [x] 2.2 Update [assignment-board-directory.ts](frontend/src/components/assignments/assignment-board-directory.ts) `Employee` shape to derive a `submittedSlots: Set<string>` (using `slotWeekdayKey()` helper) from the API response, parallel to the existing `position_ids: Set<number>` derivation. Add a helper `slotWeekdayKey(slotID, weekday)` if not already present. Verify by `cd frontend && pnpm tsc --noEmit`.
- [x] 2.3 Update unit tests in `assignment-board-directory.test.ts` to confirm the `Employee` derivation populates `submittedSlots` correctly from the API shape. Verify by `cd frontend && pnpm test assignment-board-directory`.

## 3. Frontend — drag-highlight three-color (D-3)

- [x] 3.1 In [assignment-board-seat.tsx:154-170](frontend/src/components/assignments/assignment-board-seat.tsx#L154-L170), extend `getDragClassName` to accept `slotID` and `weekday` (currently only `positionID`), and implement the three-branch logic from D-3. Wire callers (the cell rendering) to pass the new arguments. Verify by `cd frontend && pnpm tsc --noEmit`.
- [x] 3.2 Add a component test for `getDragClassName` covering all three branches (qualified+submitted, qualified+unsubmitted, unqualified) plus the `draggingUserID === null` baseline. Verify by `cd frontend && pnpm test assignment-board-seat`.

## 4. Frontend — pending-add chip (D-1)

- [x] 4.1 In [assignment-board-seat.tsx](frontend/src/components/assignments/assignment-board-seat.tsx), remove the `<Badge variant="outline">{t("assignments.drafts.added")}</Badge>` rendering on isDraft chips. Add a `border-l-4 border-l-primary` class on the chip container when `filledBy.isDraft && !filledBy.isRemoved`. Verify by `cd frontend && pnpm tsc --noEmit`.
- [x] 4.2 Update / add a component test asserting that an isDraft chip renders no `[新增]` text and DOES carry the left-border accent class. Verify by `cd frontend && pnpm test assignment-board-seat`.

## 5. Frontend — pending-remove chip (D-2)

- [x] 5.1 In [assignment-board-seat.tsx](frontend/src/components/assignments/assignment-board-seat.tsx), replace the `<Badge variant="outline">{t("assignments.drafts.toRemove")}</Badge>` for isRemoved chips with `line-through text-muted-foreground` styling on the name. Replace the `<X>` icon (currently rendered when `!isRemoved && !isReadOnly`) with `<Undo2>` from lucide when `isRemoved`. Click on `<Undo2>` invokes the existing `onCancelDraft(filledBy.draftOpID)` flow. Verify by `cd frontend && pnpm tsc --noEmit`.
- [x] 5.2 Update / add component test asserting an isRemoved chip renders no `[移除]` text, DOES carry strikethrough styling, and clicking the `<Undo2>` icon calls `onCancelDraft` with the correct `draftOpID`. Verify by `cd frontend && pnpm test assignment-board-seat`.

## 6. Frontend — dropped chip override icons (D-4)

- [x] 6.1 In [draft-state.ts](frontend/src/components/assignments/draft-state.ts), extend `ProjectedAssignment` and `DraftAssignOp` with a new `isUnsubmitted: boolean` flag. Update the projection logic to compute it from the directory's `submittedSlots` against the cell's `(slot_id, weekday)`. Verify by `cd frontend && pnpm tsc --noEmit`.
- [x] 6.2 Update [assignment-board-dnd.ts](frontend/src/components/assignments/assignment-board-dnd.ts) `resolveAssignmentBoardDrop` to compute `isUnsubmitted` for the dropped target cell and pass it through `enqueueAdd`. Verify by `cd frontend && pnpm tsc --noEmit`.
- [x] 6.3 In [assignment-board-seat.tsx](frontend/src/components/assignments/assignment-board-seat.tsx), update the `<AlertTriangle>` rendering: keep red icon for `isUnqualified`, add amber icon for `isUnsubmitted && !isUnqualified`, suppress amber when both flags are true. Add `aria-label` strings via i18n. Verify by `cd frontend && pnpm tsc --noEmit`.
- [x] 6.4 Add component tests covering the three icon outcomes (red only, amber only, red wins when both set). Verify by `cd frontend && pnpm test assignment-board-seat`.
- [x] 6.5 Update `draft-state.test.ts` to verify `isUnsubmitted` populates correctly across enqueue, projection, and cancel flows. Verify by `cd frontend && pnpm test draft-state`.

## 7. Frontend — submit confirmation dialog (D-5)

- [x] 7.1 In [draft-confirm-dialog.tsx](frontend/src/components/assignments/draft-confirm-dialog.tsx), accept both `unqualifiedDrafts` and `unsubmittedDrafts` arrays. Render the red section (existing) when unqualifiedDrafts is non-empty; render a new amber section below it when unsubmittedDrafts is non-empty. Adjust the dialog title to: existing "确认资格覆盖" when only red, new "确认可用性破例" when only amber, new "确认资格 / 可用性破例" when both. Single confirm button commits all drafts. Verify by `cd frontend && pnpm tsc --noEmit`.
- [x] 7.2 In [assignment-board.tsx](frontend/src/components/assignments/assignment-board.tsx), update the dialog-trigger logic to open the dialog whenever `unqualifiedDrafts.length + unsubmittedDrafts.length > 0`. Pass both arrays to the dialog. Verify by `cd frontend && pnpm tsc --noEmit`.
- [x] 7.3 Add component tests for the three dialog states (red-only, amber-only, both). Each test asserts the title, the visible sections, and the button behaviour. Verify by `cd frontend && pnpm test draft-confirm-dialog`.

## 8. Frontend — directory two-section split (D-6)

- [x] 8.1 In [assignment-board-side-panel.tsx](frontend/src/components/assignments/assignment-board-side-panel.tsx), partition the `employees` array by `submittedSlots.size > 0`. Render two stacked sections: top "提交了可用性 (X)" with the existing search/sort/stats UX, bottom "未提交可用性 (Y)" with muted background, no hours display, alphabetical-by-name order. Search input filters across both sections; sort buttons control only the top section. Verify by `cd frontend && pnpm tsc --noEmit`.
- [x] 8.2 In [assignment-board-directory.ts](frontend/src/components/assignments/assignment-board-directory.ts), update `computeDirectoryStats` to accept only the submitter subset's hours array. Update `Employee` row rendering to omit hours when the employee is in the non-submitter section. Verify by `cd frontend && pnpm tsc --noEmit`.
- [x] 8.3 Replace the "X 人无班" warning with "X 人未排上" — only renders when at least one **submitter** has 0 assigned hours. (Currently always 0 with the existing algorithm; future-proof for other algorithms.) Verify by component test in 8.4.
- [x] 8.4 Update / add component tests for the side panel covering: section partition, sort applies only to top section, search hits both sections, stats computed over submitters only, "X 人未排上" only renders when relevant. Verify by `cd frontend && pnpm test assignment-board-side-panel`.
- [x] 8.5 Update `assignment-board-directory.test.ts` to confirm `computeDirectoryStats` accepts only the submitter hours array and excludes non-submitters by construction. Verify by `cd frontend && pnpm test assignment-board-directory`.

## 9. Frontend — counter rename + beforeunload guard (D-8)

- [x] 9.1 In [assignment-board.tsx:296](frontend/src/components/assignments/assignment-board.tsx#L296), rename the counter label i18n key from `assignments.drafts.pendingCount` value "草稿：{{count}} 项待提交" to "未提交的更改：{{count}} 项". (Keep the i18n key, change the string in zh.json; en.json adjusted to "Unsubmitted changes: {{count}}".) Verify by `cd frontend && pnpm tsc --noEmit`.
- [x] 9.2 In [assignment-board.tsx](frontend/src/components/assignments/assignment-board.tsx), add a `useEffect` registering a `beforeunload` handler whenever `draftState.ops.length > 0`. The handler calls `event.preventDefault()` and sets `event.returnValue = ""` to trigger the browser's native confirmation prompt. Verify by `cd frontend && pnpm tsc --noEmit`.
- [x] 9.3 Add a component test that mounts `AssignmentBoard` with a non-empty draft state and asserts a `beforeunload` listener is registered (via `window.addEventListener` spy). Mount with an empty draft state and assert no listener registered. Verify by `cd frontend && pnpm test assignment-board`.

## 10. i18n strings

- [x] 10.1 Update `frontend/src/i18n/locales/{zh,en}.json` under `assignments.drafts`:
  - Rename / replace `pendingCount` value
  - Remove the `added` and `toRemove` strings (no longer used)
  - Add new keys for the unsubmitted-override aria labels and tooltip text (e.g., `assignments.drafts.unsubmittedReason: "{{user}} 没有提交此班次的可用性"`)
  - Add new keys for the dialog: `confirmDialog.titleUnsubmitted: "确认可用性破例"`, `confirmDialog.titleBoth: "确认资格 / 可用性破例"`, `confirmDialog.unsubmittedSection: "未提交可用性 ({{count}})"`, etc.
  - Add new keys for the directory sections: `assignments.directory.submitted: "提交了可用性 ({{count}})"`, `assignments.directory.notSubmitted: "未提交可用性 ({{count}})"`, `assignments.directory.notSubmittedTag: "未提交"`, `assignments.directory.unassignedCount: "{{count}} 人未排上"` (replacing `zeroCount`)
- [x] 10.2 Verify by `cd frontend && pnpm lint && pnpm tsc --noEmit`.

## 11. Final gates

- [x] 11.1 `cd backend && go build ./... && go vet ./... && go test ./...`. All clean.
- [x] 11.2 `cd backend && go test -tags=integration ./...`. All clean (Postgres must be running).
- [x] 11.3 `cd backend && govulncheck ./...`. Clean.
- [x] 11.4 `cd frontend && pnpm lint && pnpm test && pnpm build`. All clean.
- [x] 11.5 Manual smoke via Playwright on `localhost:5173/publications/:id/assignments` (with realistic seed loaded and auto-assign run). Walk through the 5 happy paths from design "Verification strategy":
  - 拖一个正常员工 → 看小蓝点、不弹窗、提交后变成普通 chip
  - 拖一个非提交者 → 看 cell 黄框、chip 黄 ⚠、提交时弹窗黄段
  - 拖一个非资格者 → 看 cell 红框、chip 红 ⚠、提交时弹窗红段
  - 移除一个 chip → 看删除线、点 ↩ 撤销
  - 有草稿时刷新页面 → 看 beforeunload 弹框
  Take an "after" screenshot for the archive.
- [x] 11.6 `openspec validate assignment-board-feedback-redesign --strict`. Clean.
