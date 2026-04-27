import userEvent from "@testing-library/user-event"
import { screen, within } from "@testing-library/react"
import type { ComponentProps } from "react"
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
  {
    slot: {
      id: 3,
      weekday: 2,
      start_time: "09:00",
      end_time: "11:00",
    },
    positions: [
      {
        position: {
          id: 102,
          name: "Kitchen",
        },
        required_headcount: 1,
        candidates: [
          { user_id: 10, name: "Alice", email: "alice@example.com" },
        ],
        non_candidate_qualified: [],
        assignments: [],
      },
    ],
  },
]

const fullSlots: AssignmentBoardSlot[] = [
  {
    slot: {
      id: 1,
      weekday: 1,
      start_time: "09:00",
      end_time: "11:00",
    },
    positions: [
      {
        position: { id: 101, name: "Front Desk" },
        required_headcount: 1,
        candidates: [],
        non_candidate_qualified: [],
        assignments: [
          {
            assignment_id: 20,
            user_id: 11,
            name: "Bob",
            email: "bob@example.com",
          },
        ],
      },
    ],
  },
]

type AssignmentBoardProps = ComponentProps<typeof AssignmentBoard>

describe("AssignmentBoard", () => {
  it("shows the summary first and selecting a grid cell opens the editor", async () => {
    const user = userEvent.setup()
    const { container } = renderBoard()

    expect(screen.getByText("assignments.summary.title")).toBeInTheDocument()

    const firstCell = getGridButtons(container)[0]
    await user.click(firstCell)

    expect(firstCell).toHaveAttribute("aria-pressed", "true")
    expect(screen.getByText("Front Desk")).toBeInTheDocument()
    expect(screen.queryByText("assignments.summary.title")).not.toBeInTheDocument()
  })

  it("click-stages a candidate and cancels the inverse staged chip", async () => {
    const user = userEvent.setup()
    const onAssign = vi.fn()
    renderBoard({ onAssign })

    await user.click(getGridButtons(document.body)[0])
    await user.click(getButtonForText("Alice (2h)"))

    expect(onAssign).not.toHaveBeenCalled()
    expect(screen.getByText("assignments.drafts.added")).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "assignments.drafts.submit" }),
    ).toBeEnabled()

    await user.click(getButtonForText("Alice (2h)"))

    expect(screen.queryByText("assignments.drafts.added")).not.toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "assignments.drafts.submit" }),
    ).toBeDisabled()
  })

  it("click-stages an assigned chip for removal", async () => {
    const user = userEvent.setup()
    const onUnassign = vi.fn()
    renderBoard({ onUnassign })

    await user.click(getGridButtons(document.body)[0])
    await user.click(getButtonForText("Bob (2h)"))

    expect(onUnassign).not.toHaveBeenCalled()
    expect(screen.getByText("assignments.drafts.toRemove")).toBeInTheDocument()
  })

  it("keeps staged drafts visible after changing selection", async () => {
    const user = userEvent.setup()
    renderBoard()

    await user.click(getGridButtons(document.body)[0])
    await user.click(getButtonForText("Alice (2h)"))
    await user.click(getGridButtons(document.body)[1])
    await user.click(getGridButtons(document.body)[0])

    expect(screen.getByText("assignments.drafts.added")).toBeInTheDocument()
  })

  it("lists summary gaps in weekday then start-time order and jumps to a gap", async () => {
    const user = userEvent.setup()
    renderBoard()

    const summary = screen.getByText("assignments.summary.title").closest("aside")
    if (!summary) {
      throw new Error("Missing summary side panel")
    }
    const gapButtons = within(summary).getAllByRole("button")

    expect(gapButtons.map((button) => button.textContent)).toEqual([
      "templates.weekday.mon assignments.shiftSummaryassignments.headcount",
      "templates.weekday.mon assignments.shiftSummaryassignments.headcount",
      "templates.weekday.tue assignments.shiftSummaryassignments.headcount",
    ])

    await user.click(gapButtons[2])

    expect(screen.getByText("Kitchen")).toBeInTheDocument()
  })

  it("shows an empty summary state when every cell is full", () => {
    renderBoard({ slots: fullSlots })

    expect(screen.getByText("assignments.summary.noGaps")).toBeInTheDocument()
  })

  it("disables staging in read-only mode", async () => {
    const user = userEvent.setup()
    const onAssign = vi.fn()
    const onUnassign = vi.fn()
    renderBoard({ isReadOnly: true, onAssign, onUnassign })

    await user.click(getGridButtons(document.body)[0])
    await user.click(getButtonForText("Alice (2h)"))
    await user.click(getButtonForText("Bob (2h)"))

    expect(onAssign).not.toHaveBeenCalled()
    expect(onUnassign).not.toHaveBeenCalled()
    expect(
      screen.getByRole("button", { name: "assignments.drafts.submit" }),
    ).toBeDisabled()
  })

  it("resolves a cross-cell drag from assigned chip as remove plus add", () => {
    const state = resolveAssignmentBoardDrop({
      slots,
      draftState: emptyDraftState,
      selection: { slotID: 1, weekday: 1 },
      source: {
        kind: "assigned",
        assignment: {
          assignment_id: 20,
          user_id: 11,
          name: "Bob",
          email: "bob@example.com",
        },
        slotID: 1,
        weekday: 1,
        positionID: 101,
      },
      target: {
        kind: "cell",
        slotID: 2,
        weekday: 1,
      },
    })

    expect(state.ops.map((op) => op.kind)).toEqual(["unassign", "assign"])
  })

  it("resolves a cross-cell drag from candidate chip as one add", () => {
    const state = resolveAssignmentBoardDrop({
      slots,
      draftState: emptyDraftState,
      selection: { slotID: 1, weekday: 1 },
      source: {
        kind: "candidate",
        candidate: {
          user_id: 10,
          name: "Alice",
          email: "alice@example.com",
        },
        slotID: 1,
        weekday: 1,
        positionID: 101,
      },
      target: {
        kind: "cell",
        slotID: 2,
        weekday: 1,
      },
    })

    expect(state.ops).toHaveLength(1)
    expect(state.ops[0]).toMatchObject({ kind: "assign", userID: 10 })
  })

  it("rejects drops on off-schedule cells", () => {
    const state = resolveAssignmentBoardDrop({
      slots,
      draftState: emptyDraftState,
      selection: { slotID: 1, weekday: 1 },
      source: {
        kind: "candidate",
        candidate: {
          user_id: 10,
          name: "Alice",
          email: "alice@example.com",
        },
        slotID: 1,
        weekday: 1,
        positionID: 101,
      },
      target: {
        kind: "cell",
        slotID: 999,
        weekday: 6,
      },
    })

    expect(state.ops).toHaveLength(0)
  })

  it("stops draft submit on the first failure and keeps the rest queued", async () => {
    const user = userEvent.setup()
    const onDraftUnassign = vi.fn().mockResolvedValue(undefined)
    const onDraftAssign = vi.fn().mockRejectedValueOnce(new Error("request failed"))
    const onDraftRefresh = vi.fn().mockResolvedValue(undefined)

    renderBoard({
      initialDraftState: makeSubmitDraftState(),
      onDraftAssign,
      onDraftRefresh,
      onDraftUnassign,
    })

    await user.click(
      screen.getByRole("button", { name: "assignments.drafts.submit" }),
    )

    await screen.findByText("request failed")

    expect(onDraftUnassign).toHaveBeenCalledWith(20)
    expect(onDraftAssign).toHaveBeenCalledTimes(1)
    expect(onDraftAssign).toHaveBeenCalledWith(10, 2, 1, 101)
    expect(onDraftRefresh).toHaveBeenCalled()
  })

  it("clears drafts and refreshes after a successful draft submit", async () => {
    const user = userEvent.setup()
    const onDraftAssign = vi.fn().mockResolvedValue(undefined)
    const onDraftRefresh = vi.fn().mockResolvedValue(undefined)

    renderBoard({
      initialDraftState: {
        ops: [
          {
            id: "assign-1",
            kind: "assign",
            slotID: 2,
            weekday: 1,
            positionID: 101,
            userID: 10,
            userName: "Alice",
            userEmail: "alice@example.com",
            isUnqualified: false,
          },
        ],
      },
      onDraftAssign,
      onDraftRefresh,
    })

    const submitButton = screen.getByRole("button", {
      name: "assignments.drafts.submit",
    })

    await user.click(submitButton)
    await screen.findByRole("button", {
      name: "assignments.drafts.submit",
    })

    expect(onDraftAssign).toHaveBeenCalledWith(10, 2, 1, 101)
    expect(onDraftRefresh).toHaveBeenCalled()
    expect(submitButton).toBeDisabled()
  })
})

