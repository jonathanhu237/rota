import type { AnchorHTMLAttributes, ForwardedRef, ReactNode } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"

import type { Leave } from "@/lib/types"

import { LeavesHistoryPage } from "./index"

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

function makeLeave(): Leave {
  return {
    id: 15,
    user_id: 1,
    publication_id: 7,
    shift_change_request_id: 21,
    category: "sick",
    reason: "Illness",
    state: "pending",
    share_url: "http://example.test/leaves/15",
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
      leave_id: 15,
      decided_by_user_id: null,
      created_at: "2026-04-21T00:00:00Z",
      decided_at: null,
      expires_at: "2026-04-24T00:00:00Z",
    },
  }
}

function renderPage(leaves: Leave[]) {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  })
  client.setQueryData(["me", "leaves", 1, 10], leaves)

  return render(
    <QueryClientProvider client={client}>
      <LeavesHistoryPage />
    </QueryClientProvider>,
  )
}

describe("LeavesHistoryPage", () => {
  it("renders history and the request-leave CTA", () => {
    renderPage([makeLeave()])

    expect(screen.getByText("leaves.history.title")).toBeInTheDocument()
    expect(screen.getByText("2026-04-25 · leave.type.give_pool")).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "leaves.requestCta" })).toHaveAttribute(
      "href",
      "/leaves/new",
    )
    expect(screen.getByRole("link", { name: "leaves.history.open" })).toHaveAttribute(
      "href",
      "/leaves/15",
    )
  })
})
