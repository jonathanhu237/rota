## Why

Today, after auto-assign produces an initial assignment set, admins fine-tune via two atomic operations on the assignment board: click `+` next to a candidate to assign, click `×` next to an assigned user to unassign. Compound operations are tedious — a swap (Alice ↔ Bob across two cells) costs four clicks and briefly leaves both cells empty between operations. There is no draft mode (every click commits immediately, no undo), no per-user workload signal while editing (admin must mentally tally hours), and no warning when an admin assigns someone who didn't actually submit availability for that slot. The auto-assigner's optimization target is total hours per user (MCMF spreading), but the editing UI surfaces zero information about that target.

This change replaces the click-based assignment board with a drag-and-drop UX that:

- Shows each assigned user's running total hours inside their cell, updating in real time as the admin drags.
- Distinguishes "swap" from "replace" by drag source: dragging from cell to cell swaps the two users (zero clicks lost between cells); dragging from the candidate panel to a cell replaces the occupant (or adds, if the cell still has open headcount).
- Colors drop targets green (qualified) or red (qualification mismatch — admin is allowed to bypass per the existing *Qualification gates employee actions* spec note "Admins bypass this check").
- Defers commit: drags accumulate in a draft state visible in the UI; clicking "Submit" applies them. If any change involves a red (unqualified) target, a confirmation dialog lists those overrides before the admin commits.

Backend behavior is unchanged. The existing endpoints `POST /publications/{id}/assignments` and `DELETE /publications/{id}/assignments/{assignment_id}` are the only writes; the new UI issues sequences of these on submit. No new endpoint, no schema change, no new validation.

## What Changes

- **Frontend `AssignmentBoard` component is rewritten** as a drag-and-drop grid using `@dnd-kit/core` (or equivalent). The existing `+ / ×` buttons stay as fallback (keyboard / accessibility) but the primary interaction is drag.
- **Each assigned user's cell badge gains an hours suffix**, e.g., `Alice (12h)`. Hours = sum over the user's draft-applied assignments of `(slot.end_time − slot.start_time)`. Recomputed live as drafts mutate.
- **Drag semantics:**
  - Cell → empty area or cell → its own current cell: no-op (drag cancels).
  - Cell A → Cell B (B has space): MOVE — user is removed from A, added to B.
  - Cell A → Cell B (B is full, including target user): SWAP — the dragged user and the target user trade cells.
  - Candidate panel → empty slot in cell: ADD.
  - Candidate panel → assigned user in cell: REPLACE — incoming user takes the spot, outgoing user is unassigned.
- **Drop-target coloring during drag:**
  - 🟢 green: dragged user is qualified for the target cell's position (their `user_positions` includes it).
  - 🔴 red: dragged user is not qualified for the target cell's position. Drop is *allowed* (admin bypass), but the resulting draft entry is marked with a red exclamation icon for review.
- **Draft-state UI:** the board renders the projected state (after applied drafts), with subtle visual diff hints (changed cells get a small badge or dot). A footer shows pending operation count and a "Discard drafts" button.
- **Submit confirmation:** clicking Submit triggers a confirmation dialog if any draft entries have red exclamations. The dialog lists the unqualified assignments with user / cell context and an explicit "Confirm anyway" button. If no red entries exist, Submit fires without confirmation.
- **Submit execution:** frontend issues the queued operations as a sequence of `POST` (assigns) and `DELETE` (unassigns). On any failure, processing stops and the UI surfaces which operation failed; remaining drafts stay in the queue so the admin can retry or discard. (No backend transactional batch — this is the simpler "option A" tradeoff documented in design.)
- **Existing keyboard / fallback flow** (the `+` / `×` buttons) remains for accessibility and for users who prefer immediate-commit semantics. These stay immediate, no-draft.
- **No backend changes.**

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `scheduling`: adds a small requirement describing the admin assignment-board UX guarantees (real-time hours, qualification warning + override). The actual API contract is unchanged; the new requirement captures user-facing behavior.

## Non-goals

- **Backend batch / transactional submit.** Submit is a sequence of single-op writes. Partial failure is surfaced; no all-or-nothing commit.
- **Atomic swap.** Cross-cell swap is implemented as `DELETE A → POST A-into-B → DELETE B → POST B-into-A` (or similar four-step sequence). Between operations, one of the two cells is briefly empty server-side. Acceptable: the assignment board is a single-admin tool and concurrent writers are rare.
- **Real-time multi-admin collaboration.** If two admins edit simultaneously, each sees their own draft. On submit, the second one to commit may see partial-failure if the first changed an assignment they're trying to mutate. No live cursor / lock.
- **Undo across submits.** Draft-level undo (within an unsubmitted session) is supported via "Discard drafts". Once submitted, undo is a fresh editing session.
- **Workload caps / overflow warnings.** Hours are *displayed* but not *constrained*. Assigning one user 80 hours is allowed; admin sees the number and decides.
- **Mobile / touch drag.** Desktop pointer drag is the supported interaction. Mobile users continue to use the `+ / ×` fallback.
- **Bulk operations** (e.g., "shift everyone in slot S forward by one position"). Out of scope; admin does each move individually.

## Impact

- **Frontend:**
  - Add dependency: `@dnd-kit/core` (and `@dnd-kit/sortable` if used). Standard React drag library; no security surface.
  - `frontend/src/components/assignments/assignment-board.tsx` is significantly rewritten (currently ~290 lines → estimated ~500-600 lines).
  - New: `frontend/src/components/assignments/draft-state.ts` (state machine for queued operations).
  - New: `frontend/src/components/assignments/draft-confirm-dialog.tsx` (the red-exclamation review dialog).
  - `frontend/src/components/assignments/assignment-board-state.ts` may grow to expose hours-tally helpers.
  - i18n strings for the new UI (warnings, dialog buttons, hours suffix).
  - Test surface grows: unit tests for draft reducer, hours tally, qualification color logic; component tests for drag → drop → render outcomes.
- **Backend:** none.
- **Spec:** one new requirement in `scheduling` capability covering the new UX guarantees.
- **No schema migration.**
- **No third-party security review needed** (`@dnd-kit/core` is a widely adopted React library).

## Risks / safeguards

- **Risk:** drag-drop has touch / accessibility pitfalls. **Mitigation:** keep the existing `+ / ×` buttons as keyboard-accessible fallback, and document that primary drag-drop is desktop-only.
- **Risk:** partial-failure on submit leaves the admin in a confusing state ("which 3 of my 5 changes landed?"). **Mitigation:** the UI explicitly surfaces failed operations and keeps undone drafts in the queue; admin can retry or discard.
- **Risk:** hours tally is computed client-side and could drift from server reality if another admin edited concurrently. **Mitigation:** on submit, a successful POST/DELETE returns the canonical assignment row; the UI re-fetches the assignment-board after submit completes. Admin sees server truth, then can edit again if desired.
- **Risk:** scope creep into "atomic batch backend endpoint". **Mitigation:** explicit non-goal; only revisit if the partial-failure UX actually trips users in practice.
