import type { AnchorHTMLAttributes, ForwardedRef, ReactNode } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { cleanup, render, screen, within } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"

import { DashboardPage } from "@/routes/_authenticated/index"
import type { Leave, Publication, PublicationState, User } from "@/lib/types"

import { CurrentPublicationCard } from "./current-publication-card"
import { ManageShortcutsCard } from "./manage-shortcuts-card"
import { RecentLeavesCard } from "./recent-leaves-card"
import { TodoCard } from "./todo-card"

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
      { href: toHref(to, params), ref, ...props },
      children,
    )
  })

  return {
    ...actual,
    Link,
  }
})

function toHref(to: string, params?: Record<string, string>) {
  let href = to
  for (const [key, value] of Object.entries(params ?? {})) {
    href = href.replace(`$${key}`, value)
  }
  return href.replace(/\/$/, "") || "/"
}

function makeUser(overrides: Partial<User> = {}): User {
  return {
    id: 1,
    email: "worker@example.com",
    name: "Worker",
    is_admin: false,
    status: "active",
    version: 1,
    language_preference: null,
    theme_preference: "system",
    ...overrides,
  }
}

function makePublication(state: PublicationState): Publication {
  return {
    id: 7,
    template_id: 3,
    template_name: "Main Template",
    name: `${state} Publication`,
    description: "",
    state,
    submission_start_at: "2026-04-20T00:00:00Z",
    submission_end_at: "2026-04-21T00:00:00Z",
    planned_active_from: "2026-04-22T00:00:00Z",
    planned_active_until: "2026-04-29T00:00:00Z",
    activated_at: state === "ACTIVE" ? "2026-04-22T00:00:00Z" : null,
    created_at: "2026-04-19T00:00:00Z",
    updated_at: "2026-04-19T00:00:00Z",
  }
}

function makeLeave(): Leave {
  return {
    id: 11,
    user_id: 1,
    publication_id: 7,
    shift_change_request_id: 21,
    category: "personal",
    reason: "Appointment",
    state: "pending",
    share_url: "http://example.test/leaves/11",
    created_at: "2026-04-21T00:00:00Z",
    updated_at: "2026-04-21T00:00:00Z",
    request: {
      id: 21,
      publication_id: 7,
      type: "give_pool",
      requester_user_id: 1,
      requester_assignment_id: 31,
      occurrence_date: "2026-04-25",
      counterpart_user_id: null,
      counterpart_assignment_id: null,
      counterpart_occurrence_date: null,
      state: "pending",
      leave_id: 11,
      decided_by_user_id: null,
      created_at: "2026-04-21T00:00:00Z",
      decided_at: null,
      expires_at: "2026-04-24T00:00:00Z",
    },
  }
}

function renderWithQueryData(
  ui: ReactNode,
  {
    user = makeUser(),
    publication = makePublication("ACTIVE") as Publication | null,
    unreadCount = 0,
    leaves = [] as Leave[],
  } = {},
) {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  })
  client.setQueryData(["auth", "me"], user)
  client.setQueryData(["publications", "current"], publication)
  client.setQueryData(["me", "notifications", "unread-count"], unreadCount)
  client.setQueryData(["me", "leaves", 1, 3], leaves)

  return render(
    <QueryClientProvider client={client}>{ui}</QueryClientProvider>,
  )
}

