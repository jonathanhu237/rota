import type { AnchorHTMLAttributes, ForwardedRef, ReactNode } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ToastProvider } from "@/components/ui/toast"
import type { Leave } from "@/lib/types"

import { LeavesWorkbenchPage } from "./index"

const { getMock, postMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  postMock: vi.fn(),
}))

vi.mock("@/lib/axios", () => ({
  default: {
    get: getMock,
    post: postMock,
  },
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

type LeavePatch = Omit<Partial<Leave>, "request"> & {
  request?: Partial<Leave["request"]>
}

function makeLeave(patch: LeavePatch = {}): Leave {
  const base: Leave = {
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
    requester_name: "Alice",
    shift: {
      assignment_id: 31,
      slot_id: 21,
      weekday: 6,
      start_time: "09:00",
      end_time: "12:00",
      position_id: 101,
      position_name: "Front Desk",
      occurrence_start: "2026-04-25T09:00:00Z",
      occurrence_end: "2026-04-25T12:00:00Z",
    },
    urgency: {
      occurrence_start: "2026-04-25T09:00:00Z",
      seconds_until_start: 7200,
      starts_within_24_hours: true,
    },
    actions: {
      can_claim: true,
      can_approve: false,
      can_reject: false,
      can_cancel: false,
    },
  }
  return {
    ...base,
    ...patch,
    request: { ...base.request, ...patch.request },
    actions: patch.actions ?? base.actions,
    shift: patch.shift === undefined ? base.shift : patch.shift,
    urgency: patch.urgency === undefined ? base.urgency : patch.urgency,
  }
}

function renderPage(poolData: {
  pending?: { leaves: Leave[]; page?: number; total_count?: number }
  pendingPage2?: { leaves: Leave[]; page?: number; total_count?: number }
  completed?: { leaves: Leave[]; page?: number; total_count?: number }
  all?: { leaves: Leave[]; page?: number; total_count?: number }
} = {}) {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  })
  const pending = poolData.pending ?? { leaves: [makeLeave()] }
  client.setQueryData(["leaves", "pool", "pending", 1, 20], {
    leaves: pending.leaves,
    page: pending.page ?? 1,
    page_size: 20,
    total_count: pending.total_count ?? pending.leaves.length,
  })
  if (poolData.pendingPage2) {
    client.setQueryData(["leaves", "pool", "pending", 2, 20], {
      leaves: poolData.pendingPage2.leaves,
      page: poolData.pendingPage2.page ?? 2,
      page_size: 20,
      total_count:
        poolData.pendingPage2.total_count ??
        poolData.pendingPage2.leaves.length,
    })
  }
  if (poolData.completed) {
    client.setQueryData(["leaves", "pool", "completed", 1, 20], {
      leaves: poolData.completed.leaves,
      page: poolData.completed.page ?? 1,
      page_size: 20,
      total_count: poolData.completed.total_count ?? poolData.completed.leaves.length,
    })
  }
  if (poolData.all) {
    client.setQueryData(["leaves", "pool", "all", 1, 20], {
      leaves: poolData.all.leaves,
      page: poolData.all.page ?? 1,
      page_size: 20,
      total_count: poolData.all.total_count ?? poolData.all.leaves.length,
    })
  }

  return render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <LeavesWorkbenchPage />
      </ToastProvider>
    </QueryClientProvider>,
  )
}

