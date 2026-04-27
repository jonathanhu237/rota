## Context

After 18 changes the app's front door has drifted: the `/` landing page is empty, the sidebar is a 10-item flat list with employee + admin items mixed, and "Leave" lives at three URL spaces (`/leave` request, `/my-leaves` history, `/leaves/:id` detail) with two of them shown as siblings in the sidebar. A frontend audit (separate spawn-agent run, archived in the conversation) flagged these three as the highest-impact, lowest-cost UX problems. This change ships them together because they're the same "where am I" question.

No backend / schema / spec changes. Pure frontend reorganization.

## Goals / Non-Goals

**Goals:**

- Dashboard surfaces enough information that an authenticated user immediately sees "what's happening, what should I do next."
- Sidebar groups visually mirror the user's role: employee navigation in one group, admin navigation in another (only for admins).
- One feature → one URL prefix: leave consolidates under `/leaves/{,new,:id}`; sidebar exposes one entry.
- Every existing link / route consumer in the codebase is updated in the same commit; no dangling references.
- Loading states are per-card so a slow query never blanks the whole dashboard.

**Non-Goals:**

- Settings as a sidebar item (stays in avatar dropdown per the `user-settings-page` decision).
- Mobile layout adjustments. Desktop-first.
- Aggregations across multiple publications.
- Admin-fork of the dashboard.
- Color / form / confirm-dialog cleanup (separate changes).
- Spec capability changes (no requirements cover sidebar / dashboard layout).
- Role-based "first-run" tutorials, empty-state illustrations, or onboarding affordances beyond the basic "no current publication" copy.

## Decisions

### D-1. Dashboard layout — five cards in a single grid

```
┌─ 欢迎 X ────────────────────────────────────┐
│  description                                  │
└───────────────────────────────────────────────┘

┌─ 当前发布 ───────────────────────────────────┐
│  Realistic Rota Week         [ASSIGNING]      │
│  生效窗口 2026-04-21 — 2026-04-27             │
│                          [打开排班板 →]        │
└───────────────────────────────────────────────┘

┌─ 待办 ─────────────┐  ┌─ 最近请假 ───────────┐
│  您有 3 条调班待处理  │  │  · 2026-04-25 ...   │
│              [查看 →] │  │  · 2026-04-20 ...   │
└─────────────────────┘  │            [查看全部] │
                         └──────────────────────┘

(admin only)
┌─ 管理快捷入口 ──────────────────────────────┐
│  [用户]  [岗位]  [模板]  [发布]              │
└───────────────────────────────────────────────┘
```

All five cards are children of one `<div className="grid gap-6">` (matches `availability.tsx` / `leave.tsx`). The 待办 + 最近请假 row uses `grid grid-cols-1 lg:grid-cols-2 gap-6` so they stack on narrow viewports.

**Card structure:**

