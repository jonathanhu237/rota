import { describe, expect, it } from "vitest"

import type { QualifiedShift } from "@/lib/types"

import { groupQualifiedShiftsByWeekday } from "./group-qualified-shifts"

describe("groupQualifiedShiftsByWeekday", () => {
  it("groups shifts by weekday and sorts by start time", () => {
    const shifts: QualifiedShift[] = [
      {
        slot_id: 23,
        weekday: 3,
        start_time: "11:00",
        end_time: "13:00",
        composition: [
          {
            position_id: 103,
            position_name: "Stock",
            required_headcount: 1,
          },
        ],
      },
      {
        slot_id: 22,
        weekday: 1,
        start_time: "12:00",
        end_time: "14:00",
        composition: [
          {
            position_id: 102,
            position_name: "Cashier",
            required_headcount: 1,
          },
        ],
      },
      {
        slot_id: 21,
        weekday: 1,
        start_time: "09:00",
        end_time: "11:00",
        composition: [
          {
            position_id: 101,
            position_name: "Front Desk",
            required_headcount: 2,
          },
        ],
      },
      {
        slot_id: 21,
        weekday: 1,
        start_time: "09:00",
        end_time: "11:00",
        composition: [
          {
            position_id: 100,
            position_name: "Lead",
            required_headcount: 1,
          },
        ],
      },
    ]

    const grouped = groupQualifiedShiftsByWeekday(shifts)

    expect(grouped).toEqual({
      1: [
        {
          ...shifts[2],
          composition: [shifts[3].composition[0], shifts[2].composition[0]],
        },
        shifts[1],
      ],
      2: [],
      3: [shifts[0]],
      4: [],
      5: [],
      6: [],
      7: [],
    })
  })
})
