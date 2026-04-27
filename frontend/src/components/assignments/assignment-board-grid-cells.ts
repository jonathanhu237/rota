import type { AssignmentBoardPosition, AssignmentBoardSlot } from "@/lib/types"

export const assignmentBoardWeekdays = [1, 2, 3, 4, 5, 6, 7] as const

export type AssignmentBoardWeekday = (typeof assignmentBoardWeekdays)[number]

export type TimeBlock = {
  index: number
  start_time: string
  end_time: string
}

export type GridCellTotals = {
  assigned: number
  required: number
  status: "full" | "partial" | "empty"
}

export type ScheduledGridCell = {
  kind: "scheduled"
  slotID: number
  weekday: number
  timeBlockIndex: number
  slot: AssignmentBoardSlot["slot"]
  totals: GridCellTotals
  positions: AssignmentBoardPosition[]
}

export type OffScheduleGridCell = {
  kind: "off-schedule"
  weekday: number
  timeBlockIndex: number
}

export type GridCell = ScheduledGridCell | OffScheduleGridCell

export function getGridCellKey(slotID: number, weekday: number) {
  return `${slotID}:${weekday}`
}

export function pivotIntoGridCells(slots: AssignmentBoardSlot[]): {
  timeBlocks: TimeBlock[]
  weekdays: number[]
  cells: GridCell[][]
} {
  const timeBlockKeys = new Set<string>()
  for (const entry of slots) {
    timeBlockKeys.add(getTimeBlockKey(entry.slot.start_time, entry.slot.end_time))
  }

  const timeBlocks = [...timeBlockKeys]
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

  const slotLookup = new Map<string, AssignmentBoardSlot>()
  for (const entry of slots) {
    slotLookup.set(
      getCellLookupKey(
        entry.slot.start_time,
        entry.slot.end_time,
        entry.slot.weekday,
      ),
      entry,
    )
  }

  const weekdays = [...assignmentBoardWeekdays]
  const cells = timeBlocks.map((timeBlock) =>
    weekdays.map((weekday): GridCell => {
      const slotEntry = slotLookup.get(
        getCellLookupKey(timeBlock.start_time, timeBlock.end_time, weekday),
      )

      if (!slotEntry) {
        return {
          kind: "off-schedule",
          weekday,
          timeBlockIndex: timeBlock.index,
        }
      }

      return {
        kind: "scheduled",
        slotID: slotEntry.slot.id,
        weekday,
        timeBlockIndex: timeBlock.index,
        slot: slotEntry.slot,
        totals: getCellTotals(slotEntry.positions),
        positions: slotEntry.positions,
      }
    }),
  )

  return { timeBlocks, weekdays, cells }
}

function getCellTotals(positions: AssignmentBoardPosition[]): GridCellTotals {
  const assigned = positions.reduce(
    (total, position) => total + position.assignments.length,
    0,
  )
  const required = positions.reduce(
    (total, position) => total + position.required_headcount,
    0,
  )

  return {
    assigned,
    required,
    status:
      assigned === 0 ? "empty" : assigned === required ? "full" : "partial",
  }
}

function getTimeBlockKey(startTime: string, endTime: string) {
  return `${startTime}|${endTime}`
}

function getCellLookupKey(startTime: string, endTime: string, weekday: number) {
  return `${getTimeBlockKey(startTime, endTime)}|${weekday}`
}
