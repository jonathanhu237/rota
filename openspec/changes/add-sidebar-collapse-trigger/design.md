## Context

The authenticated layout already uses the shadcn `SidebarProvider`, `Sidebar`, `SidebarInset`, `SidebarTrigger`, and `SidebarRail` primitives. The mobile header renders a visible `SidebarTrigger`, but desktop users only get the rail hover target and the `cmd/ctrl+b` keyboard shortcut. Because the sidebar currently uses the default `offcanvas` collapsible mode, a desktop collapse can hide the whole sidebar rather than leaving a familiar icon rail.

## Goals / Non-Goals

**Goals:**

- Expose a visible desktop toggle for collapsing and expanding the sidebar.
- Use the existing shadcn sidebar primitives rather than custom state or a bespoke layout.
- Keep mobile behavior unchanged.

**Non-Goals:**

- Redesign navigation groups or labels.
- Add resizable widths, pinned state controls, or user settings for sidebar behavior.
- Change any backend/API behavior.

## Decisions

- Use `SidebarTrigger` for the visible control. This reuses the shadcn-provided toggle behavior, cookie persistence, mobile/desktop branching, and keyboard shortcut integration.
- Set the application sidebar to `collapsible="icon"` on desktop. This matches the shadcn documented icon-collapse pattern and leaves the navigation icons visible after collapse.
- Place the desktop trigger in the sidebar header next to the Rota brand when expanded, and keep it hidden on mobile because the authenticated layout already has a mobile header trigger.

Rejected alternatives:

- A custom `useSidebar()` button was rejected because `SidebarTrigger` already exists for this behavior.
- Leaving `offcanvas` mode was rejected because an in-sidebar trigger would disappear with the sidebar on desktop.
- Adding a second persistent desktop header was rejected because it would add layout chrome unrelated to the missing sidebar control.

## Risks / Trade-offs

- Collapsed header content may become cramped in icon mode -> hide text-only brand details through existing sidebar icon-collapse CSS while keeping icons and controls reachable.
- The visible trigger adds one more header control -> use an icon-only shadcn button with an accessible label to minimize visual weight.
- Tests in jsdom cannot fully validate CSS width transitions -> assert the rendered trigger and `collapsible="icon"` semantics through DOM state and interaction.
