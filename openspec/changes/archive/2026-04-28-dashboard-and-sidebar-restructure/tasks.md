## 1. Leave route consolidation

- [x] 1.1 Move `frontend/src/routes/_authenticated/leave.tsx` → `frontend/src/routes/_authenticated/leaves/new.tsx`. Update the `createFileRoute(...)` path string from `"/_authenticated/leave"` to `"/_authenticated/leaves/new"`. The file body (request flow, drafts, slot previews) is unchanged. Verify by `pnpm tsc --noEmit`.
- [x] 1.2 Move `frontend/src/routes/_authenticated/my-leaves.tsx` → `frontend/src/routes/_authenticated/leaves/index.tsx`. Update `createFileRoute(...)` path string from `"/_authenticated/my-leaves"` to `"/_authenticated/leaves/"`. Verify by `pnpm tsc --noEmit`.
- [x] 1.3 In the new `leaves/index.tsx`, add a primary "Request leave" CTA button at the top of the page (in the existing `<CardHeader>` block alongside title + description) that links to `/leaves/new`. Use the existing link-styled button pattern. Verify by component test in 7.2.
- [x] 1.4 Regenerate `frontend/src/routeTree.gen.ts` via TanStack Router CLI (`pnpm dev` running once will regenerate, or `pnpm tsr generate` if the project has that script). Confirm the generated file has routes for `/leaves`, `/leaves/new`, `/leaves/$leaveId` and no longer has `/leave` or `/my-leaves`. Verify by `pnpm tsc --noEmit && pnpm build`.
- [x] 1.5 Grep the codebase for `to="/leave"` and `to="/my-leaves"` and update every hit to `/leaves` or `/leaves/new` as appropriate. Also search for `'/leave'` and `'/my-leaves'` string literals in tests. Verify by `grep -rn '"\/leave"\|"\/my-leaves"\|to="/leave"\|to="/my-leaves"' frontend/src/` returning zero hits.

## 2. Sidebar grouping + rename

- [x] 2.1 Update `frontend/src/components/app-sidebar.tsx`: split the single `navItems` array into `employeeItems` and `adminItems` per design D-2. The sidebar's leaves entry now points at `/leaves` (not `/leave` or `/my-leaves`) with a single icon. Remove the `is_admin` push-into-flat-list logic. Verify by `pnpm tsc --noEmit`.
- [x] 2.2 Render two `<SidebarGroup>` blocks inside `<SidebarContent>` per design D-2. The "Manage" group is wrapped in `{user?.is_admin && (...)}`. Extract the shared `<SidebarMenuItem>`/`<SidebarMenuButton>` body into a small `renderItem(item)` helper inside the component. Verify by `pnpm tsc --noEmit`.
- [x] 2.3 Update `frontend/src/i18n/locales/{en,zh}.json`: remove `sidebar.leave` and `sidebar.myLeaves`; add `sidebar.leaves`, `sidebar.groups.mySchedule`, `sidebar.groups.manage`. Update tests that reference the removed keys. Verify by `pnpm lint && pnpm tsc --noEmit`.

## 3. Dashboard sub-components

- [x] 3.1 Create `frontend/src/components/dashboard/current-publication-card.tsx`. Props: `{ user: User | undefined }`. Internally calls `useQuery(currentPublicationQueryOptions)`. Renders the card per design D-1 with the `(state × is_admin)` CTA matrix. Show skeleton while loading. Verify by component test in 7.1.
- [x] 3.2 Create `frontend/src/components/dashboard/todo-card.tsx`. Internally calls `useQuery(unreadNotificationsQueryOptions)`. Returns `null` when count is 0 or undefined. Renders the card with a chevron link to `/requests` when count > 0. Verify by component test in 7.1.
- [x] 3.3 Create `frontend/src/components/dashboard/recent-leaves-card.tsx`. Internally calls `useQuery(myLeavesQueryOptions(1, 3))`. Visibility rule per design D-1: hide when leave list is empty AND no `ACTIVE` publication (latter inferred from the same publication query the parent uses; pass it as a prop or call again — either is fine). Verify by component test in 7.1.
- [x] 3.4 Create `frontend/src/components/dashboard/manage-shortcuts-card.tsx`. Pure presentational; renders 4 chip-style links. Parent decides whether to render based on `user.is_admin`. Verify by component test in 7.1.

