import type { QualifiedShift } from "@/lib/types"

export function groupQualifiedShiftsByWeekday(shifts: QualifiedShift[]) {
  const grouped: Record<number, QualifiedShift[]> = {
    1: [],
    2: [],
    3: [],
    4: [],
    5: [],
    6: [],
    7: [],
  }

  for (const shift of shifts) {
    if (!(shift.weekday in grouped)) {
      continue
    }

    grouped[shift.weekday].push(shift)
  }

  for (const weekday of Object.keys(grouped)) {
    grouped[Number(weekday)].sort((left, right) => {
      if (left.start_time !== right.start_time) {
        return left.start_time.localeCompare(right.start_time)
      }

      if (left.slot_id !== right.slot_id) {
        return left.slot_id - right.slot_id
      }

      return left.position_id - right.position_id
    })
  }

  return grouped
}
