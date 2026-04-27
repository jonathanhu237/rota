import { DndContext } from "@dnd-kit/core"
import { render, screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"

import type { Employee } from "@/components/assignments/assignment-board-directory"
import type { ProjectedAssignment } from "@/components/assignments/draft-state"

import { AssignmentBoardSeat } from "./assignment-board-seat"

const directory = new Map<number, Employee>([
  [
    10,
    {
      user_id: 10,
      name: "Alice",
      email: "alice@example.com",
      position_ids: new Set([101]),
    },
  ],
])

const assignment: ProjectedAssignment = {
  assignment_id: 20,
  user_id: 10,
  name: "Alice",
  email: "alice@example.com",
}

describe("AssignmentBoardSeat", () => {
  it("renders a filled seat with a chip affordance", () => {
    renderSeat({ filledBy: assignment, filledLabel: "Alice (2h)" })

    expect(screen.getByText("Front Desk")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: /Alice \(2h\)/ })).toBeInTheDocument()
  })

  it("renders an empty seat placeholder", () => {
    renderSeat({ filledBy: null })

    expect(screen.getByText("assignments.seat.empty · Front Desk")).toBeInTheDocument()
  })

  it("uses green drag styling when the dragged user is qualified", () => {
    renderSeat({ filledBy: null, draggingUserID: 10 })

    expect(screen.getByTestId("assignment-seat")).toHaveClass("border-emerald-500")
  })

  it("uses yellow drag styling when the dragged user is unqualified", () => {
    renderSeat({ filledBy: null, draggingUserID: 10, positionID: 102 })

    expect(screen.getByTestId("assignment-seat")).toHaveClass("border-amber-500")
  })
})

function renderSeat({
  filledBy,
  filledLabel,
  draggingUserID = null,
  positionID = 101,
}: {
  filledBy: ProjectedAssignment | null
  filledLabel?: string
  draggingUserID?: number | null
  positionID?: number
}) {
  return render(
    <DndContext>
      <AssignmentBoardSeat
        slotID={1}
        weekday={1}
        positionID={positionID}
        headcountIndex={0}
        positionName="Front Desk"
        filledBy={filledBy}
        filledLabel={filledLabel}
        cellUserIDs={filledBy ? [filledBy.user_id] : []}
        draggingUserID={draggingUserID}
        directory={directory}
        disabled={false}
        isReadOnly={false}
        onUnassignClick={vi.fn()}
        onCancelDraft={vi.fn()}
      />
    </DndContext>,
  )
}
