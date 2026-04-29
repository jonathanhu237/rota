import { DndContext } from "@dnd-kit/core"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
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
      submittedSlots: new Set(["1:1"]),
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

  it("uses amber drag styling when the dragged user is qualified but unsubmitted", () => {
    renderSeat({ filledBy: null, draggingUserID: 10, slotID: 2, weekday: 1 })

    expect(screen.getByTestId("assignment-seat")).toHaveClass("border-amber-500")
  })

  it("uses red drag styling when the dragged user is unqualified", () => {
    renderSeat({ filledBy: null, draggingUserID: 10, positionID: 102 })

    expect(screen.getByTestId("assignment-seat")).toHaveClass("border-red-500")
  })

  it("uses neutral drag styling when no user is dragging", () => {
    renderSeat({ filledBy: null })

    expect(screen.getByTestId("assignment-seat")).toHaveClass("border-border")
  })

  it("renders draft adds with a small dot indicator and no added badge", () => {
    renderSeat({
      filledBy: {
        ...assignment,
        isDraft: true,
        draftOpID: "assign-1",
      },
    })

    expect(screen.queryByText("assignments.drafts.added")).not.toBeInTheDocument()
    const button = screen.getByRole("button", { name: /Alice/ })
    expect(button).not.toHaveClass("border-l-4")
    expect(screen.getByTestId("assignment-draft-dot")).toBeInTheDocument()
  })

  it("renders removed chips with strikethrough and undo", async () => {
    const user = userEvent.setup()
    const onCancelDraft = vi.fn()
    renderSeat({
      filledBy: {
        ...assignment,
        isRemoved: true,
        draftOpID: "unassign-1",
      },
      onCancelDraft,
    })

    expect(screen.queryByText("assignments.drafts.toRemove")).not.toBeInTheDocument()
    expect(screen.getByText("Alice")).toHaveClass("line-through")

    await user.click(screen.getByLabelText("assignments.drafts.undoRemove"))

    expect(onCancelDraft).toHaveBeenCalledWith("unassign-1")
  })

  it("renders warning icons by override severity", () => {
    const { rerender } = renderSeat({
      filledBy: {
        ...assignment,
        isUnsubmitted: true,
      },
    })

    expect(screen.getByLabelText("assignments.drafts.unsubmittedAria")).toHaveClass(
      "text-amber-500",
    )
    expect(screen.queryByLabelText("assignments.drafts.unqualifiedAria"))
      .not.toBeInTheDocument()

    rerender(
      <DndContext>
        <AssignmentBoardSeat
          slotID={1}
          weekday={1}
          positionID={101}
          headcountIndex={0}
          positionName="Front Desk"
          filledBy={{
            ...assignment,
            isUnqualified: true,
            isUnsubmitted: true,
          }}
          cellUserIDs={[assignment.user_id]}
          draggingUserID={null}
          directory={directory}
          disabled={false}
          isReadOnly={false}
          onUnassignClick={vi.fn()}
          onCancelDraft={vi.fn()}
        />
      </DndContext>,
    )

    expect(screen.getByLabelText("assignments.drafts.unqualifiedAria")).toHaveClass(
      "text-red-500",
    )
    expect(screen.queryByLabelText("assignments.drafts.unsubmittedAria"))
      .not.toBeInTheDocument()
  })
})

function renderSeat({
  filledBy,
  filledLabel,
  draggingUserID = null,
  slotID = 1,
  weekday = 1,
  positionID = 101,
  onCancelDraft = vi.fn(),
}: {
  filledBy: ProjectedAssignment | null
  filledLabel?: string
  draggingUserID?: number | null
  slotID?: number
  weekday?: number
  positionID?: number
  onCancelDraft?: (draftOpID: string) => void
}) {
  return render(
    <DndContext>
      <AssignmentBoardSeat
        slotID={slotID}
        weekday={weekday}
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
        onCancelDraft={onCancelDraft}
      />
    </DndContext>,
  )
}
