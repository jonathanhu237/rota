## Why

The frontend has accreted enough rooms that the front door has become disorienting. Specifically:

1. **The dashboard is empty.** [routes/_authenticated/index.tsx](frontend/src/routes/_authenticated/index.tsx) renders a single `<h1>` welcome and a one-line description — that's it. Every authenticated session lands on a near-blank page.
2. **The sidebar is a flat list of 10 items.** Employee actions (Roster, Availability, Requests, Leave, My leaves) and admin actions (Users, Positions, Templates, Publications) are visually identical and adjacent. `Leave` (the request form) and `My leaves` (the history list) are next to each other, doubly confusable because both are noun-shaped in English and both start with the same icon-language.
3. **"Leave" is split across three URL spaces.** `/leave` is the request flow; `/my-leaves` is the history list; `/leaves/:id` is the detail. The sidebar exposes the first two as siblings; the third is reachable only by clicking into history. Three URL spaces for one feature.

These three problems compound: the dashboard's emptiness sends users hunting through the sidebar, but the sidebar's flatness offers no orientation, and `Leave` vs `My leaves` is the worst-named pair in that flat list. Fixing all three together is the smallest believable improvement to "where am I, and where do I go next?"

## What Changes

### Dashboard becomes a real landing page

`routes/_authenticated/index.tsx` is rewritten to surface, in this order:

1. **Welcome header** — current `<h1>` and description retained.
2. **当前发布 (Current publication) card** — when one exists in non-`ENDED` state, show its name, effective-state badge, planned-active window dates, and a primary CTA that depends on the effective state and the viewer's role:
   - State `COLLECTING` → link to `/availability` for employees, link to `/publications/:id` for admins.
   - State `ASSIGNING` → link to `/publications/:id/assignments` for admins; for employees, show "可用性收集已结束，等待排班" without a CTA.
   - State `PUBLISHED` or `ACTIVE` → link to `/roster` for everyone.
   - Stored `DRAFT` (admin only) → link to `/publications/:id`.
3. **待办 (To-do) card** — surfaces the existing `unreadNotificationsQueryOptions` count as a single chip "您有 N 条调班待处理 →" linking to `/requests`. Hidden when count is 0.
4. **最近请假 (Recent leaves) card** — top 3 rows from `myLeavesQueryOptions(1, 3)` with state badges; "查看全部" link to `/leaves` (the renamed history page). Hidden when the user has zero leaves and no current publication is in a state where leave is permitted.
5. **(Admin-only) 管理快捷入口 (Manage shortcuts) card** — four chips linking to `/users`, `/positions`, `/templates`, `/publications`. Mirrors the future sidebar "Manage" group but at glance-distance on the home page.

All four content cards reuse the existing `<Card>` / `<CardHeader>` / `<CardContent>` shadcn primitives — no new component types. The page is a `grid gap-6` of cards, identical to the layout already used by `/availability`, `/leave`, and `/users`.

### Sidebar is grouped and renamed

`components/app-sidebar.tsx` splits the single "Navigation" group into two (or three) labelled groups:

- **我的排班 (My schedule)** — Dashboard, Roster, Availability, Requests, Leaves.
- **管理 (Manage)** — Users, Positions, Templates, Publications. Rendered only when `user.is_admin`.

The header (Rota R logo) and the avatar dropdown footer are untouched — they're already clean per the audit.

### Leave URLs collapse to `/leaves`

The request flow at `/leave` and the history at `/my-leaves` become children of one feature root:

- **`/leaves`** (was `/my-leaves`) — history list. Top-of-page CTA "申请请假 / Request leave" links to `/leaves/new`.
- **`/leaves/new`** (was `/leave`) — request flow. Multi-row draft, slot previews, etc., unchanged in behavior; only the URL moves.
- **`/leaves/:id`** — detail, unchanged.

