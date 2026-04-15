import { describe, expect, it } from "vitest"

import type { TemplateShift } from "@/lib/types"

import { groupTemplateShiftsByWeekday } from "./group-template-shifts"

describe("groupTemplateShiftsByWeekday", () => {
  it("groups shifts by weekday and sorts by start time", () => {
    const shifts: TemplateShift[] = [
      {
        id: 3,
        template_id: 1,
        weekday: 3,
        start_time: "11:00",
        end_time: "13:00",
        position_id: 1,
        required_headcount: 1,
        created_at: "",
        updated_at: "",
      },
      {
        id: 2,
        template_id: 1,
        weekday: 1,
        start_time: "12:00",
        end_time: "14:00",
        position_id: 2,
        required_headcount: 1,
        created_at: "",
        updated_at: "",
      },
      {
        id: 1,
        template_id: 1,
        weekday: 1,
        start_time: "09:00",
        end_time: "11:00",
        position_id: 1,
        required_headcount: 2,
        created_at: "",
        updated_at: "",
      },
    ]

    expect(groupTemplateShiftsByWeekday(shifts)).toEqual({
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
