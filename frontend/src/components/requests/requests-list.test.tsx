import userEvent from "@testing-library/user-event"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { ToastProvider } from "@/components/ui/toast"
import type {
  PublicationMember,
  RosterWeekday,
  ShiftChangeRequest,
} from "@/lib/types"
import { renderWithProviders } from "@/test-utils/render"

import { RequestsList } from "./requests-list"

const cancelMock = vi.fn()

vi.mock("@/lib/queries", async (importActual) => {
  const actual = await importActual<typeof import("@/lib/queries")>()
  return {
    ...actual,
    approveShiftChangeRequest: vi.fn(() => Promise.resolve()),
    rejectShiftChangeRequest: vi.fn(() => Promise.resolve()),
    cancelShiftChangeRequest: (...args: unknown[]) => {
      cancelMock(...args)
      return Promise.resolve()
    },
  }
})

const members: PublicationMember[] = [
  { user_id: 1, name: "Alice" },
  { user_id: 2, name: "Bob" },
  { user_id: 3, name: "Carol" },
]

const rosterWeekdays: RosterWeekday[] = [
  {
    weekday: 1,
    slots: [
      {
        occurrence_date: "2026-04-20",
        slot: {
          id: 10,
          weekday: 1,
          start_time: "09:00",
          end_time: "12:00",
        },
        positions: [
          {
            position: {
              id: 101,
              name: "Front Desk",
            },
            required_headcount: 1,
            assignments: [{ assignment_id: 100, user_id: 2, name: "Bob" }],
          },
        ],
      },
      {
        occurrence_date: "2026-04-20",
        slot: {
          id: 20,
          weekday: 1,
          start_time: "13:00",
          end_time: "17:00",
        },
        positions: [
          {
            position: {
              id: 102,
              name: "Back Office",
            },
            required_headcount: 1,
            assignments: [{ assignment_id: 200, user_id: 1, name: "Alice" }],
          },
        ],
      },
    ],
  },
]

function buildRequest(
  overrides: Partial<ShiftChangeRequest> & Pick<ShiftChangeRequest, "id">,
): ShiftChangeRequest {
  return {
    publication_id: 10,
    type: "swap",
    requester_user_id: 2,
    requester_assignment_id: 100,
    occurrence_date: "2026-04-20",
    counterpart_user_id: 1,
    counterpart_assignment_id: 200,
    counterpart_occurrence_date: "2026-04-20",
    state: "pending",
    decided_by_user_id: null,
    created_at: "2026-04-18T09:00:00Z",
    decided_at: null,
    expires_at: "2026-04-25T09:00:00Z",
    ...overrides,
  }
}

function renderList(requests: ShiftChangeRequest[], currentUserID = 1) {
  return renderWithProviders(
    <ToastProvider>
      <RequestsList
        publicationID={10}
        requests={requests}
        members={members}
        currentUserID={currentUserID}
        rosterWeekdays={rosterWeekdays}
      />
    </ToastProvider>,
  )
}

describe("RequestsList", () => {
  beforeEach(() => {
    cancelMock.mockReset()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it("shows empty states for every section when there are no requests", () => {
    const { getByText } = renderList([])

    expect(getByText("requests.sections.emptyWaiting")).toBeInTheDocument()
    expect(getByText("requests.sections.emptySent")).toBeInTheDocument()
    expect(getByText("requests.sections.emptyPool")).toBeInTheDocument()
    expect(getByText("requests.sections.emptyHistory")).toBeInTheDocument()
  })

  it("partitions sent, waiting, pool, and history requests correctly", () => {
    const requests: ShiftChangeRequest[] = [
      // sent by me (currentUser=1)
      buildRequest({
        id: 1,
        requester_user_id: 1,
        counterpart_user_id: 2,
        type: "give_direct",
      }),
      // waiting for my response
      buildRequest({
        id: 2,
        requester_user_id: 2,
        counterpart_user_id: 1,
        type: "swap",
      }),
      // pool from another user
      buildRequest({
        id: 3,
        requester_user_id: 3,
        counterpart_user_id: null,
        counterpart_assignment_id: null,
        type: "give_pool",
      }),
      // history: rejected
      buildRequest({
        id: 4,
        requester_user_id: 1,
        counterpart_user_id: 2,
        state: "rejected",
      }),
      // pool request I started — should NOT appear in pool section
      buildRequest({
        id: 5,
        requester_user_id: 1,
        counterpart_user_id: null,
        counterpart_assignment_id: null,
        type: "give_pool",
      }),
    ]

    const { queryByText, getAllByRole } = renderList(requests)

    // The sent section should contain our cancel buttons (requests 1 and 5)
    const cancelButtons = getAllByRole("button", {
      name: "requests.actions.cancel",
    })
    expect(cancelButtons).toHaveLength(2)

    // Approve button appears for waiting (id=2) AND pool (id=3)
    // Waiting uses approve, pool uses claim
    const approveButtons = getAllByRole("button", {
      name: "requests.actions.approve",
    })
    expect(approveButtons).toHaveLength(1)

    const claimButtons = getAllByRole("button", {
      name: "requests.actions.claim",
    })
    expect(claimButtons).toHaveLength(1)

    const rejectButtons = getAllByRole("button", {
      name: "requests.actions.reject",
    })
    expect(rejectButtons).toHaveLength(1)

    // Empty states should NOT be rendered because each section has something
    expect(queryByText("requests.sections.emptyWaiting")).not.toBeInTheDocument()
    expect(queryByText("requests.sections.emptySent")).not.toBeInTheDocument()
    expect(queryByText("requests.sections.emptyPool")).not.toBeInTheDocument()
    expect(queryByText("requests.sections.emptyHistory")).not.toBeInTheDocument()
  })

  it("calls cancel mutation when cancel is clicked on a sent request", async () => {
    const user = userEvent.setup()
    const requests: ShiftChangeRequest[] = [
      buildRequest({
        id: 42,
        requester_user_id: 1,
        counterpart_user_id: 2,
        type: "give_direct",
      }),
    ]

    const { getByRole } = renderList(requests)

    await user.click(
      getByRole("button", { name: "requests.actions.cancel" }),
    )

    expect(cancelMock).toHaveBeenCalledWith(10, 42)
  })

  it("shows the admin-edit invalidation reason in history", () => {
    const { getByText } = renderList([
      buildRequest({
        id: 77,
        requester_user_id: 1,
        state: "invalidated",
        decided_at: "2026-04-18T10:00:00Z",
      }),
    ])

    expect(getByText("requests.history.invalidatedReason")).toBeInTheDocument()
  })

  it("renders slot and position summaries when roster data is available", () => {
    const { getByText } = renderList([
      buildRequest({
        id: 88,
        requester_user_id: 2,
        type: "give_direct",
        requester_assignment_id: 100,
        counterpart_assignment_id: null,
      }),
    ])

    expect(getByText("requests.card.shiftSummary · 2026-04-20")).toBeInTheDocument()
  })
})