Sidebar exposes a single "请假 / Leave" item that lands on `/leaves`. The detail route stays unlinked from the sidebar (correctly — it's reached by clicking a history row).

Backward compatibility: this app has no production users yet; the URL rename does not need redirects from the old paths. Internal links in the codebase are updated in the same commit.

### What's NOT changing

- `Settings` lives in the avatar dropdown (it was put there deliberately by `user-settings-page`); not promoted to a sidebar item.
- The header section of the sidebar (logo + app description).
- The avatar dropdown menu items (theme toggle, settings, logout).
- Any non-leave route paths.
- Any backend code, schema, or migration.
- Color / variant cleanup (queued as `color-token-cleanup`).
- Form / dialog consolidation (queued as `form-and-confirm-consistency`).

## Capabilities

### New Capabilities

- `frontend-shell`: a thin spec capturing the high-level frontend shell invariants (dashboard widget contract, sidebar grouping, leave route namespace) so future redesigns have a place to record changes and don't drift silently. Three requirements:
  - *Authenticated landing page widgets* — the dashboard's five-card layout and CTA matrix.
  - *Sidebar navigation grouping* — the two-group "My schedule" / "Manage" structure with admin gating.
  - *Leave route namespace* — `/leaves`, `/leaves/new`, `/leaves/:id` are the only leave routes.

### Modified Capabilities

None.

## Impact

- **Frontend code:**
  - `frontend/src/routes/_authenticated/index.tsx` — rewrite from stub to real landing page (~150 lines).
  - `frontend/src/components/app-sidebar.tsx` — split nav array into grouped data + render two `<SidebarGroup>` blocks; rename `Leave`/`My leaves` to single `Leaves` entry pointing at `/leaves`.
  - `frontend/src/routes/_authenticated/leave.tsx` → `frontend/src/routes/_authenticated/leaves/new.tsx` (file rename + route path change).
  - `frontend/src/routes/_authenticated/my-leaves.tsx` → `frontend/src/routes/_authenticated/leaves/index.tsx` (file rename + route path change). Add the "申请请假" CTA at the top of the page.
  - `frontend/src/routes/_authenticated/leaves/$leaveId.tsx` — already correctly placed; only confirm internal links.
  - `frontend/src/routeTree.gen.ts` — regenerated by TanStack Router CLI.
  - i18n strings in `frontend/src/i18n/locales/{en,zh}.json`: rename `sidebar.leave` / `sidebar.myLeaves` → `sidebar.leaves`; add new keys for sidebar group labels (`sidebar.groups.mySchedule`, `sidebar.groups.manage`); add dashboard widget strings.
  - Search every existing `to="/leave"` / `to="/my-leaves"` usage in the codebase and update.
  - Tests: dashboard widget rendering (per role + per publication state), sidebar grouping (employee sees one group, admin sees two), leaves-routes routing.
- **No backend code changes.**
- **No schema / migration changes.**
- **Spec:** new `frontend-shell` capability with three requirements documenting the shell invariants this change introduces.

## Non-goals

- **Settings as a sidebar item.** Stays in the avatar dropdown.
- **Per-publication shortcuts on the dashboard beyond the current publication card.** Multi-publication navigation lives in `/publications`.
- **Dashboard widgets that aggregate across all publications.** Single-publication focus matches the project's invariant of at most one non-`ENDED` publication.
- **Admin-tailored dashboards.** Admins see the same five cards plus the management-shortcuts card; we don't fork the page.
- **Mobile / narrow-viewport layout for the dashboard.** Desktop-first; the cards stack naturally on narrow screens via the grid.
- **Replacing card-based layout with a different visual idiom.** The audit confirmed cards are the one consistency the app already has.
- **Color, form, or confirm-dialog cleanup.** Those are queued as separate changes.

## Risks / safeguards

- **Risk:** changing the route paths for leave breaks any in-flight links the user might have bookmarked. → Mitigation: pre-prod, no users to surprise. Internal codebase links updated in the same commit.
- **Risk:** the dashboard now does five queries on every load (`/auth/me`, current publication, unread count, my-leaves head, plus existing role check). → Mitigation: TanStack Query cache makes repeat visits cheap; each query is small. Loading states for each card are independent so the page never blocks on a slow query.
- **Risk:** sidebar group split shows employees an empty "Manage" group if the gating is done wrong. → Mitigation: render the `<SidebarGroup>` for admin only inside a single `if (user?.is_admin)` block (no empty group ever rendered for non-admins).
- **Risk:** the dashboard's "current publication" card duplicates info already on `/publications` for admins. → Acceptable: it's a glance-and-go shortcut, not authoritative; the difference is dashboard surfaces only the current one and links into the right sub-page.