describe("dashboard cards", () => {
  it.each([
    ["DRAFT", true, "dashboard.currentPublication.cta.draftAdmin", "/publications/7"],
    [
      "COLLECTING",
      false,
      "dashboard.currentPublication.cta.collectingEmployee",
      "/availability",
    ],
    [
      "COLLECTING",
      true,
      "dashboard.currentPublication.cta.collectingAdmin",
      "/publications/7",
    ],
    [
      "ASSIGNING",
      true,
      "dashboard.currentPublication.cta.assigningAdmin",
      "/publications/7/assignments",
    ],
    ["PUBLISHED", false, "dashboard.currentPublication.cta.published", "/roster"],
    ["ACTIVE", true, "dashboard.currentPublication.cta.published", "/roster"],
  ] as [PublicationState, boolean, string, string][])(
    "renders the %s current-publication CTA for admin=%s",
    (state, isAdmin, label, href) => {
      renderWithQueryData(<CurrentPublicationCard user={makeUser({ is_admin: isAdmin })} />, {
        user: makeUser({ is_admin: isAdmin }),
        publication: makePublication(state),
      })

      expect(screen.getByRole("link", { name: label })).toHaveAttribute(
        "href",
        href,
      )
    },
  )

  it("renders no-CTA current publication branches", () => {
    renderWithQueryData(<CurrentPublicationCard user={makeUser()} />, {
      publication: makePublication("ASSIGNING"),
    })

    expect(
      screen.getByText("dashboard.currentPublication.copy.awaiting"),
    ).toBeInTheDocument()
    expect(
      screen.queryByText("dashboard.currentPublication.cta.assigningAdmin"),
    ).not.toBeInTheDocument()
  })

  it("renders current-publication empty state for both roles", () => {
    renderWithQueryData(<CurrentPublicationCard user={makeUser()} />, {
      publication: null,
    })

    expect(screen.getByText("dashboard.currentPublication.empty")).toBeInTheDocument()
    expect(
      screen.queryByText("dashboard.currentPublication.cta.noneAdmin"),
    ).not.toBeInTheDocument()

    cleanup()

    renderWithQueryData(
      <CurrentPublicationCard user={makeUser({ is_admin: true })} />,
      {
        user: makeUser({ is_admin: true }),
        publication: null,
      },
    )

    expect(
      screen.getByRole("link", {
        name: "dashboard.currentPublication.cta.noneAdmin",
      }),
    ).toHaveAttribute("href", "/publications")
  })

  it("hides and shows the to-do card based on unread count", () => {
    const { container } = renderWithQueryData(<TodoCard />, {
      unreadCount: 0,
    })

    expect(container).toBeEmptyDOMElement()

    cleanup()

    renderWithQueryData(<TodoCard />, { unreadCount: 3 })

    expect(
      screen.getByText("dashboard.todo.unreadRequests"),
    ).toBeInTheDocument()
    expect(
      screen.getByRole("link", { name: "dashboard.todo.cta" }),
    ).toHaveAttribute("href", "/requests")
  })

  it("hides recent leaves when empty and no active publication", () => {
    const { container } = renderWithQueryData(<RecentLeavesCard />, {
      publication: makePublication("COLLECTING"),
      leaves: [],
    })

    expect(container).toBeEmptyDOMElement()
  })

  it("renders recent leaves when active or when leaves exist", () => {
    renderWithQueryData(<RecentLeavesCard />, {
      publication: makePublication("ACTIVE"),
      leaves: [],
    })

    expect(screen.getByText("dashboard.recentLeaves.empty")).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "leaves.requestCta" })).toHaveAttribute(
      "href",
      "/leaves/new",
    )

    cleanup()

    renderWithQueryData(<RecentLeavesCard />, {
      publication: makePublication("COLLECTING"),
      leaves: [makeLeave()],
    })

    expect(screen.getByText("2026-04-25 · leave.type.give_pool")).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "leaves.history.open" })).toHaveAttribute(
      "href",
      "/leaves/11",
    )
  })

  it("renders manage shortcuts", () => {
    renderWithQueryData(<ManageShortcutsCard />)

    const card = screen.getByText("dashboard.manage.title").closest("[data-slot='card']")
    expect(card).not.toBeNull()
    expect(within(card as HTMLElement).getAllByRole("link")).toHaveLength(4)
    expect(
      within(card as HTMLElement).getByRole("link", {
        name: "dashboard.manage.links.users",
      }),
    ).toHaveAttribute("href", "/users")
  })

  it("assembles the dashboard by role", () => {
    renderWithQueryData(<DashboardPage />, {
      user: makeUser({ name: "Employee", is_admin: false }),
      publication: makePublication("ACTIVE"),
      unreadCount: 2,
      leaves: [makeLeave()],
    })

    expect(screen.getByText("dashboard.welcome")).toBeInTheDocument()
    expect(screen.getByText("dashboard.currentPublication.title")).toBeInTheDocument()
    expect(screen.getByText("dashboard.todo.title")).toBeInTheDocument()
    expect(screen.getByText("dashboard.recentLeaves.title")).toBeInTheDocument()
    expect(screen.queryByText("dashboard.manage.title")).not.toBeInTheDocument()

    cleanup()

    renderWithQueryData(<DashboardPage />, {
      user: makeUser({ name: "Admin", is_admin: true }),
      publication: makePublication("ASSIGNING"),
      unreadCount: 2,
      leaves: [makeLeave()],
    })

    expect(screen.getByText("dashboard.manage.title")).toBeInTheDocument()
  })
})
