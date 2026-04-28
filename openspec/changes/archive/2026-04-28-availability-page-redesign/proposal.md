## Why

`/availability` is the most frequent page an employee opens — every collection cycle they walk through their week and tick the shifts they can work. The current page renders one stacked `<section>` per weekday, so to compare "which mornings am I free" the user has to scroll back and forth between seven sections. This is the same accordion problem the roster page had before its redesign; we already shipped a 2D grid for the roster and the assignment board, and the availability page is the last big employee-facing surface still on the old layout. Aligning it removes the cognitive split between "I submit my week here, but read my week there in a totally different shape."

While we're touching the page, the Chinese label "可用性" is an abstract attribute word that doesn't tell employees what they're supposed to do — switching it to the action phrase "提交空闲时间" makes the entry point self-explanatory.

## What Changes

- Rewrite [frontend/src/components/availability/availability-grid.tsx](frontend/src/components/availability/availability-grid.tsx) as a single CSS-grid container with time-block rows and weekday columns, mirroring the roster's layout. Cells render a checkbox + the time-block label; off-schedule (time block doesn't run on this weekday for this user's qualifications) renders the muted `—` cell.
- Surface position composition (which positions need how many people for this slot/weekday) via a tooltip on hover/focus instead of inline text, so the cell stays scannable.
- Add a pivot helper alongside [roster-grid-cells.ts](frontend/src/components/roster/roster-grid-cells.ts) for the `QualifiedShift[]` shape — the data isn't structurally identical to `RosterWeekday[]`, so a small parallel helper is cleaner than overloading the existing one.
- Rename Chinese user-facing strings: `sidebar.availability` and `availability.title` / `availability.description` from "可用性" → "提交空闲时间". English `Availability` stays — availability is the standard term in scheduling-domain English. URL stays `/availability`.
- Update the `frontend-shell` capability spec to (a) reflect the new sidebar label phrasing and (b) add an "Availability page layout" requirement describing the 2D-grid structure, mirroring the existing "Roster page layout" requirement.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `frontend-shell`: extend the existing capability with a new "Availability page layout" requirement and update the "Sidebar navigation grouping" requirement to reflect the new Chinese label for the availability nav entry.

## Non-goals

- **Not** changing the URL `/availability`, the backend endpoint, or the English translation `Availability`. URLs are developer-facing identifiers aligned with the API; the English term is an industry-standard scheduling word.
- **Not** redesigning the availability submission protocol, debouncing, or optimistic-update strategy — only the layout and copy change.
- **Not** moving qualification logic into the cell — qualifications still gate which (slot, weekday) tuples reach the page. Off-schedule cells in this grid mean "no qualified shift for me at this (time block, weekday)", not necessarily "no shift exists for anyone."
- **Not** unifying availability and roster into one component. They share a layout idiom but render different domain objects (selectable vs. read-only) and have different cell bodies.

## Impact

- **Frontend code:** `availability-grid.tsx` (full rewrite), one new helper file under `components/availability/`, optional tooltip component reuse from `components/ui/`. The route file `routes/_authenticated/availability.tsx` may need a 1-line label change but its data wiring is unchanged.
- **Frontend i18n:** zh.json keys `sidebar.availability`, `availability.title`, `availability.description`. en.json untouched.
- **Frontend tests:** rewrite [availability-grid.test.tsx](frontend/src/components/availability/availability-grid.test.tsx) for the new grid topology; rewrite [app-sidebar.test.tsx](frontend/src/components/app-sidebar.test.tsx) only if it asserts on the renamed label (it currently asserts on the i18n key, not the literal label, so likely no change).
- **Spec:** `openspec/specs/frontend-shell/spec.md` — modify the Sidebar navigation grouping requirement, add the Availability page layout requirement.
- **Backend / DB / API:** untouched. No schema migration, no endpoint changes.