- `welcome` — bare `<div>`, no `<Card>` wrapper (matches today's heading).
- `current-publication` — wrapped in `<Card>`. Body shows publication name, `<PublicationStateBadge>` (existing component), `effective` window dates via `Intl.DateTimeFormat`, and a single CTA `<Button>` whose label and `to=` depend on the (state × role) matrix below.
- `todo` — only rendered if `unreadNotificationsQueryOptions` returns count > 0. Single text line + chevron link.
- `recent-leaves` — only rendered if `myLeavesQueryOptions(1, 3)` returns at least one row, OR the current publication is in `ACTIVE` (so leave is permitted). When both conditions are false, omit the card entirely.
- `manage-shortcuts` — only rendered if `user.is_admin`. Four chips in a `flex flex-wrap gap-2` block.

CTA matrix for the **current publication** card:

| Effective state | Employee CTA | Admin CTA |
|---|---|---|
| `DRAFT` (stored) | (no CTA — show "排班还在筹备中") | "查看发布详情 →" `/publications/:id` |
| `COLLECTING` | "提交可用性 →" `/availability` | "查看可用性提交进度 →" `/publications/:id` |
| `ASSIGNING` | (no CTA — "等待排班") | "打开分配面板 →" `/publications/:id/assignments` |
| `PUBLISHED` | "查看排班表 →" `/roster` | "查看排班表 →" `/roster` |
| `ACTIVE` | "查看排班表 →" `/roster` | "查看排班表 →" `/roster` |
| no publication | (card body says "暂无活跃发布") | "新建发布 →" `/publications` |

The matrix is encoded as a small switch in `dashboard-current-publication-card.tsx` (a new sub-component owning this branching logic so the route file stays thin).

**Rejected — single big CTA strip below the welcome.** Mixes navigation with state-dependent guidance; the per-card layout is more honest about "what do I do next."

**Rejected — show stored state vs effective state separately.** Effective state is what users care about; stored state is implementation detail.

### D-2. Sidebar grouping

`components/app-sidebar.tsx` `navItems` array becomes two arrays:

```ts
const employeeItems: NavItem[] = [
  { title: t("sidebar.dashboard"),    url: "/",            icon: Home },
  { title: t("sidebar.roster"),       url: "/roster",      icon: CalendarRange },
  { title: t("sidebar.availability"), url: "/availability", icon: CalendarCheck },
  { title: t("sidebar.requests"),     url: "/requests",    icon: Inbox, badge: ... },
  { title: t("sidebar.leaves"),       url: "/leaves",      icon: CalendarX },
]

const adminItems: NavItem[] = [
  { title: t("sidebar.users"),        url: "/users",       icon: Users },
  { title: t("sidebar.positions"),    url: "/positions",   icon: Briefcase },
  { title: t("sidebar.templates"),    url: "/templates",   icon: CalendarDays },
  { title: t("sidebar.publications"), url: "/publications", icon: FileText },
]
```

Render two `<SidebarGroup>` blocks. The admin group is rendered inside `if (user?.is_admin)` so non-admins never see an empty "管理" header.

```tsx
<SidebarContent>
  <SidebarGroup>
    <SidebarGroupLabel>{t("sidebar.groups.mySchedule")}</SidebarGroupLabel>
    <SidebarGroupContent>
      <SidebarMenu>{employeeItems.map(renderItem)}</SidebarMenu>
    </SidebarGroupContent>
  </SidebarGroup>

  {user?.is_admin && (
    <SidebarGroup>
      <SidebarGroupLabel>{t("sidebar.groups.manage")}</SidebarGroupLabel>
      <SidebarGroupContent>
        <SidebarMenu>{adminItems.map(renderItem)}</SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  )}
</SidebarContent>
```

`renderItem` is a small inline helper — pulling the existing `<SidebarMenuItem>` / `<SidebarMenuButton>` body into a function so both groups use the same render. No new component.

The single `sidebar.navigation` i18n key is replaced by two: `sidebar.groups.mySchedule` and `sidebar.groups.manage`.

**Rejected — three groups including a "Settings" group with the avatar dropdown promoted up.** Settings is intentionally a one-touch dropdown; promoting it adds a sidebar row that 95% of users use ~3× per year. Stays in the dropdown.

### D-3. Leave URL consolidation

File moves:

| Before | After | What it does |
|---|---|---|
| `routes/_authenticated/leave.tsx` | `routes/_authenticated/leaves/new.tsx` | Request flow (multi-row draft, slot previews) |
| `routes/_authenticated/my-leaves.tsx` | `routes/_authenticated/leaves/index.tsx` | History list, paginated |
| `routes/_authenticated/leaves/$leaveId.tsx` | (unchanged path) | Detail |

After move, the history page `/leaves` adds a top-of-page CTA:

```tsx
<Card>
  <CardHeader className="flex flex-row items-start justify-between gap-4">
    <div>
      <CardTitle>{t("leaves.history.title")}</CardTitle>
      <CardDescription>{t("leaves.history.description")}</CardDescription>
    </div>
    <Button asChild>
      <Link to="/leaves/new">
        <Plus className="size-4" />
        {t("leaves.requestCta")}
      </Link>
    </Button>
  </CardHeader>
  <CardContent>{/* paginated list */}</CardContent>
</Card>
```

The `/leaves/new` page itself loses its standalone "navigation" feel — it's a sub-route now, gets a "返回" link in its header that goes back to `/leaves`. (Or simply drop the back link and let the browser back button handle it; TBD during apply, low-risk either way.)

**Internal references to update:**

```bash
grep -rn 'to="/leave"\|to="/my-leaves"\|"\/leave"\|"\/my-leaves"' frontend/src/
```

The expected hits:
- `app-sidebar.tsx` — sidebar items (handled by D-2).
- Any "view all" link from another page (likely none, but verify).
- Tests that hit those routes.
- i18n keys / route components themselves.

Each updated to `/leaves` or `/leaves/new` as appropriate.

**Rejected — keep `/leave` (request) at the top level and only move history.** The whole point is the namespace consolidation; cherry-picking only history defeats the goal.

**Rejected — make `/leaves/new` a dialog.** The leave-request flow has multi-row drafts and slot previews; it's a real page, not a 3-field dialog.

### D-4. i18n string changes

Renamed:
- `sidebar.leave` → removed.
- `sidebar.myLeaves` → renamed to `sidebar.leaves` (single sidebar entry).

Added (en + zh):
- `sidebar.groups.mySchedule` ("My schedule" / "我的排班")
- `sidebar.groups.manage` ("Manage" / "管理")
- `dashboard.welcome` (existing — kept; the value already takes `{name}`)
- `dashboard.description` (existing — kept)
- `dashboard.currentPublication.title` ("Current publication" / "当前发布")
- `dashboard.currentPublication.empty` ("No active publication" / "暂无活跃发布")
- `dashboard.currentPublication.cta.{collecting_employee, collecting_admin, assigning_admin, published, draft_admin, none_admin}` — labels per the matrix in D-1.
- `dashboard.todo.title` ("To-do" / "待办")
- `dashboard.todo.unreadRequests` ("You have {count} pending shift changes" / "您有 {count} 条调班待处理")
- `dashboard.recentLeaves.title` ("Recent leaves" / "最近请假")
- `dashboard.recentLeaves.viewAll` ("View all" / "查看全部")
- `dashboard.manage.title` ("Quick links" / "管理快捷入口")
- `leaves.history.title`, `leaves.history.description` (replaces the existing `myLeaves.title`/`description` keys; in line with the file rename).
- `leaves.requestCta` ("Request leave" / "申请请假")

Removed:
- `myLeaves.*` keys (replaced by `leaves.history.*`).

### D-5. Component shape

Files affected:

```
frontend/src/routes/_authenticated/index.tsx          (~150 lines after rewrite)
frontend/src/components/dashboard/                    (new dir)
  current-publication-card.tsx                        (~80 lines, owns the CTA matrix)
  todo-card.tsx                                       (~40 lines)
  recent-leaves-card.tsx                              (~60 lines)
  manage-shortcuts-card.tsx                           (~30 lines)
frontend/src/components/app-sidebar.tsx               (refactored, ~250 → ~250 lines net)
frontend/src/routes/_authenticated/leaves/index.tsx   (was my-leaves.tsx, +CTA in header)
frontend/src/routes/_authenticated/leaves/new.tsx     (was leave.tsx, +back link in header)
frontend/src/routes/_authenticated/leaves/$leaveId.tsx (unchanged)
frontend/src/routes/_authenticated/leave.tsx          DELETED
frontend/src/routes/_authenticated/my-leaves.tsx      DELETED
frontend/src/routeTree.gen.ts                         regenerated by TanStack Router
```

The dashboard sub-components (`dashboard/`) follow the existing per-feature directory pattern (`components/settings/`, `components/assignments/`, etc.). The route file stays thin — it just composes them in a grid.

### D-6. Tests

- `dashboard.test.tsx`:
  - Renders the welcome header with the user's name.
  - When `currentPublication` query returns null, the current-publication card shows the empty CTA (admin sees "新建发布", employee sees no CTA).
  - The CTA matrix: with mock current publications in each effective state × role combination, renders the correct CTA label and target.
  - 待办 card hidden when unread count is 0; visible with the right text when > 0.
  - 最近请假 card hidden when leaves list is empty AND no active publication.
  - 管理快捷入口 card visible only for admins.
- `app-sidebar.test.tsx` (existing, expand):
  - Employee user sees "我的排班" group with 5 items, no "管理" group.
  - Admin user sees both groups; "管理" has 4 items.
  - Renamed sidebar item is "请假" pointing at `/leaves` (not `/leave` or `/my-leaves`).
- `leaves/index.test.tsx` (was `my-leaves.test.tsx`):
  - Renders history list with the new CTA button in the card header.
  - CTA navigates to `/leaves/new`.
- `leaves/new.test.tsx` (was `leave.test.tsx`):
  - Existing tests pass after the file move; only the route path assertion updates.

## Risks / Trade-offs

- **Risk:** the dashboard's CTA matrix is a 6-state × 2-role table — easy to typo a key or miss a combination. → Mitigation: encode it as a TypeScript-typed map keyed by `[effectiveState, isAdmin]`; exhaustiveness check via `never` in the default branch ensures every state is handled.
- **Risk:** the sidebar test count grows because the rendering is now state-dependent. → Acceptable: ~5 new tests, all small.
- **Risk:** TanStack Router file-based routing requires the move to happen in one commit (otherwise the routeTree.gen.ts and the actual files disagree). → Mitigation: rename + regenerate as part of the same commit; CI's `pnpm build` catches drift.
- **Risk:** the dashboard does 4 queries on first render. → Mitigation: each query is cached by TanStack Query; per-card loading states avoid blocking. Total payload is small (current publication, unread count, 3 recent leaves, current user).
- **Trade-off:** dropping `/leave` and `/my-leaves` URL paths means any external bookmark breaks. → Acceptable: pre-prod, no production users.

## Migration Plan

Single shipping unit. Frontend-only:

1. Rename leave routes; regenerate `routeTree.gen.ts`.
2. Update sidebar to grouped layout + renamed item.
3. Build out dashboard cards.
4. Update i18n strings.
5. Run `pnpm tsc --noEmit && pnpm lint && pnpm test && pnpm build`.
6. Manual smoke: log in as employee → see one sidebar group + dashboard with 当前发布 card + history-only leaves; log in as admin → see two sidebar groups + 管理快捷入口 card + admin CTAs.

Rollback = revert the change. No backend / schema entanglement.

## Open Questions

None.
