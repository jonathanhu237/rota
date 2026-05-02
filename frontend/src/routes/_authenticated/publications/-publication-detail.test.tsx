import type {
  AnchorHTMLAttributes,
  ForwardedRef,
  ReactNode,
} from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ToastProvider } from "@/components/ui/toast"
import { TooltipProvider } from "@/components/ui/tooltip"
import type { Publication, User } from "@/lib/types"

const { navigateMock } = vi.hoisted(() => ({
  navigateMock: vi.fn(),
}))

const { getMock, patchMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  patchMock: vi.fn(),
}))

vi.mock("@/lib/axios", () => ({
  default: {
    delete: vi.fn(),
    get: getMock,
    patch: patchMock,
    post: vi.fn(),
    put: vi.fn(),
  },
}))

type LinkMockProps = {
  to: string
  children?: ReactNode
  params?: unknown
} & AnchorHTMLAttributes<HTMLAnchorElement>

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    )
  const React = await vi.importActual<typeof import("react")>("react")
  const Link = React.forwardRef(function LinkMock(
    { to, children, params, ...props }: LinkMockProps,
    ref: ForwardedRef<HTMLAnchorElement>,
  ) {
    void params
    return React.createElement("a", { href: to, ref, ...props }, children)
  })

  return {
    ...actual,
    Link,
    Outlet: () => null,
    createFileRoute: () => (options: object) => ({
      ...options,
      useParams: () => ({ publicationId: "7" }),
    }),
    useNavigate: () => navigateMock,
  }
})

import { PublicationDetailPage } from "./$publicationId"

describe("PublicationDetailPage", () => {
  beforeEach(() => {
    getMock.mockReset()
    navigateMock.mockReset()
    patchMock.mockReset()
  })

  it("converts planned_active_until edits to RFC3339 before updating", async () => {
    const user = userEvent.setup()
    const publication = makePublication()
    const plannedUntil = "2026-05-09T18:30"
    const expectedPayload = {
      planned_active_until: new Date(Date.parse(plannedUntil)).toISOString(),
    }
    getMock.mockResolvedValue({ data: { publication } })
    patchMock.mockResolvedValue({
      data: {
        publication: {
          ...publication,
          planned_active_until: expectedPayload.planned_active_until,
        },
      },
    })

    renderPage(publication)

    fireEvent.change(
      screen.getByTestId("publication-planned-until-edit-value"),
      {
        target: { value: plannedUntil },
      },
    )
    await user.click(
      screen.getByRole("button", { name: "publications.detail.save" }),
    )

    await waitFor(() => {
      expect(patchMock).toHaveBeenCalledWith("/publications/7", expectedPayload)
    })
  })

  it("updates planned_active_until through the visible date and time controls", async () => {
    const user = userEvent.setup()
    const publication = makePublication()
    getMock.mockResolvedValue({ data: { publication } })
    patchMock.mockResolvedValue({ data: { publication } })

    renderPage(publication)

    await user.click(screen.getByLabelText("publications.detail.editPlannedActiveUntil"))
    const dateButton = document.querySelector(
      '[data-day="4/30/2026"]',
    ) as HTMLElement
    const selectedDate = "2026-04-30"
    fireEvent.click(dateButton)
    fireEvent.change(
      screen.getByLabelText(
        "publications.detail.editPlannedActiveUntil common.time",
      ),
      {
        target: { value: "18:30" },
      },
    )
    await user.click(
      screen.getByRole("button", { name: "publications.detail.save" }),
    )

    await waitFor(() => {
      expect(patchMock).toHaveBeenCalledWith("/publications/7", {
        planned_active_until: new Date(
          Date.parse(`${selectedDate}T18:30`),
        ).toISOString(),
      })
    })
  })
})

function renderPage(publication: Publication) {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  })
  client.setQueryData(["auth", "me"], makeAdminUser())
  client.setQueryData(["publications", "detail", 7], publication)

  return render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <TooltipProvider>
          <PublicationDetailPage />
        </TooltipProvider>
      </ToastProvider>
    </QueryClientProvider>,
  )
}

function makeAdminUser(): User {
  return {
    id: 1,
    email: "admin@example.com",
    name: "Admin",
    is_admin: true,
    status: "active",
    version: 1,
    language_preference: null,
    theme_preference: null,
  }
}

function makePublication(): Publication {
  return {
    id: 7,
    template_id: 3,
    template_name: "Main Template",
    name: "Publication Detail",
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
