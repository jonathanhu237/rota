import { describe, expect, it } from "vitest"

import type { QualifiedShift } from "@/lib/types"

import {
  availabilityWeekdays,
  pivotAvailabilityIntoGridCells,
} from "./availability-grid-cells"

const frontDesk = {
  position_id: 101,
  position_name: "Front Desk",
  required_headcount: 2,
}
const cashier = {
  position_id: 102,
  position_name: "Cashier",
  required_headcount: 1,
}

function shift(
  slot_id: number,
  weekday: number,
  start_time: string,
  end_time: string,
): QualifiedShift {
  return {
    slot_id,
    weekday,
    start_time,
    end_time,
    composition: [frontDesk, cashier],
  }
}

describe("pivotAvailabilityIntoGridCells", () => {
  it("collects distinct time blocks sorted ascending by start and end time", () => {
    const result = pivotAvailabilityIntoGridCells([
      shift(3, 3, "12:00", "14:00"),
      shift(1, 1, "09:00", "11:00"),
      shift(2, 2, "09:00", "10:00"),
    ])

    expect(result.timeBlocks.map((block) => `${block.start_time}-${block.end_time}`)).toEqual([
      "09:00-10:00",
      "09:00-11:00",
      "12:00-14:00",
    ])
    expect(result.weekdays).toEqual([...availabilityWeekdays])
    expect(result.cells).toHaveLength(3)
    expect(result.cells[0]).toHaveLength(7)
  })

  it("emits off-schedule cells when a weekday has no matching qualified shift", () => {
    const result = pivotAvailabilityIntoGridCells([
      shift(21, 1, "09:00", "11:00"),
    ])

    expect(result.cells[0][0]).toEqual({
      kind: "qualified",
      weekday: 1,
      timeBlockIndex: 0,
      slot_id: 21,
      composition: [frontDesk, cashier],
    })

    for (let index = 1; index < 7; index++) {
      expect(result.cells[0][index]).toEqual({
        kind: "off-schedule",
        weekday: index + 1,
        timeBlockIndex: 0,
      })
    }
  })

  it("returns no time blocks or cells for empty input", () => {
    const result = pivotAvailabilityIntoGridCells([])

    expect(result.timeBlocks).toEqual([])
    expect(result.cells).toEqual([])
    expect(result.weekdays).toEqual([...availabilityWeekdays])
  })
})
