## Why

Two long-running admin pain points on the assignment board surfaced in the same hands-on session and turned out to share one root cause:

1. **"Drag-and-drop creates a noisy mess of `[新增]` / `[移除]` text badges that look nothing like a real schedule."** Admins want the schedule view to read as a schedule until they hit Submit, not as a diff log they have to mentally apply.
2. **"Why are there 7 employees with 0 hours when they're clearly available?"** Inspection showed those 7 employees never submitted availability — they're not even in the candidate pool. The directory and the drag highlights collapse "didn't submit" with "algorithm chose someone else" into a single "0 小时" cell, so the admin reads the page as a fairness bug when it's actually a UI legibility bug.

Both problems trace to the same UI sin: the assignment board's visual feedback **fails to distinguish routine state from exceptional state**. Routine drags get loud per-cell badges; genuine exceptions (admin overriding the qualification gate, or admin overriding the "this employee never said they're available" gate) are visually indistinguishable from a normal assignment.

The fix is to flip the noise budget: routine looks like a schedule, exceptions get the screen real estate they deserve.

## What Changes

- **Pending-add chip** (drop a new assignment that isn't saved yet) gets a 4px left-border accent in the primary blue tone instead of an inline `[新增]` text badge. Same chip footprint as a live assignment; admin scans the grid and sees the schedule, not the diff.
- **Pending-remove chip** stays visible with strikethrough text and the existing `X` button replaced with an `↩` (undo) icon. Strikethrough is the universal "going to be deleted" cue — readable at a glance, recoverable in one click.
- **Drag-highlight** on a target cell gains a third color. Currently green/amber by qualification only; new logic uses **green** (qualified + the dragged employee submitted availability for this slot+weekday), **amber** (qualified but no submission for this slot+weekday — admin override of employee intent), **red** (not qualified — admin override of the qualification system).
- **Dropped chip** carries a parallel `AlertTriangle` indicator: amber for "no submission", red for "no qualification". When both are true, red wins (severe override visually subsumes mild override).
- **Submit confirmation dialog** ([draft-confirm-dialog.tsx](frontend/src/components/assignments/draft-confirm-dialog.tsx)) becomes a single dialog with two color-coded sections: red "资格不匹配 (N)" first, amber "未提交可用性 (M)" below, single confirm button. Edge cases: only-amber renames the title; only-red is the existing behavior; neither = no dialog.
- **Employee directory** ([assignment-board-side-panel.tsx](frontend/src/components/assignments/assignment-board-side-panel.tsx)) splits into two stacked sections: "提交了可用性 (X)" with hours/sort/stats, and "未提交可用性 (Y)" rendered with a muted background and no hours. `computeDirectoryStats` is restricted to submitters — stddev/range/mean are no longer skewed by non-submitters, and the misleading "X 人无班" red warning is replaced with a more precise "X 人未排上" that fires only when a submitter genuinely got nothing.
- **Counter and submit affordance** stay structurally as-is (still a fixed-position row with the count, a discard button, and a submit button), but the label updates from "草稿：N 项待提交" to "未提交的更改：N 项" — less jargon, more direct.
- **Beforeunload guard** added: admins with non-empty drafts get the browser's native confirmation prompt when refreshing, closing the tab, or navigating away. Drafts are client-side only; without this, a misclick wipes them.
- **Backend extension**: `model.AssignmentBoardEmployee` and the response/repository surface gain a `SubmittedSlots []SubmittedSlot` field (each carrying `SlotID` + `Weekday`). Required so the frontend can answer "did this employee submit availability for this slot+weekday?" without an extra round-trip. **No new endpoint, no SQL migration** — the data is already in `availability_submissions` and just needs to be joined into the existing assignment-board response.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `frontend-shell` — adds a new "Assignment board admin feedback" requirement covering the chip / drag-highlight / dialog / directory contract above. Existing requirements (sidebar grouping, roster page, availability page, etc.) are not touched.
- `scheduling` — modifies the existing assignment-board-shape requirement to declare that `AssignmentBoardEmployee` carries the user's submitted `(slot_id, weekday)` pairs alongside `position_ids`. The semantics of auto-assign, candidate generation, and assignment atomicity are unchanged.

## Non-goals

- **Not** changing the auto-assign algorithm, the candidate-pool SQL, or the cost model. The earlier hours-fairness experiment was rolled back; this change does not retry that work or rely on it. Any algorithm-side improvements are a separate change.
- **Not** building keyboard-only navigation. Per-chip undo defaults to a click on the chip itself; full keyboard accessibility (focus traversal, arrow-key cell navigation, keyboard drag) is a separate accessibility-focused change.
- **Not** making the top counter clickable into an expanded list view. Per-chip undo (click `X` on a pending-add or `↩` on a pending-remove) is the canonical undo path; a global expanded list would duplicate that affordance with no new capability.
- **Not** adding admin-configurable knobs to silence the dialog or change the override severity treatment. We have no usage data justifying configuration; if real admins later complain, that becomes a different proposal.
- **Not** introducing service-worker / push / desktop-notification infrastructure for draft persistence. Beforeunload native prompt is the entire "draft loss prevention" story.
- **Not** changing the publication state machine, transaction boundary, or any backend behavior that isn't strictly the directory-data extension.

## Impact

- **Frontend code** — eight files in [frontend/src/components/assignments/](frontend/src/components/assignments/): `assignment-board-seat.tsx`, `assignment-board-cell.tsx`, `assignment-board-dnd.ts`, `draft-state.ts`, `draft-confirm-dialog.tsx`, `assignment-board-side-panel.tsx`, `assignment-board-directory.ts`, `assignment-board.tsx`. Plus `frontend/src/lib/types.ts` for the directory type extension.
- **i18n** — new strings for the unsubmitted-override variants, the dialog's amber section, the directory's two-section labels, and the renamed counter. zh.json and en.json both updated.
- **Backend code** — [model/assignment.go](backend/internal/model/assignment.go) (extend `AssignmentBoardEmployee`); [repository/assignment.go](backend/internal/repository/assignment.go) (`ListAssignmentBoardEmployees` joins `availability_submissions`); [handler/response.go](backend/internal/handler/response.go) (response shape carries the new field); [service/publication_pr4.go](backend/internal/service/publication_pr4.go) (clone helper preserves the new field). No SQL migration.
- **Tests** — extend existing `assignment-board-*` component tests, `draft-state` tests, `assignment-board-directory` tests; add new tests for the unsubmitted detection logic and the two-section dialog. Backend integration tests for the directory loader confirm the new field is populated correctly.
- **Spec** — `frontend-shell/spec.md` adds one requirement; `scheduling/spec.md` modifies the assignment-board-shape requirement.
- **Untouched** — auto-assign, candidate generation, publication state machine, submission/leave/shift-change endpoints, audit logs, migrations.
