import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, screen } from "@testing-library/react"
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

function makeUser(): User {
  return {
    id: 1,
    email: "alice@example.com",
    name: "Alice Example",
    is_admin: false,
    status: "active",
    version: 1,
    language_preference: null,
    theme_preference: "system",
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

function renderAppSidebar() {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  })
  client.setQueryData(["auth", "me"], makeUser())
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
})
