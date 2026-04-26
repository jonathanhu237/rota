import { describe, expect, it } from "vitest"

import type { QualifiedShift } from "@/lib/types"

import { groupQualifiedShiftsByWeekday } from "./group-qualified-shifts"

describe("groupQualifiedShiftsByWeekday", () => {
  it("groups shifts by weekday and sorts by start time", () => {
    const shifts: QualifiedShift[] = [
      {
        slot_id: 23,
        position_id: 103,
        weekday: 3,
        start_time: "11:00",
        end_time: "13:00",
        required_headcount: 1,
      },
      {
        slot_id: 22,
        position_id: 102,
        weekday: 1,
        start_time: "12:00",
        end_time: "14:00",
        required_headcount: 1,
      },
      {
        slot_id: 21,
        position_id: 101,
        weekday: 1,
        start_time: "09:00",
        end_time: "11:00",
        required_headcount: 2,
      },
    ]

    expect(groupQualifiedShiftsByWeekday(shifts)).toEqual({
      1: [shifts[2], shifts[1]],
      2: [],
      3: [shifts[0]],
      4: [],
      5: [],
      6: [],
      7: [],
    })
  })
})
