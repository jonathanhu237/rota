import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import type { ReactNode } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ToastProvider } from "@/components/ui/toast"
import { TooltipProvider } from "@/components/ui/tooltip"
import type { AssignmentBoard, Publication } from "@/lib/types"

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
    createFileRoute: () => (options: object) => ({
      ...options,
      useParams: () => ({ publicationId: "7" }),
    }),
    redirect: vi.fn(),
  }
})

import { PublicationAssignmentsPage } from "./$publicationId/assignments"

describe("PublicationAssignmentsPage schedule export", () => {
  beforeEach(() => {
    downloadPublicationScheduleXLSXMock.mockReset()
  })

  it("shows the download control in assigning state and passes publication id with current language", async () => {
    const user = userEvent.setup()
    downloadPublicationScheduleXLSXMock.mockResolvedValue(undefined)
    const board = makeAssignmentBoard("ASSIGNING")

    renderPage(board)

    const button = screen.getByRole("button", {
      name: "assignments.downloadExcel",
    })
    expect(button).toBeEnabled()

    await user.click(button)

    await waitFor(() => {
      expect(downloadPublicationScheduleXLSXMock).toHaveBeenCalledWith(
        board.publication,
        "en",
      )
    })
  })
})

function renderPage(board: AssignmentBoard) {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  })
  client.setQueryData(["publications", "detail", 7, "board"], board)

  return render(
    <Providers client={client}>
      <PublicationAssignmentsPage />
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

function makeAssignmentBoard(state: Publication["state"]): AssignmentBoard {
  return {
    publication: {
      id: 7,
      template_id: 3,
      template_name: "Main Template",
      name: "May Roster",
      description: "",
      state,
      submission_start_at: "2026-04-20T00:00:00Z",
      submission_end_at: "2026-04-21T00:00:00Z",
      planned_active_from: "2026-04-22T00:00:00Z",
      planned_active_until: "2026-04-29T00:00:00Z",
      activated_at: null,
      created_at: "2026-04-19T00:00:00Z",
      updated_at: "2026-04-19T00:00:00Z",
    },
    slots: [],
    employees: [],
  }
}
