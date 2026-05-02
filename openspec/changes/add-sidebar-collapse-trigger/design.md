## Context

The authenticated layout already uses shadcn sidebar primitives. The sidebar component supports `variant="floating"` and `collapsible="icon"`, and `SidebarInset` already provides a compatible main-content container. The application does not yet have the shadcn breadcrumb primitive, and authenticated pages currently express parent navigation inconsistently through page-level back links.

## Goals / Non-Goals

**Goals:**

- Make the authenticated sidebar floating while preserving existing business navigation, role gates, unread badge, avatar dropdown, theme toggle, and logout.
- Put the visible `SidebarTrigger` in the main shell header, followed by a breadcrumb trail.
- Provide breadcrumbs for every authenticated route, with dynamic names for publication, template, and leave detail routes when available.
- Remove page-level back links that duplicate breadcrumb parent navigation.

**Non-Goals:**

- Rebuild the sidebar from a shadcn demo block.
- Change content-page design, table/card structure, or page headings.
- Add mobile-specific design work.
- Change backend behavior or data shape.

## Decisions

- Use existing sidebar primitives instead of importing `sidebar-04`. The current sidebar already contains Rota-specific role-aware navigation and account controls; a block would add cleanup work and demo assumptions.
- Configure `AppSidebar` with `variant="floating"` and `collapsible="icon"`. This gets the shadcn floating treatment and keeps icons available after collapse.
- Render one authenticated header inside `SidebarInset` for all authenticated pages. The header contains `SidebarTrigger`, a vertical separator, and `AppBreadcrumbs`.
- Implement `AppBreadcrumbs` as a centralized route-to-crumb mapper. The route set is small and file routes do not currently define breadcrumb metadata, so central mapping keeps shell navigation logic in one place.
- Dynamic crumbs fetch only the detail query needed by the current path:
  - `/publications/:id...` uses `publicationQueryOptions(id)`.
  - `/templates/:id` uses `templateQueryOptions(id)`.
  - `/leaves/:id` uses `leaveQueryOptions(id)`.
  React Query cache reuse prevents duplicate network work when the page already has the same query.
- Breadcrumbs express URL/information hierarchy, not browser history. For example `/leaves/:id` always renders under `Leaves` regardless of whether the user entered from dashboard or the leave list.
- Ancestor breadcrumb items are links; the current page item uses breadcrumb page semantics and is not clickable.
- Do not add `Dashboard` as a root crumb on every page. The homepage shows `Dashboard`; other pages start at their feature area, such as `Publications` or `Leaves`.

Rejected alternatives:

- Per-route breadcrumb exports were rejected because they would scatter shell navigation details across every route component without a current local convention.
- Reading only existing query cache was rejected because pages such as publication assignments do not load publication detail but still need a meaningful parent crumb.
- Keeping page-level back links was rejected because it duplicates breadcrumb behavior and adds visual noise.

## Risks / Trade-offs

- Dynamic breadcrumbs can briefly show fallback labels while detail data loads -> use stable fallbacks such as `Publication` and `Leave #id`.
- Breadcrumbs add a small number of extra detail queries on nested pages -> limit fetching to the current resource type and rely on React Query caching.
- jsdom tests cannot assert exact floating-sidebar visual shape -> assert shell structure, sidebar variant/collapse semantics, breadcrumb output, and removal of duplicate back links.
