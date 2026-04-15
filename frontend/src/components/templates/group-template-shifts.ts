import type { TemplateShift } from "@/lib/types"

export function groupTemplateShiftsByWeekday(shifts: TemplateShift[]) {
  const grouped: Record<number, TemplateShift[]> = {
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

      return left.id - right.id
    })
  }

  return grouped
}
