## Context

The roster page redesign (archived 2026-04-28) replaced its per-weekday accordion with a 2D `(time block) × (weekday)` grid, and the assignment board has used the same idiom for longer. `/availability` is the last big employee-facing surface still on the old accordion layout — see [availability-grid.tsx](frontend/src/components/availability/availability-grid.tsx) for the existing implementation. The data shape feeding it is `QualifiedShift[]` (one entry per `(slot_id, weekday)` tuple the user is qualified for), distinct from `RosterWeekday[]` (the read-only nested shape the roster consumes).

The page has two functional aspects we keep intact:

1. The toggle: each `(slot_id, weekday)` is a checkbox; checking calls `createAvailabilitySubmission`, unchecking calls `deleteAvailabilitySubmission`. The mutation already invalidates the relevant queries on success/error and optimistically reflects the toggle via the `selectedSlots` prop. We do not change this.
2. The empty / loading / error states already live in [availability.tsx](frontend/src/routes/_authenticated/availability.tsx) at the route level. The grid component only renders when there's data.

## Goals / Non-Goals

**Goals:**

- Replace the per-weekday `<section>` accordion with a single 2D CSS grid: rows = distinct `(start_time, end_time)` tuples sorted ascending; columns = weekdays Mon–Sun; cells = either a checkbox (qualified) or muted "—" (off-schedule for this user).
- Keep the cell visually scannable: the cell body is just the checkbox + the time-block label is on the row, not inside each cell. Position composition (which positions, how many) moves into a tooltip surfaced on hover/focus.
- Rename Chinese user-facing strings: "可用性" → "提交空闲时间" (sidebar nav, page title, page description). English `Availability` and the URL `/availability` stay unchanged.
- Update the `frontend-shell` capability spec with a new "Availability page layout" requirement and align the existing "Sidebar navigation grouping" requirement with the new label.

**Non-Goals:**

- No change to the API protocol, the toggle mutation, the optimistic-update pattern, or the loading/error states at the route level.
- No new dependencies. We reuse `<Checkbox>` and `<Tooltip>` from `frontend/src/components/ui/`.
- No URL change, no English label change, no backend change.
- No mobile-specific tweak in this change. The grid will inherit the same `overflow-x-auto` wrapper pattern as the roster, which handles narrow viewports adequately for this iteration.

## Decisions

### D-1: Pivot helper lives next to the consumer

**Decision:** Add `frontend/src/components/availability/availability-grid-cells.ts` (and `.test.ts`) exporting `pivotAvailabilityIntoGridCells(shifts: QualifiedShift[]): { timeBlocks, weekdays, cells }`. Cells are either `QualifiedAvailabilityCell` (carrying `slot_id`, `weekday`, `composition`) or `OffScheduleAvailabilityCell`.

**Why:** Mirrors the [roster-grid-cells.ts](frontend/src/components/roster/roster-grid-cells.ts) pattern. The data shapes differ enough (`QualifiedShift` is flat, `RosterWeekday` is nested with positions/assignments) that overloading a single helper would cost more in generics than two near-identical files cost in duplication. Each helper is ~50 LOC and has its own tests.

**Alternatives considered:**

- Generic `pivotIntoGridCells<T>` with adapter functions: cleaner in theory; in practice the cell type hierarchy diverges (status totals only make sense for the roster), so the abstraction would leak.
- Inline the pivot in the component: the roster precedent is to keep pivot logic separately tested. Same here.

### D-2: Cell body — checkbox only, composition in tooltip

**Decision:** Each qualified cell renders only a checkbox horizontally centered, with `aria-label` derived from time block + weekday + position summary so screen readers still get full context. On hover/focus the cell shows a `<Tooltip>` listing `{position_name} × {required_headcount}` per position, joined by " / " (matching the existing `availability.shift.compositionEntry` i18n format).

**Why:** Cell density is the whole point of the redesign — putting position composition inline reproduces the visual heaviness the accordion already had, just in a tighter container. Employees are not deciding "do I want to work the cashier slot vs the front-desk slot" on this page — qualifications already gate that, the user-visible question is "can I work this time block on this day". The composition is reference info, not decision input. Hover surfaces it for users who want it.

**Alternatives considered:**

- Show position composition inline as a small subtitle under the time-block label in the row header: doesn't actually solve density — the row header is shared across 7 cells and a multi-position composition would dominate the row.
- Show composition only on click (popover): adds a click for info that should be passive. Tooltip is a better fit.

### D-3: Off-schedule cell renders the muted `—` glyph

**Decision:** When a `(time block, weekday)` pair has no qualified shift entry for this user, render an off-schedule cell — same visual as the roster (`bg-muted/40` + `—` + `aria-label="roster.offSchedule"`-equivalent). The label key for availability is new: `availability.offSchedule` ("排班外" / "Off-schedule"). Reuse `roster.offSchedule` would also work but seeds cross-domain coupling; a parallel key keeps the namespaces clean and lets the strings diverge later if needed.

**Why:** Two semantically distinct off-schedule meanings:

