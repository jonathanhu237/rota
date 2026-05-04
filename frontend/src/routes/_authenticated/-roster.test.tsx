import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import type { ReactNode } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ToastProvider } from "@/components/ui/toast"
import { TooltipProvider } from "@/components/ui/tooltip"
import type { Roster, User } from "@/lib/types"

const { downloadPublicationScheduleXLSXMock } = vi.hoisted(() => ({
  downloadPublicationScheduleXLSXMock: vi.fn(),
}))

vi.mock("@/lib/publications", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/publications")>(
      "@/lib/publications",
    )
  return {
    ...actual,
    downloadPublicationScheduleXLSX: downloadPublicationScheduleXLSXMock,
  }
})

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    )

  return {
    ...actual,
    createFileRoute: () => (options: object) => options,
  }
})

import { RosterPage } from "./roster"

describe("RosterPage schedule export", () => {
  beforeEach(() => {
    downloadPublicationScheduleXLSXMock.mockReset()
  })

  it("shows the download control when a roster publication is present", () => {
    renderPage(makeRoster())

    expect(
      screen.getByRole("button", { name: "roster.downloadExcel" }),
    ).toBeInTheDocument()
  })

  it("calls the shared download helper with the roster publication and current language", async () => {
    const user = userEvent.setup()
    downloadPublicationScheduleXLSXMock.mockResolvedValue(undefined)
    const roster = makeRoster()
    renderPage(roster)

    await user.click(screen.getByRole("button", { name: "roster.downloadExcel" }))

    expect(downloadPublicationScheduleXLSXMock).toHaveBeenCalledWith(
      roster.publication,
      "en",
    )
  })

  it("omits the download control from the empty roster state", () => {
    renderPage({
      publication: null,
      week_start: "",
      weekdays: [],
    })

    expect(
      screen.queryByRole("button", { name: "roster.downloadExcel" }),
    ).not.toBeInTheDocument()
  })
})

function renderPage(roster: Roster) {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  })
  client.setQueryData(["auth", "me"], makeUser())
  client.setQueryData(["roster", "current"], roster)
  client.setQueryData(["publications", 7, "members"], [])

  return render(
    <Providers client={client}>
      <RosterPage />
    </Providers>,
  )
}

function Providers({
  children,
  client,
}: {
  children: ReactNode
  client: QueryClient
}) {
  return (
    <QueryClientProvider client={client}>
      <ToastProvider>
        <TooltipProvider>{children}</TooltipProvider>
      </ToastProvider>
    </QueryClientProvider>
  )
}

function makeRoster(): Roster {
  return {
    publication: {
      id: 7,
      template_id: 3,
      template_name: "Main Template",
      name: "May Roster",
      description: "",
      state: "PUBLISHED",
      submission_start_at: "2026-04-20T00:00:00Z",
      submission_end_at: "2026-04-21T00:00:00Z",
      planned_active_from: "2026-04-22T00:00:00Z",
      planned_active_until: "2026-04-29T00:00:00Z",
      activated_at: null,
      created_at: "2026-04-19T00:00:00Z",
      updated_at: "2026-04-19T00:00:00Z",
    },
    week_start: "2026-04-20",
    weekdays: [
      { weekday: 1, slots: [] },
      { weekday: 2, slots: [] },
      { weekday: 3, slots: [] },
      { weekday: 4, slots: [] },
      { weekday: 5, slots: [] },
      { weekday: 6, slots: [] },
      { weekday: 7, slots: [] },
    ],
  }
}

function makeUser(): User {
  return {
    id: 7,
    email: "worker@example.com",
    name: "Worker",
    is_admin: false,
    status: "active",
    version: 1,
    language_preference: null,
    theme_preference: null,
  }
}
