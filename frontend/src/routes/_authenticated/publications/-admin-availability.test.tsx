import type {
  AnchorHTMLAttributes,
  ForwardedRef,
  ReactNode,
} from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ToastProvider } from "@/components/ui/toast"
import { TooltipProvider } from "@/components/ui/tooltip"
import type {
  AdminAvailabilityBoard,
  AdminAvailabilityDetail,
  Publication,
} from "@/lib/types"

const { getMock, putMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  putMock: vi.fn(),
}))

const { navigateMock, useBlockerMock } = vi.hoisted(() => ({
  navigateMock: vi.fn(),
  useBlockerMock: vi.fn(),
}))

vi.mock("@/lib/axios", () => ({
  default: {
    delete: vi.fn(),
    get: getMock,
    patch: vi.fn(),
    post: vi.fn(),
    put: putMock,
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
      { href: hrefFor(to, params), ref, ...props },
      children,
    )
  })

  return {
    ...actual,
    Link,
    Outlet: () => React.createElement("div", null, "standalone-child"),
    createFileRoute: () => (options: object) => ({
      ...options,
      useParams: () => ({ publicationId: "7", userId: "12" }),
    }),
    redirect: vi.fn(),
    useBlocker: useBlockerMock,
    useNavigate: () => navigateMock,
  }
})

import { PublicationLayout } from "./$publicationId"
import { PublicationAvailabilityLayout } from "./$publicationId/availability"
import { PublicationAvailabilityEditorPage } from "./$publicationId/availability/$userId"
import { PublicationAvailabilityPage } from "./$publicationId/availability/index"

