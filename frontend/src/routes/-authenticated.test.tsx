import type { AnchorHTMLAttributes, ForwardedRef, ReactNode } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ToastProvider } from "@/components/ui/toast"
import { TooltipProvider } from "@/components/ui/tooltip"
import type { User } from "@/lib/types"

import { AuthenticatedLayout } from "./_authenticated"

const routerMocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  pathname: "/",
}))

type LinkMockProps = {
  to: string
  params?: Record<string, string>
  children?: ReactNode
} & AnchorHTMLAttributes<HTMLAnchorElement>

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    )
  const React = await vi.importActual<typeof import("react")>("react")
  const Link = React.forwardRef(function LinkMock(
    { to, params, children, ...props }: LinkMockProps,
    ref: ForwardedRef<HTMLAnchorElement>,
  ) {
    return React.createElement(
      "a",
      { href: hrefFor(to, params), ref, ...props },
      children,
    )
  })

  return {
    ...actual,
    Link,
    Outlet: () => React.createElement("div", null, "route content"),
    useNavigate: () => routerMocks.navigate,
    useRouterState: () => ({
      location: {
        pathname: routerMocks.pathname,
      },
    }),
  }
})

vi.mock("@/i18n", () => ({
  applyLanguagePreference: vi.fn(),
}))

function renderLayout() {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  })
  client.setQueryData(["auth", "me"], makeUser({ is_admin: true }))
  client.setQueryData(["me", "notifications", "unread-count"], 0)

  return render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <TooltipProvider>
          <AuthenticatedLayout />
        </TooltipProvider>
      </ToastProvider>
    </QueryClientProvider>,
  )
}

describe("AuthenticatedLayout", () => {
  beforeEach(() => {
    mockMatchMedia()
    routerMocks.navigate.mockReset()
    routerMocks.pathname = "/publications"
  })

  it("renders the main header trigger and breadcrumbs", async () => {
    const user = userEvent.setup()
    const { container } = renderLayout()

    const sidebar = container.querySelector("[data-slot='sidebar']")
    expect(sidebar).toHaveAttribute("data-variant", "floating")
    expect(sidebar).toHaveAttribute("data-state", "expanded")
    expect(screen.getByText("breadcrumbs.publications")).toBeInTheDocument()

    await user.click(
      screen.getByRole("button", { name: "sidebar.toggleNavigation" }),
    )

    expect(sidebar).toHaveAttribute("data-state", "collapsed")
    expect(sidebar).toHaveAttribute("data-collapsible", "icon")
  })
})

function makeUser(overrides: Partial<User> = {}): User {
  return {
    id: 1,
    email: "admin@example.com",
    name: "Admin User",
    is_admin: true,
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

function hrefFor(to: string, params?: Record<string, string>) {
  let href = to
  for (const [key, value] of Object.entries(params ?? {})) {
    href = href.replace(`$${key}`, value)
  }
  return href
}
