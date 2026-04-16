import type { AssignmentBoardShift } from "@/lib/types"

export const assignmentBoardWeekdays = [1, 2, 3, 4, 5, 6, 7] as const

export function groupAssignmentBoardShiftsByWeekday(
  shifts: AssignmentBoardShift[],
) {
  const grouped: Record<number, AssignmentBoardShift[]> = {
    1: [],
    2: [],
    3: [],
    4: [],
    5: [],
    6: [],
    7: [],
  }

  for (const entry of shifts) {
    if (!(entry.shift.weekday in grouped)) {
      continue
    }

    grouped[entry.shift.weekday].push(entry)
  }

  for (const weekday of Object.keys(grouped)) {
    grouped[Number(weekday)].sort((left, right) => {
      if (left.shift.start_time !== right.shift.start_time) {
        return left.shift.start_time.localeCompare(right.shift.start_time)
      }

      return left.shift.id - right.shift.id
    })
  }

  return grouped
}