describe("admin publication availability routes", () => {
  beforeEach(() => {
    getMock.mockReset()
    putMock.mockReset()
    navigateMock.mockReset()
    useBlockerMock.mockReset()
  })

  it("uses a thin publication layout instead of rendering the detail card above child pages", () => {
    render(<PublicationLayout />)

    expect(screen.getByText("standalone-child")).toBeInTheDocument()
    expect(
      screen.queryByText("publications.detail.description"),
    ).not.toBeInTheDocument()
  })

  it("uses a thin availability layout so the editor child route can render", () => {
    render(<PublicationAvailabilityLayout />)

    expect(screen.getByText("standalone-child")).toBeInTheDocument()
    expect(screen.queryByText("adminAvailability.title")).not.toBeInTheDocument()
  })

  it("renders zero-submission employees and opens the selected editor", async () => {
    getMock.mockImplementation((url: string, config?: { params?: unknown }) => {
      if (url === "/publications/7/availability-board") {
        const params = config?.params as { page?: number; search?: string }
        return Promise.resolve({
          data: makeBoard(params?.page ?? 1, params?.search ?? ""),
        })
      }
      return Promise.reject(new Error(`unexpected ${url}`))
    })

    renderWithClient(<PublicationAvailabilityPage />)

    expect(await screen.findByText("Zero Submitter")).toBeInTheDocument()
    expect(screen.getByText("zero@example.com")).toBeInTheDocument()
    await userEvent.click(
      screen.getAllByRole("button", {
        name: "adminAvailability.table.edit",
      })[0],
    )

    expect(navigateMock).toHaveBeenCalledWith({
      to: "/publications/$publicationId/availability/$userId",
      params: {
        publicationId: "7",
        userId: "12",
      },
    })
  })

  it("resets pagination to page 1 when search changes", async () => {
    const user = userEvent.setup()
    getMock.mockImplementation((url: string, config?: { params?: unknown }) => {
      if (url === "/publications/7/availability-board") {
        const params = config?.params as { page?: number; search?: string }
        return Promise.resolve({
          data: makeBoard(params?.page ?? 1, params?.search ?? ""),
        })
      }
      return Promise.reject(new Error(`unexpected ${url}`))
    })

    renderWithClient(<PublicationAvailabilityPage />)

    expect(await screen.findByText("Zero Submitter")).toBeInTheDocument()
    await user.click(
      screen.getByRole("button", {
        name: "adminAvailability.pagination.next",
      }),
    )
    expect(await screen.findByText("Page User 2")).toBeInTheDocument()
    await user.click(
      screen.getByRole("button", {
        name: "adminAvailability.pagination.next",
      }),
    )
    expect(await screen.findByText("Page User 3")).toBeInTheDocument()

    await user.type(
      screen.getByLabelText("adminAvailability.search.label"),
      "lin",
    )

    await waitFor(() => {
      expect(getMock).toHaveBeenLastCalledWith(
        "/publications/7/availability-board",
        {
          params: {
            page: 1,
            page_size: 10,
            search: "lin",
          },
        },
      )
    })
  })

  it("blocks unchecked ineligible cells while allowing submitted exceptions to be cleared", async () => {
    const detail = makeDetail("ASSIGNING")
    const client = makeClient()
    client.setQueryData(
      ["publications", "detail", 7, "availability", "detail", 12],
      detail,
    )

    renderWithClient(<PublicationAvailabilityEditorPage />, client)

    const ineligibleUnchecked = within(
      screen.getByTestId("admin-availability-cell-23-3"),
    ).getByRole("checkbox")
    const ineligibleSubmitted = within(
      screen.getByTestId("admin-availability-cell-22-2"),
    ).getByRole("checkbox")

    expect(ineligibleUnchecked).toBeDisabled()
    expect(ineligibleSubmitted).toBeEnabled()

    fireEvent.click(ineligibleSubmitted)

    expect(ineligibleSubmitted).not.toBeChecked()
    expect(ineligibleSubmitted).toBeDisabled()
    expect(
      screen.getByTestId("admin-availability-save-bar"),
    ).toBeInTheDocument()
  })

  it("discards local editor changes", async () => {
    const client = makeClient()
    client.setQueryData(
      ["publications", "detail", 7, "availability", "detail", 12],
      makeDetail("ASSIGNING"),
    )

    renderWithClient(<PublicationAvailabilityEditorPage />, client)

    const eligible = within(
      screen.getByTestId("admin-availability-cell-21-1"),
    ).getByRole("checkbox")
    fireEvent.click(eligible)
    expect(
      screen.getByTestId("admin-availability-save-bar"),
    ).toBeInTheDocument()

    await userEvent.click(
      screen.getByRole("button", {
        name: "adminAvailability.editor.discard",
      }),
    )

    expect(
      screen.queryByTestId("admin-availability-save-bar"),
    ).not.toBeInTheDocument()
    expect(eligible).toBeChecked()
  })

  it("saves the complete draft set and refreshes editor data", async () => {
    const client = makeClient()
    client.setQueryData(
      ["publications", "detail", 7, "availability", "detail", 12],
      makeDetail("ASSIGNING"),
    )
    putMock.mockResolvedValue({
      data: {
        ...makeDetail("ASSIGNING"),
        submissions: [{ slot_id: 22, weekday: 2 }],
      },
    })

    renderWithClient(<PublicationAvailabilityEditorPage />, client)

    fireEvent.click(
      within(screen.getByTestId("admin-availability-cell-22-2")).getByRole(
        "checkbox",
      ),
    )
    await userEvent.click(
      screen.getByRole("button", {
        name: "adminAvailability.editor.save",
      }),
    )

    await waitFor(() => {
      expect(putMock).toHaveBeenCalledWith(
        "/publications/7/availability-submissions/12",
        {
          submissions: [{ slot_id: 21, weekday: 1 }],
        },
      )
    })
  })

  it("registers a dirty route/refresh blocker and hides save controls in read-only states", () => {
    const mutableClient = makeClient()
    mutableClient.setQueryData(
      ["publications", "detail", 7, "availability", "detail", 12],
      makeDetail("ASSIGNING"),
    )
    const { unmount } = renderWithClient(
      <PublicationAvailabilityEditorPage />,
      mutableClient,
    )
    fireEvent.click(
      within(screen.getByTestId("admin-availability-cell-21-1")).getByRole(
        "checkbox",
      ),
    )

    expect(
      screen.getByRole("button", {
        name: "adminAvailability.editor.save",
      }),
    ).toBeDisabled()
    expect(useBlockerMock).toHaveBeenLastCalledWith(
      expect.objectContaining({
        disabled: false,
        enableBeforeUnload: expect.any(Function),
      }),
    )
    unmount()
    useBlockerMock.mockClear()

    const readOnlyClient = makeClient()
    readOnlyClient.setQueryData(
      ["publications", "detail", 7, "availability", "detail", 12],
      makeDetail("ACTIVE"),
    )
    renderWithClient(<PublicationAvailabilityEditorPage />, readOnlyClient)

    expect(
      within(screen.getByTestId("admin-availability-cell-21-1")).getByRole(
        "checkbox",
      ),
    ).toBeDisabled()
    expect(
      screen.queryByRole("button", {
        name: "adminAvailability.editor.save",
      }),
    ).not.toBeInTheDocument()
  })
})

