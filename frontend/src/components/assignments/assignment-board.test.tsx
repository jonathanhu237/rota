import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import type { AssignmentBoardSlot } from "@/lib/types"
import { renderWithProviders } from "@/test-utils/render"

import { AssignmentBoard } from "./assignment-board"

const slots: AssignmentBoardSlot[] = [
  {
    slot: {
      id: 1,
      weekday: 1,
      start_time: "09:00",
      end_time: "11:00",
    },
    positions: [
      {
        position: {
          id: 101,
          name: "Front Desk",
        },
        required_headcount: 2,
        candidates: [
          { user_id: 10, name: "Alice", email: "alice@example.com" },
        ],
        non_candidate_qualified: [
          { user_id: 12, name: "Dana", email: "dana@example.com" },
        ],
        assignments: [
          { assignment_id: 20, user_id: 11, name: "Bob", email: "bob@example.com" },
        ],
      },
    ],
  },
]

describe("AssignmentBoard", () => {
  it("renders candidates and assigned users and supports assignment actions", async () => {
    const user = userEvent.setup()
    const onAssign = vi.fn()
    const onUnassign = vi.fn()

    const { container, getByRole, getByText, queryByText } = renderWithProviders(
      <AssignmentBoard
        isPending={false}
        isReadOnly={false}
        onAssign={onAssign}
        onUnassign={onUnassign}
        slots={slots}
      />,
    )

    expect(getByText("assignments.understaffed")).toBeInTheDocument()
    expect(
      getByText("assignments.understaffed").closest("article"),
    ).toHaveClass("border-amber-300")

    const buttons = Array.from(container.querySelectorAll("button"))
    const aliceButton = buttons.find((button) => button.textContent === "Alice")
    const bobButton = buttons.find((button) => button.textContent === "Bob")

    expect(aliceButton).toBeTruthy()
    expect(bobButton).toBeTruthy()
    expect(queryByText("Dana")).not.toBeInTheDocument()

    await user.click(aliceButton as HTMLElement)
    await user.click(bobButton as HTMLElement)
    await user.click(
      getByRole("switch", {
        name: "publications.assignmentBoard.showAllQualified",
      }),
    )

    const danaButton = Array.from(container.querySelectorAll("button")).find(
      (button) => button.textContent?.includes("Dana"),
    )

    expect(danaButton).toBeTruthy()

    await user.click(danaButton as HTMLElement)

    expect(onAssign).toHaveBeenCalledWith(10, 1, 101)
    expect(onAssign).toHaveBeenCalledWith(12, 1, 101)
    expect(onUnassign).toHaveBeenCalledWith(20)
  })

  it("disables mutations in read-only mode", async () => {
    const user = userEvent.setup()
    const onAssign = vi.fn()
    const onUnassign = vi.fn()

    const { container } = renderWithProviders(
      <AssignmentBoard
        isPending={false}
        isReadOnly
        onAssign={onAssign}
        onUnassign={onUnassign}
        slots={slots}
      />,
    )

    const buttons = Array.from(container.querySelectorAll("button"))
    const aliceButton = buttons.find((button) => button.textContent === "Alice")

    expect(aliceButton).toBeTruthy()

    await user.click(aliceButton as HTMLElement)

    expect(onAssign).not.toHaveBeenCalled()
    expect(onUnassign).not.toHaveBeenCalled()
  })
})
