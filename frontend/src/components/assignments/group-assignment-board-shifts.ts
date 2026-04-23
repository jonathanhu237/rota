import type { AssignmentBoardSlot } from "@/lib/types"

export const assignmentBoardWeekdays = [1, 2, 3, 4, 5, 6, 7] as const

export function groupAssignmentBoardSlotsByWeekday(
  slots: AssignmentBoardSlot[],
) {
  const grouped: Record<number, AssignmentBoardSlot[]> = {
    1: [],
    2: [],
    3: [],
    4: [],
    5: [],
    6: [],
    7: [],
  }

  for (const entry of slots) {
    if (!(entry.slot.weekday in grouped)) {
      continue
    }

    grouped[entry.slot.weekday].push(entry)
  }

  for (const weekday of Object.keys(grouped)) {
    grouped[Number(weekday)].sort((left, right) => {
      if (left.slot.start_time !== right.slot.start_time) {
        return left.slot.start_time.localeCompare(right.slot.start_time)
      }

      return left.slot.id - right.slot.id
    })
  }

  return grouped
}
