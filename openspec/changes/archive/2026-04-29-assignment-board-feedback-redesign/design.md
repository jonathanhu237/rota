## Context

The assignment board today communicates two unrelated dimensions through the same visual channel:

1. **Draft state** — has the admin's edit been saved to the server, or is it still client-side?
2. **Override severity** — is this assignment a routine match (qualified + employee submitted), an admin override of the employee ("they didn't say they could come"), or an admin override of the system ("they're not qualified for this role")?

Currently both dimensions get expressed as text badges (`[新增]`, `[移除]`, an `<AlertTriangle>` icon for `isUnqualified`). The two collapse into the same noisy band of chip decorations, and the most important distinction (severity) is partially missing — "qualified but didn't submit availability" looks identical to "qualified and submitted".

The user observed this in two ways. First, the per-cell `[新增]` / `[移除]` badges make the board read like a diff log instead of a schedule, which makes "what's the actual schedule going to look like?" hard to answer at a glance. Second, the right-hand directory shows 7 employees at "0 小时" — these turn out to be employees who never submitted availability (so the algorithm correctly skipped them), but the directory's stats line ("X 人无班") visually equates them with employees the algorithm allegedly mishandled. That second misperception is what triggered the (later-rolled-back) auto-assign-hours-fairness change attempt.

The root fix is a clean separation of concerns:

- **Draft state** gets one minimal visual cue — small enough to scan past on a "what's the schedule?" read.
- **Override severity** gets a stronger, color-coded visual — green for routine, amber for "admin overrode the employee", red for "admin overrode the system".
- **Directory** physically separates submitters from non-submitters so stats can describe the algorithm's input fairly, and admins can find non-submitters when they need to deliberately override.

The seven design-question outcomes from the grill-me session (recorded as decisions D-1..D-7 below) are the agreed-on encoding of that separation.

## Goals / Non-Goals

**Goals:**

- Per-chip draft state communicated by the lightest possible cue (left-border accent for adds, strikethrough text for removes, single-click undo on each).
- Per-chip override severity communicated via existing `<AlertTriangle>` with color encoding the gradient.
- Drag-target highlight matches the chip's eventual override state, so the admin's hover preview agrees with the post-drop chip color.
- Submit-time confirmation dialog gives one consolidated review of all override drafts (red severe, amber mild) instead of two separate dialogs or only red.
- Directory honestly distinguishes "this employee was input to the algorithm" from "this employee was not", with stats restricted to the former.
- Beforeunload protection so a misclick doesn't drop the admin's pending edits.

**Non-Goals:**

- Algorithmic changes (auto-assign, candidate pool, fairness optimisation): out of scope; tracked in a separate future change if needed.
- Keyboard-only navigation / full a11y for the assignment board: out of scope.
- Admin-configurable severity thresholds, dialog suppressions, or "always allow X without confirmation" toggles: YAGNI.
- Cross-publication draft persistence (autosave to backend): out of scope; beforeunload is the entire draft-loss-prevention story.
- Replacing the `<AlertTriangle>` with a different icon family: keep existing iconography, just colorize.

## Decisions

### D-1: Pending-add chip = small dot before the name, no text badge

**Decision:** Replace the existing `<Badge variant="outline">{t("assignments.drafts.added")}</Badge>` on isDraft chips with a small filled circle (`<span className="size-1.5 shrink-0 rounded-full bg-primary" aria-hidden />`) rendered immediately before the employee name inside the chip body. The chip's container, height, padding, border, and icons are otherwise unchanged from a normal saved chip — no left-border accent, no ring outline.

**Why:**

- Routine drag (qualified + submitted) is the dominant operation in admin workflow. Visual debt accumulates fastest here, so it gets the lightest cue.
- A 6px dot inside the chip body is the smallest visual signal that still survives a glance — it sits near the eye's natural fixation point (the start of the name text), so admins running their eye across a row pick it up without effort.
- Color: `bg-primary` (the existing primary token) signals "active / current edit" without introducing a new semantic color.
- Crucially, this keeps the chip's outer geometry identical to a saved chip, so cells visually read as "the schedule, with a few subtly-marked drafts" rather than as "a diff log against the saved schedule".

**Alternatives considered:**

- 4px left-border accent (`border-l-4 border-l-primary`): considered and prototyped. Rejected because it changes the chip's outer shape, conflicting with the user's "looks like a normal schedule" goal. Also collides with the roster's existing "today column" ring/border styling, blurring the visual vocabulary.
- 4px left-border + ring outline: even louder than the bar alone; rejected for the same reason, more emphatically.
- Right-edge dot or top-right pip: harder to scan because the eye doesn't naturally stop at the right edge of a chip.
- Background tint: too heavy; competes with cell-coverage tinting.
- Keep the text badge: rejected by the user explicitly.

