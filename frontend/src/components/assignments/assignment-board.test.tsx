import userEvent from "@testing-library/user-event"
import { screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"

import type { AssignmentBoardSlot } from "@/lib/types"
import { renderWithProviders } from "@/test-utils/render"

import { resolveAssignmentBoardDrop } from "./assignment-board-dnd"
import { AssignmentBoard } from "./assignment-board"
import { emptyDraftState, type DraftState } from "./draft-state"

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
          {
            assignment_id: 20,
            user_id: 11,
            name: "Bob",
            email: "bob@example.com",
          },
        ],
      },
      {
        position: {
          id: 102,
          name: "Kitchen",
        },
        required_headcount: 1,
        candidates: [],
        non_candidate_qualified: [],
        assignments: [
          {
            assignment_id: 21,
            user_id: 13,
            name: "Cara",
            email: "cara@example.com",
          },
        ],
      },
    ],
  },
  {
    slot: {
      id: 2,
      weekday: 1,
      start_time: "12:00",
      end_time: "15:30",
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
          { user_id: 11, name: "Bob", email: "bob@example.com" },
        ],
        assignments: [],
      },
    ],
  },
]

describe("AssignmentBoard", () => {
  it("renders candidates and assigned users with hours and supports immediate actions", async () => {
    const user = userEvent.setup()
    const onAssign = vi.fn()
    const onUnassign = vi.fn()

    const { container, getAllByText, getByRole, getByText, queryByText } =
      renderWithProviders(
        <AssignmentBoard
          isPending={false}
          isReadOnly={false}
          onAssign={onAssign}
          onUnassign={onUnassign}
          slots={slots}
        />,
      )

    expect(getAllByText("assignments.understaffed")).toHaveLength(2)
    expect(
      getAllByText("assignments.understaffed")[0].closest("section"),
    ).toHaveClass("border-amber-300")
    expect(getByText("Alice (2h)")).toBeInTheDocument()
    expect(getByText("Bob (2h)")).toBeInTheDocument()
    expect(queryByText("Dana")).not.toBeInTheDocument()

    await user.click(getActualButtonByText(container, "Alice (2h)"))
    await user.click(getActualButtonByText(container, "Bob (2h)"))
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

    await user.click(getActualButtonByText(container, "Alice (2h)"))

    expect(onAssign).not.toHaveBeenCalled()
    expect(onUnassign).not.toHaveBeenCalled()
  })

  it("resolves a drag MOVE from one cell to an open cell", () => {
    const state = resolveAssignmentBoardDrop({
      slots,
      draftState: emptyDraftState,
      source: {
        kind: "assignment",
        assignment: {
          assignment_id: 20,
          user_id: 11,
          name: "Bob",
          email: "bob@example.com",
        },
        slotID: 1,
        positionID: 101,
      },
      target: {
        kind: "cell",
        slotID: 2,
        positionID: 101,
      },
    })

    expect(state.ops.map((op) => op.kind)).toEqual(["unassign", "assign"])
  })

  it("resolves a drag SWAP when an assigned user is dropped on a full cell user", () => {
    const state = resolveAssignmentBoardDrop({
      slots,
      draftState: emptyDraftState,
      source: {
        kind: "assignment",
        assignment: {
          assignment_id: 20,
          user_id: 11,
          name: "Bob",
          email: "bob@example.com",
        },
        slotID: 1,
        positionID: 101,
      },
      target: {
        kind: "assignment",
        assignment: {
          assignment_id: 21,
          user_id: 13,
          name: "Cara",
          email: "cara@example.com",
        },
        slotID: 1,
        positionID: 102,
      },
    })

    expect(state.ops.map((op) => op.kind)).toEqual([
      "unassign",
      "unassign",
      "assign",
      "assign",
    ])
  })

  it("resolves a candidate drag REPLACE when dropped on an assigned user", () => {
    const state = resolveAssignmentBoardDrop({
      slots,
      draftState: emptyDraftState,
      source: {
        kind: "candidate",
        candidate: {
          user_id: 10,
          name: "Alice",
          email: "alice@example.com",
        },
        slotID: 1,
        positionID: 101,
      },
      target: {
        kind: "assignment",
        assignment: {
          assignment_id: 20,
          user_id: 11,
          name: "Bob",
          email: "bob@example.com",
        },
        slotID: 1,
        positionID: 101,
      },
    })

    expect(state.ops.map((op) => op.kind)).toEqual(["unassign", "assign"])
    expect(state.ops[1]).toMatchObject({ kind: "assign", userID: 10 })
  })

  it("resolves a candidate drag ADD when dropped on an open cell", () => {
    const state = resolveAssignmentBoardDrop({
      slots,
      draftState: emptyDraftState,
      source: {
        kind: "candidate",
        candidate: {
          user_id: 10,
          name: "Alice",
          email: "alice@example.com",
        },
        slotID: 1,
        positionID: 101,
      },
      target: {
        kind: "cell",
        slotID: 2,
        positionID: 101,
      },
    })

    expect(state.ops).toHaveLength(1)
    expect(state.ops[0]).toMatchObject({ kind: "assign", userID: 10 })
  })

  it("stops draft submit on the first failure and keeps the rest queued", async () => {
    const user = userEvent.setup()
    const onDraftUnassign = vi.fn().mockResolvedValue(undefined)
    const onDraftAssign = vi.fn().mockRejectedValueOnce(new Error("request failed"))
    const onDraftRefresh = vi.fn().mockResolvedValue(undefined)

    renderWithProviders(
      <AssignmentBoard
        isPending={false}
        isReadOnly={false}
        initialDraftState={makeSubmitDraftState()}
        onAssign={vi.fn()}
        onDraftAssign={onDraftAssign}
        onDraftRefresh={onDraftRefresh}
        onDraftUnassign={onDraftUnassign}
        onUnassign={vi.fn()}
        slots={slots}
      />,
    )

    await user.click(screen.getByRole("button", { name: "assignments.drafts.submit" }))

    await screen.findByText("request failed")

    expect(onDraftUnassign).toHaveBeenCalledWith(20)
    expect(onDraftAssign).toHaveBeenCalledTimes(1)
    expect(onDraftAssign).toHaveBeenCalledWith(10, 2, 101)
    expect(onDraftRefresh).toHaveBeenCalled()
  })

  it("clears drafts and refreshes after a successful draft submit", async () => {
    const user = userEvent.setup()
    const onDraftAssign = vi.fn().mockResolvedValue(undefined)
    const onDraftRefresh = vi.fn().mockResolvedValue(undefined)

    renderWithProviders(
      <AssignmentBoard
        isPending={false}
        isReadOnly={false}
        initialDraftState={{
          ops: [
            {
              id: "assign-1",
              kind: "assign",
              slotID: 2,
              positionID: 101,
              userID: 10,
              userName: "Alice",
              userEmail: "alice@example.com",
              isUnqualified: false,
            },
          ],
        }}
        onAssign={vi.fn()}
        onDraftAssign={onDraftAssign}
        onDraftRefresh={onDraftRefresh}
        onUnassign={vi.fn()}
        slots={slots}
      />,
    )

    const submitButton = screen.getByRole("button", {
      name: "assignments.drafts.submit",
    })

    await user.click(submitButton)
    await screen.findByRole("button", {
      name: "assignments.drafts.submit",
    })

    expect(onDraftAssign).toHaveBeenCalledWith(10, 2, 101)
    expect(onDraftRefresh).toHaveBeenCalled()
    expect(submitButton).toBeDisabled()
  })
})

function makeSubmitDraftState(): DraftState {
  return {
    ops: [
      {
        id: "unassign-1",
        kind: "unassign",
        assignmentID: 20,
        userID: 11,
        userName: "Bob",
        slotID: 1,
        positionID: 101,
      },
      {
        id: "assign-2",
        kind: "assign",
        slotID: 2,
        positionID: 101,
        userID: 10,
        userName: "Alice",
        userEmail: "alice@example.com",
        isUnqualified: false,
      },
      {
        id: "assign-3",
        kind: "assign",
        slotID: 2,
        positionID: 101,
        userID: 12,
        userName: "Dana",
        userEmail: "dana@example.com",
        isUnqualified: false,
      },
    ],
  }
}

function getActualButtonByText(container: HTMLElement, text: string) {
  const button = Array.from(container.querySelectorAll("button")).find(
    (element) => element.textContent === text,
  )

  if (!button) {
    throw new Error(`Unable to find button: ${text}`)
  }

  return button
}