function renderBoard({
  slots: boardSlots = slots,
  isReadOnly = false,
  initialDraftState,
  onAssign = vi.fn(),
  onDraftAssign,
  onDraftRefresh,
  onDraftUnassign,
  onUnassign = vi.fn(),
}: {
  slots?: AssignmentBoardSlot[]
  isReadOnly?: boolean
  initialDraftState?: DraftState
  onAssign?: ReturnType<typeof vi.fn>
  onDraftAssign?: ReturnType<typeof vi.fn>
  onDraftRefresh?: ReturnType<typeof vi.fn>
  onDraftUnassign?: ReturnType<typeof vi.fn>
  onUnassign?: ReturnType<typeof vi.fn>
} = {}) {
  return renderWithProviders(
    <AssignmentBoard
      isPending={false}
      isReadOnly={isReadOnly}
      initialDraftState={initialDraftState}
      onAssign={onAssign as AssignmentBoardProps["onAssign"]}
      onDraftAssign={
        onDraftAssign as AssignmentBoardProps["onDraftAssign"] | undefined
      }
      onDraftRefresh={
        onDraftRefresh as AssignmentBoardProps["onDraftRefresh"] | undefined
      }
      onDraftUnassign={
        onDraftUnassign as AssignmentBoardProps["onDraftUnassign"] | undefined
      }
      onUnassign={onUnassign as AssignmentBoardProps["onUnassign"]}
      slots={boardSlots}
    />,
  )
}

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
        weekday: 1,
        positionID: 101,
      },
      {
        id: "assign-2",
        kind: "assign",
        slotID: 2,
        weekday: 1,
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
        weekday: 1,
        positionID: 101,
        userID: 12,
        userName: "Dana",
        userEmail: "dana@example.com",
        isUnqualified: false,
      },
    ],
  }
}

function getGridButtons(container: HTMLElement) {
  const table = container.querySelector("table")
  if (!table) {
    throw new Error("Missing assignment grid")
  }

  return within(table).getAllByRole("button")
}

function getButtonForText(text: string) {
  const node = screen.getByText(text)
  const button = node.closest("button")
  if (!button) {
    throw new Error(`Unable to find button for text: ${text}`)
  }

  return button
}
