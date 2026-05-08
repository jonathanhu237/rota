import type { AnchorHTMLAttributes, ForwardedRef, ReactNode } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ToastProvider } from "@/components/ui/toast"
import type { LeavePreviewOccurrence, Publication } from "@/lib/types"

import { LeavePage } from "./new"

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

function makeOccurrence(
  patch: Partial<LeavePreviewOccurrence> = {},
): LeavePreviewOccurrence {
  const base: LeavePreviewOccurrence = {
    assignment_id: 31,
    occurrence_date: "2026-04-25",
    slot: {
      id: 21,
      weekday: 6,
      start_time: "09:00",
      end_time: "12:00",
    },
    position: {
      id: 101,
      name: "Front Desk",
    },
    occurrence_start: "2026-04-25T09:00:00Z",
    occurrence_end: "2026-04-25T12:00:00Z",
    direct_candidates: [{ user_id: 2, name: "Bob" }],
  }
  return {
    ...base,
    ...patch,
    slot: { ...base.slot, ...patch.slot },
    position: { ...base.position, ...patch.position },
    direct_candidates: patch.direct_candidates ?? base.direct_candidates,
  }
}

function renderPage(occurrences: LeavePreviewOccurrence[] = []) {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  })
  client.setQueryData(["publications", "current"], makePublication())
  const today = new Date().toISOString().slice(0, 10)
  client.setQueryData(
    ["me", "leaves", "preview", today, addDays(today, 14)],
    occurrences,
  )

  return render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <LeavePage />
      </ToastProvider>
    </QueryClientProvider>,
  )
}

describe("LeavePage", () => {
  beforeEach(() => {
    getMock.mockReset()
    postMock.mockReset()
    getMock.mockResolvedValue({ data: { occurrences: [] } })
    postMock.mockResolvedValue({
      data: {
        leave: {
          id: 42,
          share_url: "/leaves/42",
        },
      },
    })
  })

  it("renders the moved leave request page without a duplicate back link", async () => {
    renderPage()

    expect(screen.getByText("leave.title")).toBeInTheDocument()
    expect(
      screen.queryByRole("link", { name: "leaves.backToHistory" }),
    ).not.toBeInTheDocument()
  })

  it("uses occurrence direct candidates and submits one row independently", async () => {
    const user = userEvent.setup()
    renderPage([
      makeOccurrence(),
      makeOccurrence({
        assignment_id: 32,
        occurrence_date: "2026-04-26",
        position: { id: 102, name: "Nurse" },
        direct_candidates: [{ user_id: 3, name: "Dana" }],
      }),
    ])

    expect(screen.getByText("Front Desk · 2026-04-25")).toBeInTheDocument()
    expect(screen.getByText("Nurse · 2026-04-26")).toBeInTheDocument()

    const selects = screen.getAllByRole("combobox")
    await user.selectOptions(selects[0], "give_direct")
    expect(screen.getByRole("option", { name: "Bob" })).toBeInTheDocument()
    expect(screen.queryByRole("option", { name: "Carol" })).not.toBeInTheDocument()
    await user.selectOptions(selects[2], "2")
    await user.click(screen.getAllByRole("button", { name: "leave.submit" })[0])

    await waitFor(() => {
      expect(postMock).toHaveBeenCalledTimes(1)
    })
    expect(postMock).toHaveBeenCalledWith("/leaves", {
      assignment_id: 31,
      occurrence_date: "2026-04-25",
      type: "give_direct",
      counterpart_user_id: 2,
      category: "personal",
      reason: "",
    })
  })
})

function addDays(dateValue: string, days: number) {
  const date = new Date(`${dateValue}T00:00:00Z`)
  date.setUTCDate(date.getUTCDate() + days)
  return date.toISOString().slice(0, 10)
}
