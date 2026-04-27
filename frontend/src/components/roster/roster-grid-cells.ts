import type {
  PublicationSlot,
  RosterPosition,
  RosterWeekday,
} from "@/lib/types"

export const rosterWeekdays = [1, 2, 3, 4, 5, 6, 7] as const

export type RosterGridWeekday = (typeof rosterWeekdays)[number]

export type RosterTimeBlock = {
  index: number
  start_time: string
  end_time: string
}

export type RosterCellTotals = {
  assigned: number
  required: number
  status: "full" | "partial" | "empty"
}

export type ScheduledRosterCell = {
  kind: "scheduled"
  weekday: number
  timeBlockIndex: number
  slot: PublicationSlot
  occurrence_date: string
  positions: RosterPosition[]
  totals: RosterCellTotals
}

export type OffScheduleRosterCell = {
  kind: "off-schedule"
  weekday: number
  timeBlockIndex: number
}

export type RosterCell = ScheduledRosterCell | OffScheduleRosterCell

export function pivotRosterIntoGridCells(weekdays: RosterWeekday[]): {
  timeBlocks: RosterTimeBlock[]
  weekdays: number[]
  cells: RosterCell[][]
} {
  const timeBlockKeys = new Set<string>()
  for (const day of weekdays) {
    for (const slotEntry of day.slots) {
      timeBlockKeys.add(
        getTimeBlockKey(slotEntry.slot.start_time, slotEntry.slot.end_time),
      )
    }
  }

  const timeBlocks: RosterTimeBlock[] = [...timeBlockKeys]
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

  const slotLookup = new Map<string, { weekday: number; entry: RosterWeekday["slots"][number] }>()
  for (const day of weekdays) {
    for (const slotEntry of day.slots) {
      slotLookup.set(
        getCellLookupKey(
          slotEntry.slot.start_time,
          slotEntry.slot.end_time,
          day.weekday,
        ),
        { weekday: day.weekday, entry: slotEntry },
      )
    }
  }

  const weekdayList = [...rosterWeekdays]
  const cells = timeBlocks.map((timeBlock) =>
    weekdayList.map((weekday): RosterCell => {
      const match = slotLookup.get(
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
        kind: "scheduled",
        weekday,
        timeBlockIndex: timeBlock.index,
        slot: match.entry.slot,
        occurrence_date: match.entry.occurrence_date,
        positions: match.entry.positions,
        totals: getCellTotals(match.entry.positions),
      }
    }),
  )

  return { timeBlocks, weekdays: weekdayList, cells }
}

function getCellTotals(positions: RosterPosition[]): RosterCellTotals {
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
      assigned === 0 ? "empty" : assigned >= required ? "full" : "partial",
  }
}

function getTimeBlockKey(startTime: string, endTime: string) {
  return `${startTime}|${endTime}`
}

function getCellLookupKey(startTime: string, endTime: string, weekday: number) {
  return `${getTimeBlockKey(startTime, endTime)}|${weekday}`
}
