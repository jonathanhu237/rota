import { DndContext } from "@dnd-kit/core"
import { render, screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"

import type { Employee } from "@/components/assignments/assignment-board-directory"
import { pivotIntoGridCells } from "@/components/assignments/assignment-board-grid-cells"
import type { AssignmentBoardSlot } from "@/lib/types"

import { AssignmentBoardCell } from "./assignment-board-cell"
import {
  applyDraftToSlots,
  emptyDraftState,
  enqueueAdd,
  type DraftState,
} from "./draft-state"

const directory = new Map<number, Employee>([
  [
    10,
    {
      user_id: 10,
      name: "Lead Person",
      email: "lead@example.com",
      position_ids: new Set([101]),
      submittedSlots: new Set(["1:1"]),
    },
  ],
  [
    11,
    {
      user_id: 11,
      name: "Assistant One",
      email: "assistant-one@example.com",
      position_ids: new Set([102]),
      submittedSlots: new Set(["1:1"]),
    },
  ],
  [
    12,
    {
      user_id: 12,
      name: "Assistant Two",
      email: "assistant-two@example.com",
      position_ids: new Set([102]),
      submittedSlots: new Set(["1:1"]),
    },
  ],
])

const composedSlots: AssignmentBoardSlot[] = [
  {
    slot: {
      id: 1,
      weekday: 1,
      start_time: "09:00",
      end_time: "11:00",
    },
    positions: [
      {
        position: { id: 101, name: "Lead" },
        required_headcount: 1,
        assignments: [
          {
            assignment_id: 20,
            user_id: 10,
            name: "Lead Person",
            email: "lead@example.com",
          },
        ],
      },
      {
        position: { id: 102, name: "Assistant" },
        required_headcount: 2,
        assignments: [],
      },
    ],
  },
]

const overflowSlots: AssignmentBoardSlot[] = [
  {
    slot: {
      id: 2,
      weekday: 1,
      start_time: "13:00",
      end_time: "15:00",
    },
    positions: [
      {
        position: { id: 102, name: "Assistant" },
        required_headcount: 1,
        assignments: [
          {
            assignment_id: 21,
            user_id: 11,
            name: "Assistant One",
            email: "assistant-one@example.com",
          },
          {
            assignment_id: 22,
            user_id: 12,
            name: "Assistant Two",
            email: "assistant-two@example.com",
          },
        ],
      },
    ],
  },
]

describe("AssignmentBoardCell", () => {
  it("renders one stable seat per required position headcount", () => {
    renderCell(composedSlots, 0, 0)

    const seats = screen.getAllByTestId("assignment-seat")
    expect(seats).toHaveLength(3)
    expect(seats[0]).toHaveTextContent("Lead")
    expect(seats[0]).toHaveTextContent("Lead Person (2h)")
    expect(seats[1]).toHaveTextContent("Assistant")
    expect(seats[2]).toHaveTextContent("Assistant")
  })

  it("renders overflow seats for over-assigned positions", () => {
    renderCell(overflowSlots, 0, 0)

    expect(screen.getAllByTestId("assignment-seat")).toHaveLength(2)
    expect(screen.getByText("assignments.seat.overflow")).toBeInTheDocument()
    expect(screen.getByText("Assistant Two (2h)")).toBeInTheDocument()
  })

  it("renders no seats for off-schedule cells", () => {
    renderCell(composedSlots, 0, 1)

    expect(screen.queryByTestId("assignment-seat")).not.toBeInTheDocument()
    expect(screen.getByText("—")).toBeInTheDocument()
  })

  it("appends pending-add drafts after saved chips so saved chips keep their seat", () => {
    const slotsWithTwoSaved: AssignmentBoardSlot[] = [
      {
        slot: { id: 1, weekday: 1, start_time: "09:00", end_time: "11:00" },
        positions: [
          {
            position: { id: 102, name: "Assistant" },
            required_headcount: 3,
            assignments: [
              {
                assignment_id: 30,
                user_id: 11,
                name: "Assistant One",
                email: "assistant-one@example.com",
              },
              {
                assignment_id: 31,
                user_id: 12,
                name: "Assistant Two",
                email: "assistant-two@example.com",
              },
            ],
          },
        ],
      },
    ]

    const draftState: DraftState = enqueueAdd(
      emptyDraftState,
      {
        userID: 13,
        name: "Late Joiner",
        email: "late@example.com",
      },
      {
        slotID: 1,
        weekday: 1,
        positionID: 102,
        isUnqualified: false,
      },
    )

    const projectedSlots = applyDraftToSlots(slotsWithTwoSaved, draftState)
    const cell = pivotIntoGridCells(projectedSlots).cells[0][0]

    render(
      <DndContext>
        <AssignmentBoardCell
          cell={cell}
          serverSlots={slotsWithTwoSaved}
          renderDraftState={draftState}
          disabled={false}
          isReadOnly={false}
          draggingUserID={null}
          directory={
            new Map<number, Employee>([
              [
                11,
                {
                  user_id: 11,
                  name: "Assistant One",
                  email: "assistant-one@example.com",
                  position_ids: new Set([102]),
                  submittedSlots: new Set(["1:1"]),
                },
              ],
              [
                12,
                {
                  user_id: 12,
                  name: "Assistant Two",
                  email: "assistant-two@example.com",
                  position_ids: new Set([102]),
                  submittedSlots: new Set(["1:1"]),
                },
              ],
              [
                13,
                {
                  user_id: 13,
                  name: "Late Joiner",
                  email: "late@example.com",
                  position_ids: new Set([102]),
                  submittedSlots: new Set(["1:1"]),
                },
              ],
            ])
          }
          onUnassignClick={vi.fn()}
          onCancelDraft={vi.fn()}
        />
      </DndContext>,
    )

    const seats = screen.getAllByTestId("assignment-seat")
    // Three seats (required_headcount = 3); saved chips first, draft last.
    expect(seats[0]).toHaveTextContent("Assistant One")
    expect(seats[1]).toHaveTextContent("Assistant Two")
    expect(seats[2]).toHaveTextContent("Late Joiner")
    // The draft chip carries the dot indicator; saved chips do not.
    expect(seats[2]).toContainElement(screen.getByTestId("assignment-draft-dot"))
  })
})

function renderCell(
  slots: AssignmentBoardSlot[],
  rowIndex: number,
  weekdayIndex: number,
) {
  const cell = pivotIntoGridCells(slots).cells[rowIndex][weekdayIndex]

  return render(
    <DndContext>
      <AssignmentBoardCell
        cell={cell}
        serverSlots={slots}
        renderDraftState={emptyDraftState}
        disabled={false}
        isReadOnly={false}
        draggingUserID={null}
        directory={directory}
        onUnassignClick={vi.fn()}
        onCancelDraft={vi.fn()}
      />
    </DndContext>,
  )
}