describe("LeavesWorkbenchPage", () => {
  beforeEach(() => {
    getMock.mockReset()
    postMock.mockReset()
    getMock.mockResolvedValue({
      data: { leaves: [], page: 1, page_size: 20, total_count: 0 },
    })
  })

  it("requests pending by default and renders urgency", () => {
    renderPage()

    expect(screen.getByText("leaves.workbench.title")).toBeInTheDocument()
    expect(screen.getByText("Alice")).toBeInTheDocument()
    expect(screen.getByText(/Front Desk/)).toBeInTheDocument()
    expect(screen.getByText("leaves.workbench.urgent")).toBeInTheDocument()
    expect(
      screen.getByText("leaves.workbench.urgency.remaining"),
    ).toBeInTheDocument()
  })

  it("renders workbench, pending pool, and the request-leave CTA", () => {
    renderPage({ pending: { leaves: [makeLeave()] } })

    expect(screen.getByText("leaves.workbench.title")).toBeInTheDocument()
    expect(screen.getByText("Alice")).toBeInTheDocument()
    expect(screen.getByText(/Front Desk/)).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "leaves.requestCta" })).toHaveAttribute(
      "href",
      "/leaves/new",
    )
    expect(screen.getByRole("link", { name: "leaves.workbench.actions.detail" })).toHaveAttribute(
      "href",
      "/leaves/15",
    )
  })

  it("resets pagination when changing status filters", async () => {
    const user = userEvent.setup()
    renderPage({
      pending: { leaves: [makeLeave()], total_count: 41 },
      pendingPage2: {
        leaves: [makeLeave({ id: 16, requester_name: "Page Two" })],
        total_count: 41,
      },
      completed: {
        leaves: [
          makeLeave({
            id: 17,
            state: "completed",
            requester_name: "Completed Alice",
            actions: {
              can_claim: false,
              can_approve: false,
              can_reject: false,
              can_cancel: false,
            },
            request: { state: "approved" },
          }),
        ],
      },
    })

    await user.click(screen.getByRole("button", { name: "leaves.workbench.next" }))
    expect(screen.getByText("Page Two")).toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: "leaves.workbench.filters.completed" }))

    expect(screen.getByText("Completed Alice")).toBeInTheDocument()
    expect(screen.getByText("leaves.workbench.page")).toBeInTheDocument()
  })

  it("wires public claim direct approval rejection and cancel actions", async () => {
    const user = userEvent.setup()
    postMock.mockResolvedValue({ data: undefined })

    const directLeave = makeLeave({
      id: 16,
      requester_name: "Direct Alice",
      request: {
        id: 22,
        type: "give_direct",
        counterpart_user_id: 2,
      },
      counterpart_name: "Bob",
      actions: {
        can_claim: false,
        can_approve: true,
        can_reject: true,
        can_cancel: false,
      },
    })
    const ownLeave = makeLeave({
      id: 17,
      requester_name: "Own Alice",
      actions: {
        can_claim: false,
        can_approve: false,
        can_reject: false,
        can_cancel: true,
      },
    })
    const leaves = [makeLeave(), directLeave, ownLeave]
    getMock.mockResolvedValue({
      data: { leaves, page: 1, page_size: 20, total_count: leaves.length },
    })
    renderPage({ pending: { leaves } })

    await user.click(screen.getByRole("button", { name: "leaves.workbench.actions.claim" }))
    await user.click(screen.getByRole("button", { name: "leaves.workbench.actions.approve" }))
    await user.click(screen.getByRole("button", { name: "leaves.workbench.actions.reject" }))
    await user.click(screen.getByRole("button", { name: "leaves.workbench.actions.cancel" }))

    await waitFor(() => {
      expect(postMock).toHaveBeenCalledWith(
        "/publications/7/shift-changes/21/approve",
      )
      expect(postMock).toHaveBeenCalledWith(
        "/publications/7/shift-changes/22/approve",
      )
      expect(postMock).toHaveBeenCalledWith(
        "/publications/7/shift-changes/22/reject",
      )
      expect(postMock).toHaveBeenCalledWith("/leaves/17/cancel")
    })
  })

  it("renders disabled not-qualified and admin view-only rows", () => {
    renderPage({
      pending: {
        leaves: [
          makeLeave({
            id: 18,
            requester_name: "No Position",
            actions: {
              can_claim: false,
              can_approve: false,
              can_reject: false,
              can_cancel: false,
              disabled_reason: "not_qualified",
            },
          }),
          makeLeave({
            id: 19,
            requester_name: "Admin Only",
            actions: {
              can_claim: false,
              can_approve: false,
              can_reject: false,
              can_cancel: false,
              disabled_reason: "admin_view_only",
            },
          }),
        ],
      },
    })

    expect(screen.getByText("leaves.workbench.disabled.not_qualified")).toBeInTheDocument()
    expect(screen.getByText("leaves.workbench.disabled.admin_view_only")).toBeInTheDocument()
  })
})