function renderWithClient(ui: ReactNode, client = makeClient()) {
  return render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <TooltipProvider>{ui}</TooltipProvider>
      </ToastProvider>
    </QueryClientProvider>,
  )
}

function makeClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  })
}

function makeBoard(page: number, search: string): AdminAvailabilityBoard {
  const baseEmployee = search
    ? {
        user_id: 12,
        name: "Lin Hehuang",
        email: "lin@example.com",
        positions: [{ id: 101, name: "前台助理" }],
        submitted_count: 2,
      }
    : {
        user_id: 12,
        name: page === 1 ? "Zero Submitter" : `Page User ${page}`,
        email: "zero@example.com",
        positions: [{ id: 101, name: "前台助理" }],
        submitted_count: 0,
      }

  return {
    publication: makePublication("ASSIGNING"),
    employees: [baseEmployee],
    pagination: {
      page,
      page_size: 10,
      total: 30,
      total_pages: 3,
    },
  }
}

function makeDetail(state: Publication["state"]): AdminAvailabilityDetail {
  return {
    publication: makePublication(state),
    user: {
      id: 12,
      email: "lin@example.com",
      name: "林鹤煌",
      is_admin: false,
      status: "active",
      version: 1,
      language_preference: null,
      theme_preference: null,
    },
    positions: [{ id: 101, name: "前台助理" }],
    slots: [
      {
        slot: {
          id: 21,
          weekday: 1,
          start_time: "09:00",
          end_time: "10:00",
        },
        positions: [
          {
            position: { id: 101, name: "前台助理" },
            required_headcount: 1,
          },
        ],
      },
      {
        slot: {
          id: 22,
          weekday: 2,
          start_time: "09:00",
          end_time: "10:00",
        },
        positions: [
          {
            position: { id: 102, name: "外勤助理" },
            required_headcount: 1,
          },
        ],
      },
      {
        slot: {
          id: 23,
          weekday: 3,
          start_time: "09:00",
          end_time: "10:00",
        },
        positions: [
          {
            position: { id: 102, name: "外勤助理" },
            required_headcount: 1,
          },
        ],
      },
    ],
    submissions: [
      { slot_id: 21, weekday: 1 },
      { slot_id: 22, weekday: 2 },
    ],
    cells: [
      { slot_id: 21, weekday: 1, eligible: true, submitted: true },
      { slot_id: 22, weekday: 2, eligible: false, submitted: true },
      { slot_id: 23, weekday: 3, eligible: false, submitted: false },
    ],
  }
}

function makePublication(state: Publication["state"]): Publication {
  return {
    id: 7,
    template_id: 3,
    template_name: "Main Template",
    name: "May Rota",
    description: "",
    state,
    submission_start_at: "2026-04-20T00:00:00Z",
    submission_end_at: "2026-04-21T00:00:00Z",
    planned_active_from: "2026-04-22T00:00:00Z",
    planned_active_until: "2026-04-29T00:00:00Z",
    activated_at: null,
    created_at: "2026-04-19T00:00:00Z",
    updated_at: "2026-04-19T00:00:00Z",
  }
}

function hrefFor(to: string, params?: Record<string, string>) {
  let href = to
  for (const [key, value] of Object.entries(params ?? {})) {
    href = href.replace(`$${key}`, value)
  }
  return href
}