- **Roster's off-schedule:** "this time block doesn't run on this weekday in the publication."
- **Availability's off-schedule:** "this time block runs on this weekday, but you don't have qualifications for the position(s) involved." Or: "no shift for this weekday at all."

Visually they're identical (`—`); semantically employees don't need to disambiguate — both mean "you can't tick this box, this cell is not for you." Identical rendering, parallel i18n key.

### D-4: Today column gets the same `bg-primary/10` highlight as the roster

**Decision:** The header cell for today's weekday (Mon=1..Sun=7) gets `bg-primary/10 text-primary` and the localized "今天" / "Today" badge — identical to the roster. No ring on body cells; the column header alone is enough since the user is interacting (via checkbox) rather than reading.

**Why:** Visual consistency with the roster — the user moves between availability and roster within a single submission cycle, and "today" should be highlighted the same way in both. We omit the body-cell ring (which the roster has) because the checkbox cells are interactive and adding a ring competes with the focus ring.

### D-5: i18n rename — Chinese only, narrow surface

**Decision:** Update three zh.json keys:

- `sidebar.availability`: "可用性" → "提交空闲时间"
- `availability.title`: "可用性" → "提交空闲时间"
- `availability.description`: rephrase to action-framing (e.g., "选择您能值班的时段。") if the current text leans on "可用性 = some property".

en.json keys are NOT touched. Update the `frontend-shell` "Sidebar navigation grouping" spec to describe the entry as `Availability` (English label, since the spec is in English) but note in the requirement text that the localized Chinese label uses the action phrase "提交空闲时间".

**Why:** The user explicitly chose this scope. English `Availability` is the standard scheduling-domain term; rewording it to "Submit free time" or similar would force a new term on English users for no gain. The Chinese rename is purely a UI-copy improvement — entry points should describe actions, not properties.

### D-6: Component shape

```
AvailabilityGrid
├─ pivotAvailabilityIntoGridCells(shifts) → { timeBlocks, weekdays, cells }
├─ <div role="grid"> (CSS grid: 110px repeat(7, minmax(120px, 1fr)))
│  ├─ Header row: corner cell + 7 weekday headers (today highlighted)
│  └─ For each timeBlock: time-block label cell + 7 cells
│     ├─ QualifiedCell: <Tooltip>(<Checkbox>)</Tooltip>
│     └─ OffScheduleCell: muted "—" with aria-label
```

`AvailabilityGrid` keeps the same props (`shifts`, `selectedSlots`, `isPending`, `onToggle`) — the route file [availability.tsx](frontend/src/routes/_authenticated/availability.tsx) doesn't need changes. The narrow-viewport wrapper (`overflow-x-auto`) is added inside `AvailabilityGrid` (matching the roster's pattern) so the route file stays oblivious.

### D-7: Tests cover the new topology

Test coverage matches what we did for the roster:

- Pivot helper: distinct time blocks, off-schedule emission, empty input.
- Component: grid header (corner + 7 weekdays), today badge, off-schedule cells, qualified cells with checkbox state reflecting `selectedSlots`, toggle callback fires with correct `(slot_id, weekday, checked)` tuple, tooltip content visible on hover/focus.

Tooltip behavior in jsdom is finicky — we test that the tooltip element is present in the DOM (with the correct content) rather than asserting on visual visibility. The tooltip lib (Radix-based shadcn) renders the trigger + content even when collapsed.

## Risks / Trade-offs

- **[Risk]** Tooltip-only composition could leave users confused about which positions a slot covers when they're new to the page → **Mitigation:** the cell's `aria-label` includes the composition summary, so screen-reader users get it; sighted users discover it on hover/focus, and the position context was already established on the roster page they read after submission.
- **[Risk]** A shift with many positions (e.g., 4+) makes the tooltip wide → **Mitigation:** tooltip already has `max-w-` constraint via shadcn defaults; long compositions wrap. If this ever becomes a real problem, we can switch to a popover; the change is contained.
- **[Risk]** The grid's `min-w-[1090px]` (matching roster) could feel cramped on tablets → **Mitigation:** `overflow-x-auto` works the same way it does for the roster; not a regression. A separate change can address mobile/tablet polish across all grid pages once the patterns are aligned.
- **[Risk]** Renaming "可用性" without parallel English rename creates terminology drift between zh and en surfaces → **Mitigation:** the in-product term in en stays `Availability`; we only diverge for the Chinese sidebar label and page heading. Tooltips, error messages, and table headers in zh that already use "可用性" deeper in the page (if any) are left as-is for this change to keep scope tight; we can sweep them in a follow-up if any survive.

## Migration Plan

No data migration. Frontend-only change; on next deploy, the grid is reshaped and the Chinese label changes. No backend, no schema, no API.

Rollback: revert the component + i18n diff; the spec change is a pure refinement and reverting is mechanical.

## Open Questions

None — the three design decisions the user already validated (cell body minimal, English unchanged, URL unchanged) cover the open questions surfaced during exploration.
