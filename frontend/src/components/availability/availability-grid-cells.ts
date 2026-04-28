import type { QualifiedShift, QualifiedShiftComposition } from "@/lib/types"

export const availabilityWeekdays = [1, 2, 3, 4, 5, 6, 7] as const

export type AvailabilityGridWeekday = (typeof availabilityWeekdays)[number]

export type AvailabilityTimeBlock = {
  index: number
  start_time: string
  end_time: string
}

export type QualifiedAvailabilityCell = {
  kind: "qualified"
  weekday: number
  timeBlockIndex: number
  slot_id: number
  composition: QualifiedShiftComposition[]
}

export type OffScheduleAvailabilityCell = {
  kind: "off-schedule"
  weekday: number
  timeBlockIndex: number
}

export type AvailabilityCell =
  | QualifiedAvailabilityCell
  | OffScheduleAvailabilityCell

export function pivotAvailabilityIntoGridCells(shifts: QualifiedShift[]): {
  timeBlocks: AvailabilityTimeBlock[]
  weekdays: number[]
  cells: AvailabilityCell[][]
} {
  const timeBlockKeys = new Set<string>()
  for (const shift of shifts) {
    timeBlockKeys.add(getTimeBlockKey(shift.start_time, shift.end_time))
  }

  const timeBlocks: AvailabilityTimeBlock[] = [...timeBlockKeys]
    .map((key) => {
      const [start_time, end_time] = key.split("|")
      return { start_time, end_time }
    })
    .sort((left, right) => {
      if (left.start_time !== right.start_time) {
        return left.start_time.localeCompare(right.start_time)
      }

      return left.end_time.localeCompare(right.end_time)
    })
    .map((block, index) => ({ ...block, index }))

  const shiftLookup = new Map<string, QualifiedShift>()
  for (const shift of shifts) {
    shiftLookup.set(
      getCellLookupKey(shift.start_time, shift.end_time, shift.weekday),
      shift,
    )
  }

  const weekdayList = [...availabilityWeekdays]
  const cells = timeBlocks.map((timeBlock) =>
    weekdayList.map((weekday): AvailabilityCell => {
      const match = shiftLookup.get(
        getCellLookupKey(timeBlock.start_time, timeBlock.end_time, weekday),
      )

      if (!match) {
        return {
          kind: "off-schedule",
          weekday,
          timeBlockIndex: timeBlock.index,
        }
      }

      return {
        kind: "qualified",
        weekday,
        timeBlockIndex: timeBlock.index,
        slot_id: match.slot_id,
        composition: match.composition,
      }
    }),
  )

  return { timeBlocks, weekdays: weekdayList, cells }
}

function getTimeBlockKey(startTime: string, endTime: string) {
  return `${startTime}|${endTime}`
}

function getCellLookupKey(startTime: string, endTime: string, weekday: number) {
  return `${getTimeBlockKey(startTime, endTime)}|${weekday}`
}
