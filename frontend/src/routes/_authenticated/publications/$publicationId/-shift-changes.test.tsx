import { describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"
import type { ShiftChangeRequest } from "@/lib/types"

import { ShiftChangeRequestsTable } from "./shift-changes"

const sampleRequests: ShiftChangeRequest[] = [
  {
    id: 42,
    publication_id: 1,
    type: "swap",
    requester_user_id: 10,
    requester_assignment_id: 100,
    occurrence_date: "2026-04-20",
    counterpart_user_id: 20,
    counterpart_assignment_id: 200,
    counterpart_occurrence_date: "2026-04-22",
    state: "pending",
    decided_by_user_id: null,
    created_at: "2026-04-17T09:00:00Z",
    decided_at: null,
    expires_at: "2026-04-24T09:00:00Z",
  },
  {
    id: 43,
    publication_id: 1,
    type: "give_pool",
    requester_user_id: 11,
    requester_assignment_id: 101,
    occurrence_date: "2026-04-21",
    counterpart_user_id: null,
    counterpart_assignment_id: null,
    counterpart_occurrence_date: null,
    state: "approved",
    decided_by_user_id: 30,
    created_at: "2026-04-17T10:00:00Z",
    decided_at: "2026-04-17T11:00:00Z",
    expires_at: "2026-04-24T10:00:00Z",
  },
]

describe("ShiftChangeRequestsTable", () => {
  it("renders a row per request with the resolved requester name", () => {
    const names = new Map<number, string>([
      [10, "Alice"],
      [11, "Bob"],
      [20, "Charlie"],
    ])

    const resolveName = vi.fn((userID: number | null) => {
      if (userID == null) return "common.notAvailable"
      return names.get(userID) ?? `#${userID}`
    })

    const { getByText, getAllByRole } = renderWithProviders(
      <ShiftChangeRequestsTable
        requests={sampleRequests}
        resolveName={resolveName}
        formatTimestamp={(value) => value ?? "common.notAvailable"}
      />,
    )

    expect(getByText("#42")).toBeInTheDocument()
    expect(getByText("#43")).toBeInTheDocument()
    expect(getByText("Alice")).toBeInTheDocument()
    expect(getByText("Bob")).toBeInTheDocument()
    expect(getByText("Charlie")).toBeInTheDocument()
    expect(
      getByText("publications.shiftChanges.requestType.swap"),
    ).toBeInTheDocument()
    expect(
      getByText("publications.shiftChanges.requestType.give_pool"),
    ).toBeInTheDocument()
    expect(
      getByText("publications.shiftChanges.state.pending"),
    ).toBeInTheDocument()
    expect(
      getByText("publications.shiftChanges.state.approved"),
    ).toBeInTheDocument()

    // header row + 2 data rows
    expect(getAllByRole("row")).toHaveLength(3)
  })

  it("shows an empty state when there are no requests", () => {
    const { getByText, getAllByRole } = renderWithProviders(
      <ShiftChangeRequestsTable
        requests={[]}
        resolveName={() => ""}
        formatTimestamp={() => ""}
      />,
    )

    expect(getByText("publications.shiftChanges.empty")).toBeInTheDocument()
    // header + empty-state row
    expect(getAllByRole("row")).toHaveLength(2)
  })
})