### D-2: Pending-remove chip = strikethrough text + opacity-70 fade + X→↩ icon

**Decision:** When `isRemoved` is true, render the chip name with `line-through text-muted-foreground` AND apply `opacity-70` to the chip container, then replace the trailing `<X>` icon with a lucide `<Undo2>` icon. Click on the icon (or the chip body) cancels the removal.

**Why:**

- Strikethrough is the universally understood "this is being deleted" affordance — admins don't need to learn anything new.
- Strikethrough alone, on a chip otherwise rendered at full intensity, can read as a CSS rendering glitch ("why is this name crossed out but the chip still vibrant?"). Pairing it with `opacity-70` resolves the dissonance: the whole chip enters a "fading away" visual state that matches the semantic.
- Keeping the chip visible (rather than hiding it like the post-submit projection would) preserves the **per-chip undo affordance**: 1 click on `↩` reverses the operation. With the chip hidden, undo would require navigating up to a top-of-page list — 3 clicks vs 1.
- After submit, the strikethrough+faded chip vanishes (the underlying assignment is deleted server-side), so the schedule converges to the "looks normal" state the user wants.
- The visual heaviness of strikethrough+fade vs the lighter add-dot is intentional — removing an employee from a schedule is a more consequential edit than adding one (it can drop a cell from full → partial → empty), and the visual weight matches.

**Alternatives considered:**

