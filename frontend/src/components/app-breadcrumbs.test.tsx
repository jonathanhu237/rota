import type { AnchorHTMLAttributes, ForwardedRef, ReactNode } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, screen } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"

import type { Leave, Publication, TemplateDetail } from "@/lib/types"

import { AppBreadcrumbs } from "./app-breadcrumbs"

const routerMocks = vi.hoisted(() => ({
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
    useRouterState: () => ({
      location: {
        pathname: routerMocks.pathname,
      },
    }),
  }
})

function renderBreadcrumbs(seed?: (client: QueryClient) => void) {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  })
  seed?.(client)

  return render(
    <QueryClientProvider client={client}>
      <AppBreadcrumbs />
    </QueryClientProvider>,
  )
}

describe("AppBreadcrumbs", () => {
  beforeEach(() => {
    routerMocks.pathname = "/"
  })

  it("renders a single current crumb for top-level authenticated routes", () => {
    routerMocks.pathname = "/roster"
    renderBreadcrumbs()

    expect(screen.getByText("breadcrumbs.roster")).toBeInTheDocument()
    expect(screen.queryByText("breadcrumbs.dashboard")).not.toBeInTheDocument()
  })

  it("renders the leave request hierarchy", () => {
    routerMocks.pathname = "/leaves/new"
    renderBreadcrumbs()

    expect(screen.getByText("breadcrumbs.leaves").closest("a")).toHaveAttribute(
      "href",
      "/leaves",
    )
    expect(screen.getByText("breadcrumbs.requestLeave")).toBeInTheDocument()
  })

  it("renders publication child routes with the loaded publication name", () => {
    routerMocks.pathname = "/publications/5/assignments"
    renderBreadcrumbs((client) => {
      client.setQueryData(["publications", "detail", 5], makePublication())
    })

    expect(
      screen.getByText("breadcrumbs.publications").closest("a"),
    ).toHaveAttribute("href", "/publications")
    expect(screen.getByText("Next Week Rota").closest("a")).toHaveAttribute(
      "href",
      "/publications/5",
    )
    expect(screen.getByText("breadcrumbs.assignments")).toBeInTheDocument()
  })

  it("uses dynamic template and leave labels when records are cached", () => {
    routerMocks.pathname = "/templates/3"
    const { rerender } = renderBreadcrumbs((client) => {
      client.setQueryData(["templates", "detail", 3], makeTemplate())
      client.setQueryData(["leaves", 15], makeLeave())
    })

    expect(screen.getByText("Default Rota").closest("a")).toBeNull()

    routerMocks.pathname = "/leaves/15"
    rerender(
      <QueryClientProvider client={newSeededClient()}>
        <AppBreadcrumbs />
      </QueryClientProvider>,
    )

    expect(screen.getByText("2026-04-25 · leave.category.sick")).toBeInTheDocument()
  })
})

function hrefFor(to: string, params?: Record<string, string>) {
  let href = to
  for (const [key, value] of Object.entries(params ?? {})) {
    href = href.replace(`$${key}`, value)
  }
  return href
}

function makePublication(): Publication {
  return {
    id: 5,
    template_id: 2,
    template_name: "Default Rota",
    name: "Next Week Rota",
    description: "",
    state: "ASSIGNING",
    submission_start_at: "2026-04-20T00:00:00Z",
    submission_end_at: "2026-04-21T00:00:00Z",
    planned_active_from: "2026-04-22T00:00:00Z",
    planned_active_until: "2026-04-29T00:00:00Z",
    activated_at: null,
    created_at: "2026-04-19T00:00:00Z",
    updated_at: "2026-04-19T00:00:00Z",
  }
}

function makeTemplate(): TemplateDetail {
  return {
    id: 3,
    name: "Default Rota",
    description: "",
    is_locked: false,
    shift_count: 0,
    slots: [],
    created_at: "2026-04-19T00:00:00Z",
    updated_at: "2026-04-19T00:00:00Z",
  }
}

function makeLeave(): Leave {
  return {
    id: 15,
    user_id: 1,
    publication_id: 5,
    shift_change_request_id: 42,
    category: "sick",
    reason: "",
    state: "pending",
    share_url: "http://example.test/leaves/15",
    created_at: "2026-04-19T00:00:00Z",
    updated_at: "2026-04-19T00:00:00Z",
    request: {
      id: 42,
      publication_id: 5,
      type: "give_pool",
      requester_user_id: 1,
      requester_assignment_id: 100,
      occurrence_date: "2026-04-25",
      counterpart_user_id: null,
      counterpart_assignment_id: null,
      counterpart_occurrence_date: null,
      state: "pending",
      leave_id: 15,
      decided_by_user_id: null,
      created_at: "2026-04-19T00:00:00Z",
      decided_at: null,
      expires_at: "2026-04-20T00:00:00Z",
    },
  }
}

function newSeededClient() {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  })
  client.setQueryData(["templates", "detail", 3], makeTemplate())
  client.setQueryData(["leaves", 15], makeLeave())
  return client
}
