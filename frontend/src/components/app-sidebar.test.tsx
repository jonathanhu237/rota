import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import type { AnchorHTMLAttributes, ForwardedRef, ReactNode } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { SidebarProvider } from "@/components/ui/sidebar"
import { ToastProvider } from "@/components/ui/toast"
import { TooltipProvider } from "@/components/ui/tooltip"
import type { User } from "@/lib/types"

import { AppSidebar } from "./app-sidebar"

const routerMocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  pathname: "/",
}))
const themeMocks = vi.hoisted(() => ({
  toggleThemePreference: vi.fn(() => "dark"),
}))

type LinkMockProps = {
  to: string
  children?: ReactNode
} & AnchorHTMLAttributes<HTMLAnchorElement>

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    )
  const React = await vi.importActual<typeof import("react")>("react")
  const Link = React.forwardRef(function LinkMock(
    { to, children, ...props }: LinkMockProps,
    ref: ForwardedRef<HTMLAnchorElement>,
  ) {
    return React.createElement("a", { href: to, ref, ...props }, children)
  })

  return {
    ...actual,
    Link,
    useNavigate: () => routerMocks.navigate,
    useRouterState: () => ({
      location: {
        pathname: routerMocks.pathname,
      },
    }),
  }
})

vi.mock("@/components/theme-context", () => ({
  useTheme: () => ({
    toggleThemePreference: themeMocks.toggleThemePreference,
  }),
}))

function makeUser(overrides: Partial<User> = {}): User {
  return {
    id: 1,
    email: "alice@example.com",
    name: "Alice Example",
    is_admin: false,
    status: "active",
    version: 1,
    language_preference: null,
    theme_preference: "system",
    ...overrides,
  }
}

function mockMatchMedia() {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  })
}

function renderAppSidebar(user: User = makeUser()) {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  })
  client.setQueryData(["auth", "me"], user)
  client.setQueryData(["me", "notifications", "unread-count"], 0)

  return render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <TooltipProvider>
          <SidebarProvider>
            <AppSidebar />
          </SidebarProvider>
        </TooltipProvider>
      </ToastProvider>
    </QueryClientProvider>,
  )
}

describe("AppSidebar", () => {
  beforeEach(() => {
    mockMatchMedia()
    routerMocks.navigate.mockReset()
    themeMocks.toggleThemePreference.mockClear()
  })

  it("renders avatar dropdown items and navigates to settings", async () => {
    const user = userEvent.setup()
    renderAppSidebar()

    await user.click(screen.getByText("Alice Example"))

    expect(await screen.findByText("sidebar.toggleTheme")).toBeInTheDocument()
    expect(screen.getByText("sidebar.settings")).toBeInTheDocument()
    expect(screen.getByText("sidebar.logout")).toBeInTheDocument()

    await user.click(screen.getByText("sidebar.settings"))

    expect(routerMocks.navigate).toHaveBeenCalledWith({ to: "/settings" })
  })

  it("renders employee schedule navigation without admin management links", () => {
    renderAppSidebar()

    const myScheduleGroup = screen
      .getByText("sidebar.groups.mySchedule")
      .closest("[data-slot='sidebar-group']")
    expect(myScheduleGroup).not.toBeNull()

    const navItems = [
      ["sidebar.dashboard", "/"],
      ["sidebar.roster", "/roster"],
      ["sidebar.availability", "/availability"],
      ["sidebar.requests", "/requests"],
      ["sidebar.leaves", "/leaves"],
    ] as const

    for (const [label, href] of navItems) {
      expect(
        within(myScheduleGroup as HTMLElement).getByText(label).closest("a"),
      ).toHaveAttribute("href", href)
    }

    expect(screen.queryByText("sidebar.groups.manage")).not.toBeInTheDocument()
    expect(screen.queryByText("sidebar.users")).not.toBeInTheDocument()
    expect(screen.queryByText("sidebar.publications")).not.toBeInTheDocument()
  })

  it("renders admin management navigation as a separate group", () => {
    renderAppSidebar(makeUser({ is_admin: true }))

    const manageGroup = screen
      .getByText("sidebar.groups.manage")
      .closest("[data-slot='sidebar-group']")
    expect(manageGroup).not.toBeNull()

    const adminItems = [
      ["sidebar.users", "/users"],
      ["sidebar.positions", "/positions"],
      ["sidebar.templates", "/templates"],
      ["sidebar.publications", "/publications"],
    ] as const

    for (const [label, href] of adminItems) {
      expect(
        within(manageGroup as HTMLElement).getByText(label).closest("a"),
      ).toHaveAttribute("href", href)
    }
  })

  it("uses the consolidated leaves entry point", () => {
    const { container } = renderAppSidebar()
    const legacyRequestPath = "/lea" + "ve"
    const legacyHistoryPath = "/my-" + "leaves"

    expect(screen.getByText("sidebar.leaves").closest("a")).toHaveAttribute(
      "href",
      "/leaves",
    )
    expect(
      container.querySelector(`a[href="${legacyRequestPath}"]`),
    ).not.toBeInTheDocument()
    expect(
      container.querySelector(`a[href="${legacyHistoryPath}"]`),
    ).not.toBeInTheDocument()
  })
})
