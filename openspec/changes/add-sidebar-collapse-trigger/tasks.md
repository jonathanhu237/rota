## 1. Shell Layout

- [x] 1.1 Configure the authenticated sidebar as a floating icon-collapse sidebar and move the visible `SidebarTrigger` to the main content header; verify with `cd frontend && pnpm test src/components/app-sidebar.test.tsx src/routes/-authenticated.test.tsx`.
- [x] 1.2 Add the shadcn breadcrumb primitive and implement a centralized `AppBreadcrumbs` component for every authenticated route; verify with `cd frontend && pnpm test src/components/app-breadcrumbs.test.tsx`.

## 2. Breadcrumb Behavior

- [x] 2.1 Implement static breadcrumbs for all top-level authenticated routes; verify with `cd frontend && pnpm test src/components/app-breadcrumbs.test.tsx`.
- [x] 2.2 Implement nested breadcrumbs with dynamic publication, template, and leave labels plus stable fallbacks; verify with `cd frontend && pnpm test src/components/app-breadcrumbs.test.tsx`.
- [x] 2.3 Remove duplicate page-level parent back links now covered by breadcrumbs; verify with `cd frontend && pnpm test src/routes/_authenticated/leaves/-new.test.tsx && rg "backToHistory|backToPublication" frontend/src/routes/_authenticated -g '!*.test.tsx'`.

## 3. Validation

- [x] 3.1 Run frontend checks for the shell change; verify with `cd frontend && pnpm lint && pnpm test && pnpm build`.
- [x] 3.2 Validate OpenSpec artifacts; verify with `openspec validate add-sidebar-collapse-trigger --strict`.
