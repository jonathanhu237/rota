import { describe, expect, it } from "vitest"

import type { AssignmentBoardSlot } from "@/lib/types"

import { pivotIntoGridCells } from "./assignment-board-grid-cells"

const slots: AssignmentBoardSlot[] = [
  {
    slot: {
      id: 2,
      weekday: 2,
      start_time: "13:00",
      end_time: "15:00",
    },
    positions: [
      {
        position: { id: 102, name: "Kitchen" },
        required_headcount: 1,
        assignments: [],
      },
    ],
  },
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
    ],
  },
]

describe("pivotIntoGridCells", () => {
  it("returns one sorted row per distinct time block", () => {
    const grid = pivotIntoGridCells(slots)

    expect(grid.timeBlocks.map((block) => block.start_time)).toEqual([
      "09:00",
      "13:00",
    ])
    expect(grid.cells).toHaveLength(2)
    expect(grid.weekdays).toEqual([1, 2, 3, 4, 5, 6, 7])
  })

  it("marks weekdays outside a slot schedule as off-schedule cells", () => {
    const grid = pivotIntoGridCells(slots)

    expect(grid.cells[0][0]).toMatchObject({
      kind: "scheduled",
      slotID: 1,
      weekday: 1,
    })
    expect(grid.cells[0][1]).toMatchObject({
      kind: "off-schedule",
      weekday: 2,
      timeBlockIndex: 0,
    })
  })

  it("computes totals and status from assignments and required headcount", () => {
    const grid = pivotIntoGridCells(slots)

    expect(grid.cells[0][0]).toMatchObject({
      kind: "scheduled",
      totals: {
        assigned: 1,
        required: 2,
        status: "partial",
      },
    })
    expect(grid.cells[1][1]).toMatchObject({
      kind: "scheduled",
      totals: {
        assigned: 0,
        required: 1,
        status: "empty",
      },
    })
  })
})
