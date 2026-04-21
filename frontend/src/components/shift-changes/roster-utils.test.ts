import { describe, expect, it } from "vitest"

import type { RosterWeekday } from "@/lib/types"

import { findShiftsForMember } from "./roster-utils"

const sampleWeekdays: RosterWeekday[] = [
  {
    weekday: 1,
    shifts: [
      {
        shift: {
          id: 101,
          weekday: 1,
          start_time: "09:00",
          end_time: "12:00",
          position_id: 1,
          position_name: "Front Desk",
          required_headcount: 2,
        },
        assignments: [
          { assignment_id: 11, user_id: 7, name: "Alice" },
          { assignment_id: 12, user_id: 8, name: "Bob" },
        ],
      },
      {
        shift: {
          id: 102,
          weekday: 1,
          start_time: "13:00",
          end_time: "17:00",
          position_id: 1,
          position_name: "Front Desk",
          required_headcount: 1,
        },
        assignments: [{ assignment_id: 13, user_id: 7, name: "Alice" }],
      },
    ],
  },
  {
    weekday: 3,
    shifts: [
      {
        shift: {
          id: 103,
          weekday: 3,
          start_time: "09:00",
          end_time: "12:00",
          position_id: 2,
          position_name: "Back Office",
          required_headcount: 1,
        },
        assignments: [{ assignment_id: 14, user_id: 8, name: "Bob" }],
      },
    ],
  },
]

describe("findShiftsForMember", () => {
  it("returns every assignment for the requested user, sorted by weekday then start time", () => {
    const result = findShiftsForMember(sampleWeekdays, 7)

    expect(result.map((o) => o.assignmentID)).toEqual([11, 13])
    expect(result[0].weekday).toBe(1)
    expect(result[0].shift.start_time).toBe("09:00")
    expect(result[1].shift.start_time).toBe("13:00")
  })

  it("returns an empty list when the user has no assignments", () => {
    expect(findShiftsForMember(sampleWeekdays, 999)).toEqual([])
  })

  it("returns shifts across multiple weekdays in weekday order", () => {
    const result = findShiftsForMember(sampleWeekdays, 8)

    expect(result.map((o) => o.assignmentID)).toEqual([12, 14])
    expect(result.map((o) => o.weekday)).toEqual([1, 3])
  })
})
