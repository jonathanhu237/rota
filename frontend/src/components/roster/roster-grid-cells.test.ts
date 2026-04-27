import { describe, expect, it } from "vitest"

import type {
  PublicationPosition,
  PublicationSlot,
  RosterAssignment,
  RosterWeekday,
} from "@/lib/types"

import {
  pivotRosterIntoGridCells,
  rosterWeekdays,
} from "./roster-grid-cells"

const positionA: PublicationPosition = { id: 1, name: "前台负责人" }
const positionB: PublicationPosition = { id: 2, name: "前台助理" }

function slot(
  id: number,
  weekday: number,
  start_time: string,
  end_time: string,
): PublicationSlot {
  return { id, weekday, start_time, end_time }
}

function assignment(
  assignment_id: number,
  user_id: number,
  name: string,
): RosterAssignment {
  return { assignment_id, user_id, name }
}

describe("pivotRosterIntoGridCells", () => {
  it("collects distinct time blocks across weekdays sorted ascending", () => {
    const weekdays: RosterWeekday[] = [
      {
        weekday: 1,
        slots: [
          {
            slot: slot(1, 1, "09:00", "10:00"),
            occurrence_date: "2026-04-27",
            positions: [],
          },
          {
            slot: slot(2, 1, "10:00", "12:00"),
            occurrence_date: "2026-04-27",
            positions: [],
          },
        ],
      },
      {
        weekday: 2,
        slots: [
          {
            slot: slot(3, 2, "09:00", "10:00"),
            occurrence_date: "2026-04-28",
            positions: [],
          },
        ],
      },
    ]

    const result = pivotRosterIntoGridCells(weekdays)

    expect(result.timeBlocks.map((b) => `${b.start_time}-${b.end_time}`)).toEqual([
      "09:00-10:00",
      "10:00-12:00",
    ])
    expect(result.weekdays).toEqual([...rosterWeekdays])
    expect(result.cells).toHaveLength(2)
    expect(result.cells[0]).toHaveLength(7)
  })

  it("emits off-schedule for (time, weekday) pairs absent from the input", () => {
    const weekdays: RosterWeekday[] = [
      {
        weekday: 1,
        slots: [
          {
            slot: slot(1, 1, "09:00", "10:00"),
            occurrence_date: "2026-04-27",
            positions: [],
          },
        ],
      },
    ]

    const result = pivotRosterIntoGridCells(weekdays)

    // (09:00-10:00, weekday 1) is scheduled
    expect(result.cells[0][0].kind).toBe("scheduled")
    // (09:00-10:00, every other weekday) is off-schedule
    for (let i = 1; i < 7; i++) {
      expect(result.cells[0][i].kind).toBe("off-schedule")
    }
  })

  it("classifies cell totals as full / partial / empty", () => {
    const weekdays: RosterWeekday[] = [
      {
        weekday: 1,
        slots: [
          {
            slot: slot(1, 1, "09:00", "10:00"),
            occurrence_date: "2026-04-27",
            positions: [
              {
                position: positionA,
                required_headcount: 1,
                assignments: [assignment(100, 10, "Alice")],
              },
              {
                position: positionB,
                required_headcount: 2,
                assignments: [assignment(101, 11, "Bob")],
              },
            ],
          },
        ],
      },
      {
        weekday: 2,
        slots: [
          {
            slot: slot(2, 2, "09:00", "10:00"),
            occurrence_date: "2026-04-28",
            positions: [
              {
                position: positionA,
                required_headcount: 1,
                assignments: [assignment(102, 10, "Alice")],
              },
              {
                position: positionB,
                required_headcount: 2,
                assignments: [
                  assignment(103, 11, "Bob"),
                  assignment(104, 12, "Carol"),
                ],
              },
            ],
          },
        ],
      },
      {
        weekday: 3,
        slots: [
          {
            slot: slot(3, 3, "09:00", "10:00"),
            occurrence_date: "2026-04-29",
            positions: [
              {
                position: positionA,
                required_headcount: 1,
                assignments: [],
              },
              {
                position: positionB,
                required_headcount: 2,
                assignments: [],
              },
            ],
          },
        ],
      },
    ]

    const result = pivotRosterIntoGridCells(weekdays)

    const monday = result.cells[0][0]
    const tuesday = result.cells[0][1]
    const wednesday = result.cells[0][2]

    expect(monday.kind).toBe("scheduled")
    if (monday.kind === "scheduled") {
      expect(monday.totals).toEqual({
        assigned: 2,
        required: 3,
        status: "partial",
      })
    }

    expect(tuesday.kind).toBe("scheduled")
    if (tuesday.kind === "scheduled") {
      expect(tuesday.totals).toEqual({
        assigned: 3,
        required: 3,
        status: "full",
      })
    }

    expect(wednesday.kind).toBe("scheduled")
    if (wednesday.kind === "scheduled") {
      expect(wednesday.totals).toEqual({
        assigned: 0,
        required: 3,
        status: "empty",
      })
    }
  })

  it("returns empty grid when no weekdays carry slots", () => {
    const result = pivotRosterIntoGridCells([])

    expect(result.timeBlocks).toEqual([])
    expect(result.cells).toEqual([])
    expect(result.weekdays).toEqual([...rosterWeekdays])
  })
})
