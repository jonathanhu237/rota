## Why

Desktop users can only collapse the sidebar through an undiscoverable rail hover target or the keyboard shortcut. Before delivery, the authenticated shell should expose the shadcn sidebar collapse behavior as a visible control so navigation chrome can be reduced without guesswork.

## What Changes

- Add a visible desktop sidebar toggle control to the authenticated sidebar.
- Use shadcn's icon-collapse sidebar mode so the sidebar contracts to the icon rail instead of disappearing entirely on desktop.
- Preserve the existing mobile navigation trigger and sidebar rail behavior.

## Non-goals

- Do not redesign sidebar grouping, labels, or route destinations.
- Do not add user-configurable sidebar width or drag-resize behavior.
- Do not introduce a new navigation framework or replace the existing shadcn sidebar primitives.
- Do not change authorization or which menu entries each role sees.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `frontend-shell`: Authenticated sidebar behavior gains a visible desktop collapse/expand trigger and icon-collapse mode.

## Impact

- Frontend authenticated shell/sidebar components and tests.
- Frontend-shell OpenSpec requirement for sidebar navigation behavior.
- No backend, database, API, or dependency changes.
