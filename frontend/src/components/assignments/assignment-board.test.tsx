import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import type { AssignmentBoardShift } from "@/lib/types"
import { renderWithProviders } from "@/test-utils/render"

import { AssignmentBoard } from "./assignment-board"

const shifts: AssignmentBoardShift[] = [
  {
    shift: {
      id: 1,
      weekday: 1,
      start_time: "09:00",
      end_time: "11:00",
      position_id: 101,
      position_name: "Front Desk",
      required_headcount: 2,
    },
    candidates: [
      { user_id: 10, name: "Alice", email: "alice@example.com" },
    ],
    assignments: [
      { assignment_id: 20, user_id: 11, name: "Bob", email: "bob@example.com" },
    ],
  },
]

describe("AssignmentBoard", () => {
  it("renders candidates and assigned users and supports assignment actions", async () => {
    const user = userEvent.setup()
    const onAssign = vi.fn()
    const onUnassign = vi.fn()

    const { container, getByText } = renderWithProviders(
      <AssignmentBoard
        isPending={false}
        isReadOnly={false}
        onAssign={onAssign}
        onUnassign={onUnassign}
        shifts={shifts}
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

    await user.click(aliceButton as HTMLElement)
    await user.click(bobButton as HTMLElement)

    expect(onAssign).toHaveBeenCalledWith(10, 1)
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
        shifts={shifts}
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
