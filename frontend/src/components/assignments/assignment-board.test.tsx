import userEvent from "@testing-library/user-event"
import { screen, within } from "@testing-library/react"
import type { ComponentProps } from "react"
import { describe, expect, it, vi } from "vitest"

import {
  deriveEmployeeDirectory,
  type Employee,
} from "@/components/assignments/assignment-board-directory"
import type { AssignmentBoardEmployee, AssignmentBoardSlot } from "@/lib/types"
import { renderWithProviders } from "@/test-utils/render"

import { AssignmentBoard } from "./assignment-board"
import { resolveAssignmentBoardDrop } from "./assignment-board-dnd"
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
        assignments: [],
      },
    ],
  },
]

const employees: AssignmentBoardEmployee[] = [
  { user_id: 10, name: "Alice", email: "alice@example.com", position_ids: [101] },
  { user_id: 11, name: "Bob", email: "bob@example.com", position_ids: [101] },
  { user_id: 12, name: "Dana", email: "dana@example.com", position_ids: [101] },
  { user_id: 13, name: "Cara", email: "cara@example.com", position_ids: [102] },
]

type AssignmentBoardProps = ComponentProps<typeof AssignmentBoard>

describe("AssignmentBoard", () => {
  it("renders the seat grid and always-visible employee directory", () => {
    renderBoard()

    const directory = getDirectory()

    expect(screen.getAllByTestId("assignment-seat")).toHaveLength(6)
    expect(within(directory).getByText("Alice")).toBeInTheDocument()
    expect(within(directory).getByText("Bob")).toBeInTheDocument()
    expect(within(directory).getByText("Cara")).toBeInTheDocument()
    expect(within(directory).getByText("Dana")).toBeInTheDocument()
    expect(within(directory).getByText("assignments.directory.gaps"))
      .toBeInTheDocument()
  })

  it("filters the directory search by name", async () => {
    const user = userEvent.setup()
    renderBoard()

    const directory = getDirectory()
    await user.type(
      within(directory).getByLabelText("assignments.directory.search"),
      "da",
    )

    expect(getDirectoryRowNames(directory)).toEqual(["Dana"])
  })

  it("sorts the directory by hours by default and by name on toggle", async () => {
    const user = userEvent.setup()
    renderBoard()

    const directory = getDirectory()

    expect(getDirectoryRowNames(directory)).toEqual([
      "Alice",
      "Dana",
      "Bob",
      "Cara",
    ])

    await user.click(
      within(directory).getByRole("button", {
        name: "assignments.directory.sortByName",
      }),
    )

    expect(getDirectoryRowNames(directory)).toEqual([
      "Alice",
      "Bob",
      "Cara",
      "Dana",
    ])
  })

  it("clicks a filled seat x to stage unassign and clicks it again to cancel", async () => {
    const user = userEvent.setup()
    renderBoard()

    await user.click(getButtonForText("Bob (2h)"))

    expect(screen.getByText("assignments.drafts.toRemove")).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "assignments.drafts.submit" }),
    ).toBeEnabled()

    const stagedButton = screen
      .getByText("assignments.drafts.toRemove")
      .closest("button")
    if (!stagedButton) {
      throw new Error("Missing staged unassign button")
    }

    await user.click(stagedButton)

    expect(
      screen.queryByText("assignments.drafts.toRemove"),
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "assignments.drafts.submit" }),
    ).toBeDisabled()
  })

  it("disables staging in read-only mode", async () => {
    const user = userEvent.setup()
    renderBoard({ isReadOnly: true })

    await user.click(getButtonForText("Bob (2h)"))

    expect(
      screen.queryByText("assignments.drafts.toRemove"),
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "assignments.drafts.submit" }),
    ).toBeDisabled()
  })

  it("resolves a directory drop on an empty seat as one add", () => {
    const directory = makeDirectory()
    const state = resolveAssignmentBoardDrop({
      directory,
      draftState: emptyDraftState,
      source: {
        kind: "directory-employee",
        employee: getEmployee(directory, 10),
      },
      target: {
        kind: "seat",
        slotID: 2,
        weekday: 1,
        positionID: 101,
        headcountIndex: 0,
        filledBy: null,
        cellUserIDs: [],
      },
    })

    expect(state.ops).toHaveLength(1)
    expect(state.ops[0]).toMatchObject({ kind: "assign", userID: 10 })
  })

  it("resolves a directory drop on a filled seat as replace", () => {
    const directory = makeDirectory()
    const state = resolveAssignmentBoardDrop({
      directory,
      draftState: emptyDraftState,
      source: {
        kind: "directory-employee",
        employee: getEmployee(directory, 10),
      },
      target: {
        kind: "seat",
        slotID: 1,
        weekday: 1,
        positionID: 101,
        headcountIndex: 0,
        filledBy: {
          assignment_id: 20,
          user_id: 11,
          name: "Bob",
          email: "bob@example.com",
        },
        cellUserIDs: [11],
      },
    })

    expect(state.ops.map((op) => op.kind)).toEqual(["unassign", "assign"])
    expect(state.ops[0]).toMatchObject({ assignmentID: 20, userID: 11 })
    expect(state.ops[1]).toMatchObject({ userID: 10 })
  })

  it("resolves a cross-seat drag from an assigned chip", () => {
    const directory = makeDirectory()
    const state = resolveAssignmentBoardDrop({
      directory,
      draftState: emptyDraftState,
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
        kind: "seat",
        slotID: 2,
        weekday: 1,
        positionID: 101,
        headcountIndex: 1,
        filledBy: null,
        cellUserIDs: [],
      },
    })

    expect(state.ops.map((op) => op.kind)).toEqual(["unassign", "assign"])
    expect(state.ops[1]).toMatchObject({
      kind: "assign",
      userID: 11,
      slotID: 2,
      weekday: 1,
      positionID: 101,
    })
  })

  it("treats a drop onto a seat already held by the same user as a no-op", () => {
    const directory = makeDirectory()
    const state = resolveAssignmentBoardDrop({
      directory,
      draftState: emptyDraftState,
      source: {
        kind: "directory-employee",
        employee: getEmployee(directory, 11),
      },
      target: {
        kind: "seat",
        slotID: 1,
        weekday: 1,
        positionID: 101,
        headcountIndex: 0,
        filledBy: {
          assignment_id: 20,
          user_id: 11,
          name: "Bob",
          email: "bob@example.com",
        },
        cellUserIDs: [11],
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
  employees: boardEmployees = employees,
  isReadOnly = false,
  initialDraftState,
  onAssign = vi.fn(),
  onDraftAssign,
  onDraftRefresh,
  onDraftUnassign,
  onUnassign = vi.fn(),
}: {
  slots?: AssignmentBoardSlot[]
  employees?: AssignmentBoardEmployee[]
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
      employees={boardEmployees}
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

function getDirectory() {
  const directory = screen
    .getByText("assignments.directory.title")
    .closest("aside")
  if (!directory) {
    throw new Error("Missing employee directory")
  }

  return directory
}

function getDirectoryRowNames(directory: HTMLElement) {
  return within(directory)
    .getAllByTestId("assignment-directory-row")
    .map((row) => row.getAttribute("aria-label"))
}

function getButtonForText(text: string) {
  const node = screen.getByText(text)
  const button = node.closest("button")
  if (!button) {
    throw new Error(`Unable to find button for text: ${text}`)
  }

  return button
}

function makeDirectory() {
  return deriveEmployeeDirectory(employees)
}

function getEmployee(directory: Map<number, Employee>, userID: number) {
  const employee = directory.get(userID)
  if (!employee) {
    throw new Error(`Missing employee ${userID}`)
  }

  return employee
}
