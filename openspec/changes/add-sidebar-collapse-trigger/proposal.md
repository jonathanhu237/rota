## Why

The authenticated shell should match the shadcn navigation pattern more closely before delivery. The current sidebar is fixed and only has an undiscoverable rail/keyboard collapse path, while child pages still rely on scattered "Back to ..." links instead of a consistent breadcrumb hierarchy.

## What Changes

- Render the authenticated sidebar as a shadcn floating sidebar with icon-collapse behavior.
- Move the visible sidebar trigger into the main content header instead of placing it inside the sidebar.
- Add a persistent authenticated header with breadcrumbs on every authenticated route.
- Use dynamic breadcrumb labels for publication, template, and leave detail routes when the relevant record is available, with stable fallbacks while loading.
- Remove redundant page-level "Back to ..." links where breadcrumbs now provide the parent navigation path.
- Preserve existing role-based sidebar entries, unread badge behavior, avatar dropdown actions, and page titles.

## Non-goals

- Do not redesign the product's page content, cards, tables, assignment board, or visual palette.
- Do not introduce shadcn's `sidebar-04` block or replace the existing business sidebar with demo navigation.
- Do not add route-history-aware breadcrumbs; breadcrumbs represent URL/information architecture only.
- Do not optimize for mobile usage beyond keeping the existing shadcn mobile sidebar behavior from obviously breaking.
- Do not change authorization, backend APIs, database schema, or scheduling behavior.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `frontend-shell`: Authenticated shell navigation uses a floating sidebar, main-header collapse trigger, and route breadcrumbs across authenticated pages.

## Impact

- Frontend authenticated layout/sidebar components, new breadcrumb shell component, and related tests.
- shadcn breadcrumb UI primitive added to the frontend.
- Frontend-shell OpenSpec requirement for sidebar and breadcrumb behavior.
- No backend, database, API, or dependency changes.