- Chip vanishes on remove, undo only via top counter: rejected because it punishes accidental removals (admin can't easily undo without finding the right entry in a list).
- Strikethrough only, no opacity (the original design write-up): considered. Implementation revealed the chip looks "buggy" at full intensity with crossed-out text; opacity-70 is the small added cue that resolves it.
- Strikethrough + small red dot (mirror of D-1's small dot for adds): rejected; strikethrough already encodes "going to be deleted", a red dot is redundant noise.
- Faded opacity-50 chip + small red dot, no strikethrough: workable but less semantic than strikethrough.

### D-3: Drag-highlight = three colors keyed off (qualification, submission)

**Decision:** [assignment-board-seat.tsx:154-170](frontend/src/components/assignments/assignment-board-seat.tsx#L154-L170)'s `getDragClassName` extends from two-color (qualified/unqualified) to three-color:

```ts
function getDragClassName({ draggingUserID, directory, slotID, weekday, positionID }) {
  if (draggingUserID === null) return "border-border"
  const draggedUser = directory.get(draggingUserID)
  if (!draggedUser) return "border-border"

  const qualified = draggedUser.position_ids.has(positionID)
  if (!qualified) return "border-red-500 bg-red-50 dark:border-red-800 dark:bg-red-950/25"

  const submitted = draggedUser.submitted_slots.has(slotWeekdayKey(slotID, weekday))
  if (!submitted) return "border-amber-500 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/25"

  return "border-emerald-500 bg-emerald-50 dark:border-emerald-800 dark:bg-emerald-950/25"
}
```

**Why:**

- Color gradient (red ≪ amber ≪ green, severity decreasing) matches the existing roster/availability pattern (red empty, amber partial, green full). Admins already speak this color language; we reuse it instead of inventing a new palette.
- Red wins over amber when both apply (unqualified is a stricter override than unsubmitted), because the qualification gate is harder for the system to validate post-hoc than the availability gate.
- The pre-drop hover color agrees with the post-drop chip color (D-4) — admins see the same severity signal twice (drag and result), reinforcing the override category.

**Dependency:** the directory's `Employee` type must carry `submitted_slots` so the frontend can answer the submission question without an extra fetch. See D-7.

### D-4: Dropped chip = AlertTriangle icon, color-coded by severity

**Decision:** Extend `ProjectedAssignment` with a new `isUnsubmitted: boolean` flag (alongside the existing `isUnqualified`). Render:

```tsx
{filledBy.isUnqualified && (
  <AlertTriangle className="size-3.5 text-red-500" aria-label={t("...unqualified")} />
)}
{filledBy.isUnsubmitted && !filledBy.isUnqualified && (
  <AlertTriangle className="size-3.5 text-amber-500" aria-label={t("...unsubmitted")} />
)}
```

When both flags are true (admin dropped someone who is both unqualified AND didn't submit), only the red icon shows — the more severe override visually subsumes the milder one, avoiding double-icon clutter.

**Why:**

- Reuses existing `<AlertTriangle>` import — no new icon dependency.
- Color encoding matches D-3 (red severe, amber mild), so admins see continuity from drag to drop.
- The single-icon approach keeps chip width within current bounds; double-icon would force chip layout changes.

**Alternatives considered:**

- Distinct icons (e.g., `<Clock>` for unsubmitted, `<AlertTriangle>` for unqualified): rejected because no canonical icon means "didn't submit availability"; admins would have to learn the pairing.
- Colored chip border: rejected, conflicts with D-1's left-border accent for draft state.
- Background tint: rejected, conflicts with cell-status tinting.

### D-5: Submit confirmation dialog = single dialog, two color-coded sections

**Decision:** [draft-confirm-dialog.tsx](frontend/src/components/assignments/draft-confirm-dialog.tsx) accepts both `unqualified` and `unsubmitted` draft lists and renders them in two stacked sections, red on top, amber below. Single confirm button at the bottom commits all drafts at once.

```
确认资格 / 可用性破例
━━━━━━━━━━━━━━━━━━━━
🔴 资格不匹配 (N)
  • 员工 A → 09-12 周一 / 前台负责人
    "员工 A 没有'前台负责人'的资格"
━━━━━━━━━━━━━━━━━━━━
🟡 未提交空闲时间 (M)
  • 员工 B → 13-15 周三 / 外勤助理
    "员工 B 没有提交此班次的空闲时间"
━━━━━━━━━━━━━━━━━━━━
              [取消]  [确认并提交]
```

**Edge cases:**

- Only red drafts: title stays as the existing "确认资格覆盖", only the red section renders.
- Only amber drafts: title becomes "确认未提交破例", only the amber section renders.
- Neither: dialog doesn't open (current behavior preserved).

**Why:**

- One admin action ("submit my edits") deserves one confirmation surface. Splitting into two dialogs would make the admin click through two prompts and treat the second one as ceremony rather than review.
- Red on top forces the admin's eye to start at the most severe overrides. If amber is shown first and is long, the admin may lose focus before reaching the red section.
- Severity matches D-3, D-4 — same color story.

**Alternatives considered:**

- Two sequential dialogs: rejected per above.
- Skip dialog for amber, rely on chip warning only: rejected because chip warning is a "moment-of-action" cue and doesn't enforce "moment-of-commit" review when admin is in submit-and-go mode.
- Configurable opt-out of amber dialog: YAGNI.

### D-6: Directory = two stacked sections; stats only count submitters

**Decision:** [assignment-board-side-panel.tsx](frontend/src/components/assignments/assignment-board-side-panel.tsx) renders the directory as two sections:

```
[🔍 search]
[Sort: hours | name]      ← controls top section only

已提交空闲时间 (X)
  平均 ... · 范围 ... · 标准差 ...
  N 人未排上                    ← only when N > 0, amber text
  ━━━━━━━━━━━━━━━━━━━━━━
  ⠿ 员工 12 (4h) 前台助理        ← sortable, draggable, normal styling
  ...

未提交空闲时间 (Y)            ← muted background, lighter divider
  ━━━━━━━━━━━━━━━━━━━━━━
  ⠿ 员工 10 未提交 前台助理      ← no hours, alphabetical, draggable
  ...
```

`computeDirectoryStats` accepts only the **submitter** subset's hours array. The "X 人无班" warning is renamed to "X 人未排上" (i.e., submitters who got 0 hours despite being viable candidates) — currently always 0, but a future-proof slot for any algorithm that does leave viable candidates unassigned. The text color is `text-amber-700 dark:text-amber-300`, **not red**: this signal is "needs admin's attention", not "needs immediate action" — keeping it amber preserves the system's gradient (red = severe / cell empty / unqualified, amber = mild / partial / unsubmitted) and avoids competing for attention with the truly severe red signals on cells and chips.

Sort buttons control the upper section only; the lower section is fixed alphabetically by name. Search filters across both sections (admins searching by name shouldn't have to know which section the person lives in).

**Why:**

- Splits the legitimate algorithm-output stats from the "didn't participate" baseline. The misleading "7 人无班" red-warning is replaced with a precise "0 人未排上" (amber, encouraging) plus a separate "7 人未提交空闲时间" descriptive count in the section header.
- "Sort by hours ascending" is now meaningful: the lowest-hours submitter is the first relevant row, not seven non-submitters at 0h burying the real signal.
- Non-submitters are still findable for the rare deliberate-override case (drag a non-submitter onto a cell — which is now visually loud per D-3 and D-4).

**Why not hide non-submitters by default with a toggle:**

- Adds UI surface (a checkbox/segmented control) for a behavior most admins won't change.
- Hides relevant data (admin may want to know "did everyone submit?" at a glance).

### D-7: Backend extension — `AssignmentBoardEmployee.SubmittedSlots`

**Decision:** Extend [model.AssignmentBoardEmployee](backend/internal/model/assignment.go) with:

```go
type AssignmentBoardEmployee struct {
    UserID         int64
    Name           string
    Email          string
    PositionIDs    []int64
    SubmittedSlots []SubmittedSlot   // new
}

type SubmittedSlot struct {
    SlotID  int64
    Weekday int
}
```

Update [`PublicationRepository.ListAssignmentBoardEmployees`](backend/internal/repository/assignment.go#L776) to also load each employee's submissions for this publication via a second query (or a `LEFT JOIN` if performance allows; details in 2.x). Update [response.go](backend/internal/handler/response.go) to serialise the new field as `submitted_slots`. Update the clone helper in [publication_pr4.go](backend/internal/service/publication_pr4.go) to deep-copy the new slice.

**Why:**

- The frontend needs to answer "did user X submit availability for slot S on weekday W?" inside drag-handler code — a per-drag fetch would be unacceptable. Caching the answer in the directory payload is the cleanest path.
- No SQL migration: `availability_submissions` already has `(publication_id, user_id, slot_id, weekday)`; we just join it.
- Performance envelope: even with 200 employees × 100 submissions each = 20 000 rows, this is a single query. Realistic seed has ~30 employees × ~10 submissions = a few hundred rows. No risk.

**Alternatives considered:**

- Separate endpoint for "submission map": rejected, doubles the round-trip count on assignment-board open.
- Compute on the frontend by reading individual `availability_submissions` rows from a new endpoint: same problem.
- Inline the data into `ListAssignmentCandidates` instead: candidates are a different shape (per-position, not per-employee) and merging would muddle two unrelated concepts.

### D-8: Beforeunload guard

**Decision:** Add a `useEffect` in [assignment-board.tsx](frontend/src/components/assignments/assignment-board.tsx) that registers a `beforeunload` handler whenever `draftState.ops.length > 0`:

```tsx
useEffect(() => {
  if (draftState.ops.length === 0) return
  const handler = (event: BeforeUnloadEvent) => {
    event.preventDefault()
    event.returnValue = ""
  }
  window.addEventListener("beforeunload", handler)
  return () => window.removeEventListener("beforeunload", handler)
}, [draftState.ops.length])
```

**Why:**

- Drafts are client-state; refresh / tab close / hard navigation drops them silently today. Admins doing 5–10 drag operations and accidentally hitting `Cmd+R` lose all work without warning.
- Browsers display a generic "leave site? changes you made may not be saved" prompt regardless of `event.returnValue` content (modern Chrome/Firefox/Safari ignore custom messages); we still need to set `returnValue` to trigger the prompt.
- The handler attaches/detaches based on draft count so users without pending edits see no prompt.

**Limitation:** SPA route changes (TanStack Router internal navigation) bypass `beforeunload`. Mitigation: TanStack Router's `useBlocker` can be wired separately if we observe admins losing drafts to internal navigation — flagged as a possible follow-up if it bites.

### D-9: Seat order is stable — drafts append after saved chips

**Decision:** [assignment-board-cell.tsx](frontend/src/components/assignments/assignment-board-cell.tsx)'s `getVisibleAssignments` SHALL preserve the order produced by `applyDraftToBoard` and append `removedAssignments` at the end:

```ts
return [...projectedAssignments, ...removedAssignments]
```

`applyDraftToBoard` already returns saved chips in the server's natural order with pending-add drafts appended at the end (in `DraftState.ops` queue order). The previous implementation used `.sort((l, r) => l.assignment_id - r.assignment_id)`, which threw away that order and — because draft chips carry negative `assignment_id`s — sorted drafts to the front. This is the "I dropped on seat 3 but the chip jumped to seat 1" symptom the user reported.

Pending-remove projections render at the end of the cell. The underlying saved assignment is filtered out of `projectedAssignments` by `applyDraftToBoard`'s unassign branch, then a strikethrough/faded version is rebuilt locally in cell.tsx from the unassign op + the original server snapshot. Putting the rebuilt chip back into its original seat would require either suppressing the filter in `applyDraftToBoard` (and overlaying `isRemoved` instead) or threading position metadata through — both larger changes than this fix's scope.

**Why:**

- Admin's mental model is "the schedule has fixed slots; I'm filling them in order". When the admin drops a chip on seat 3 with seats 1-2 already occupied, they expect the new chip to appear at seat 3, not displace seat 1 to seat 2.
- Saved-first / drafts-last preserves the visual stability of saved chips: their seat index never changes mid-edit. Admins can build muscle memory ("Alice is always in the first seat of this cell").
- Drafts queued in `DraftState.ops` order is the admin's own action order — preserving it means the most-recent draft is always at the rightmost / bottom-most position, matching the natural reading direction.

**Why not full seat-index tracking (i.e., honor exactly which seat the admin dropped on):**

- The current rendering loop renders seats 0..(required_headcount-1) and pulls assignments out of the array by index. There's no concept of "seat 3 is occupied but seat 2 is empty" — the array compacts.
- Honoring exact drop position (e.g., dropping on seat 3 while seat 2 is empty fills seat 3 specifically) requires either:
  - Adding a `seatIndex` field to `ProjectedAssignment` and changing the rendering loop to render placeholders for empty intermediate seats, or
  - Tracking seat assignments at the backend with an explicit ordering column and a schema migration.
- Both are substantially larger changes. The simpler stability rule (drafts append) covers the common case (admin drops on the next available seat) and avoids the visual displacement that triggered this complaint, without paying the structural cost. If admins later report needing exact seat-index control, that's a follow-up change.

**Alternatives considered:**

- Status quo (sort by `assignment_id`, drafts get negative IDs and sort first): the bug. Rejected.
- Backend-side seat-index column on `assignments` with a schema migration: too big for this change's scope; deferred to a follow-up if needed.
- Frontend-side seat-index tracking with placeholder slots: same — deferred. Even within frontend it requires non-trivial work to render "empty seat between two filled seats", which the current grid does not support.

## Risks / Trade-offs

- **[Risk]** Admins relied on the explicit `[新增]` / `[移除]` text badges to find pending changes; replacing them with a 4px bar / strikethrough may feel "where did my edits go?" until they recalibrate. → **Mitigation:** the top counter still says "未提交的更改：N 项" (renamed for clarity), so admins always know how many drafts exist. A short rollout-note in the change announcement is enough; no retraining required.
- **[Risk]** Color-blind admins may struggle with the green/amber/red gradient. → **Mitigation:** chip icons (`<AlertTriangle>`) and `aria-label` strings carry the semantic meaning, so screen readers and high-contrast users are unaffected. The gradient is reinforcement, not the only signal.
- **[Risk]** `computeDirectoryStats` change breaks any test that asserts on the "7 人无班" string for the realistic seed. → **Mitigation:** task 4.x audits all directory tests and updates expectations.
- **[Risk]** Backend `SubmittedSlots` field bloats the assignment-board response. → **Mitigation:** even at 50 employees × 50 submissions, the JSON addition is ~10 KB — negligible vs the existing payload. If it ever becomes a real concern (e.g., 500-employee tenants), we can paginate the directory; not needed now.
- **[Trade-off]** Strikethrough (D-2) is heavier than the small dot of D-1, breaking the visual symmetry the user initially expected. We chose semantic clarity over aesthetic symmetry — strikethrough is the canonical "going to be deleted" cue and per-chip undo gains a 1-step affordance.
- **[Trade-off]** Two-section directory takes more vertical space than a single list. We accept the cost because the alternative (single list with per-row tagging) breaks the "sort by hours" affordance for non-submitters.
- **[Trade-off]** No keyboard-only flow means visually-impaired admins still can't use this efficiently. We're not addressing accessibility in this change; that's a separate, larger effort.

## Migration Plan

Frontend-only behavior change plus an additive backend field. On deploy:

1. Backend ships first (or simultaneously) — `AssignmentBoardEmployee.SubmittedSlots` is now populated on every `GET /publications/:id/assignments` response.
2. Frontend ships — uses the new field for drag-highlight and override detection.

If frontend ships before backend (deploy ordering accident), the frontend gracefully degrades: `submitted_slots` is undefined → falsy → all employees treated as "didn't submit" → drag highlights are amber for everyone qualified. A regression but not a failure mode. Acceptable.

**Rollback:** revert the eight frontend files + the backend additive changes. The backend field defaults to `null`/`omitempty`, so revert is mechanical. Spec changes revert mechanically.

**Behavior visible to admins:** chip styling changes immediately on next page load. No coordination needed.

## Open Questions

None — the seven design questions in the grill-me session resolved every open point. D-7 and D-8 are derived constraints that follow mechanically from D-1..D-6. D-9 was added during verify after the user reported "drag-target seat position is not honored"; the same grill-me phase resolved it as the smaller "drafts-append" rule rather than the larger "explicit seat-index" rule.
