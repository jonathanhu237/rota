import type { AnchorHTMLAttributes, ForwardedRef, ReactNode } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"

import type { Publication } from "@/lib/types"
import { ToastProvider } from "@/components/ui/toast"

import { LeavePage } from "./new"

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
  }
})

function makePublication(): Publication {
  return {
    id: 7,
    template_id: 3,
    template_name: "Main Template",
    name: "Active Publication",
    description: "",
    state: "ACTIVE",
    submission_start_at: "2026-04-20T00:00:00Z",
    submission_end_at: "2026-04-21T00:00:00Z",
    planned_active_from: "2026-04-22T00:00:00Z",
    planned_active_until: "2026-04-29T00:00:00Z",
    activated_at: "2026-04-22T00:00:00Z",
    created_at: "2026-04-19T00:00:00Z",
    updated_at: "2026-04-19T00:00:00Z",
  }
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  })
  client.setQueryData(["publications", "current"], makePublication())
  client.setQueryData(["publications", 7, "members"], [])
  const today = new Date().toISOString().slice(0, 10)
  client.setQueryData(["me", "leaves", "preview", today, addDays(today, 14)], [])

  return render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <LeavePage />
      </ToastProvider>
    </QueryClientProvider>,
  )
}

describe("LeavePage", () => {
  it("renders the moved leave request page with a back link", async () => {
    renderPage()

    expect(screen.getByText("leave.title")).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "leaves.backToHistory" })).toHaveAttribute(
      "href",
      "/leaves",
    )
  })
})

function addDays(dateValue: string, days: number) {
  const date = new Date(`${dateValue}T00:00:00Z`)
  date.setUTCDate(date.getUTCDate() + days)
  return date.toISOString().slice(0, 10)
}