## 4. Dashboard route assembly

- [x] 4.1 Rewrite `frontend/src/routes/_authenticated/index.tsx` per design D-1. Welcome header + `<div className="grid gap-6">` wrapping `<CurrentPublicationCard>`, a `grid grid-cols-1 lg:grid-cols-2 gap-6` row containing `<TodoCard>` + `<RecentLeavesCard>`, and `{user?.is_admin && <ManageShortcutsCard />}` at the bottom. Each card owns its own loading/empty state. Verify by `pnpm tsc --noEmit`.

## 5. i18n strings — dashboard

- [x] 5.1 Add to `frontend/src/i18n/locales/{en,zh}.json` per design D-4: all `dashboard.*` keys for the new cards (titles, CTAs in the matrix, empty states, pluralized "N pending shift changes"). Keep existing `dashboard.welcome` / `dashboard.description`. Verify by `pnpm lint && pnpm tsc --noEmit`.

## 6. i18n strings — leaves

- [x] 6.1 In `frontend/src/i18n/locales/{en,zh}.json`: rename the `myLeaves.*` block to `leaves.history.*`; add `leaves.requestCta`. Update all consumers (the renamed `leaves/index.tsx`, the new "Request leave" CTA, any tests). Verify by `pnpm lint && pnpm tsc --noEmit`.

## 7. Frontend tests

- [x] 7.1 Dashboard component tests (`frontend/src/routes/_authenticated/index.test.tsx` or `frontend/src/components/dashboard/*.test.tsx`):
  - `<CurrentPublicationCard>` renders correct CTA for each `(state, is_admin)` pair (use a small parametrized table — 6 states × 2 roles minus the "no CTA" cells).
  - `<CurrentPublicationCard>` empty state renders correctly for both roles.
  - `<TodoCard>` returns `null` at count 0; renders link at count > 0.
  - `<RecentLeavesCard>` hidden when empty + no ACTIVE; visible otherwise.
  - `<ManageShortcutsCard>` renders 4 chips.
  - Route file integration: admin sees all five cards; employee sees the four employee-relevant cards (no manage shortcuts).
- [x] 7.2 Leave-route tests:
  - `frontend/src/routes/_authenticated/leaves/-index.test.tsx` (renamed from `my-leaves.test.tsx`, prefixed so TanStack Router ignores it): renders history list; CTA button visible at top; CTA navigates to `/leaves/new`.
  - `frontend/src/routes/_authenticated/leaves/-new.test.tsx` (renamed from `leave.test.tsx`, prefixed so TanStack Router ignores it): existing assertions pass after the path move.
- [x] 7.3 Sidebar tests (`frontend/src/components/app-sidebar.test.tsx`): employee user → 1 group + 5 items; admin user → 2 groups + 9 items total; sidebar renders "请假" entry at `/leaves`, never `/leave` or `/my-leaves`.

## 8. Spec sync

- [x] 8.1 Confirm the change-folder spec delta at `openspec/changes/dashboard-and-sidebar-restructure/specs/frontend-shell/spec.md` matches the implemented behavior (new capability with 3 requirements). The new capability will be created as `openspec/specs/frontend-shell/spec.md` by `/opsx:archive` automatically.

## 9. Final gates

- [x] 9.1 `cd frontend && pnpm lint && pnpm test && pnpm build`. All clean.
- [x] 9.2 Manual smoke:
  - Log in as an employee → land on `/`. Verify: welcome + current-publication card + (conditionally) to-do/recent-leaves cards, NO manage shortcuts. Sidebar shows one "我的排班" group with 5 items.
  - Log in as admin → land on `/`. Verify: welcome + current-publication card with admin CTA + to-do (if any unread) + recent-leaves (if any) + manage-shortcuts (always). Sidebar shows two groups.
  - Click "请假" in sidebar → land on `/leaves` (history). Click "申请请假" → land on `/leaves/new`. Click a history row → land on `/leaves/:id`.
  - Try the old paths `/leave` and `/my-leaves` directly in the URL bar → should not resolve (404 from TanStack Router).
- [x] 9.3 `openspec validate dashboard-and-sidebar-restructure --strict`. Clean.
