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

    const existing = grouped[shift.weekday].find(
      (candidate) => candidate.slot_id === shift.slot_id,
    )
    if (existing) {
      const seenPositions = new Set(
        existing.composition.map((entry) => entry.position_id),
      )
      existing.composition.push(
        ...shift.composition.filter(
          (entry) => !seenPositions.has(entry.position_id),
        ),
      )
      existing.composition.sort((left, right) =>
        left.position_id - right.position_id,
      )
      continue
    }

    grouped[shift.weekday].push({
      ...shift,
      composition: [...shift.composition].sort(
        (left, right) => left.position_id - right.position_id,
      ),
    })
  }

  for (const weekday of Object.keys(grouped)) {
    grouped[Number(weekday)].sort((left, right) => {
      if (left.start_time !== right.start_time) {
        return left.start_time.localeCompare(right.start_time)
      }

      if (left.slot_id !== right.slot_id) {
        return left.slot_id - right.slot_id
      }

      return 0
    })
  }

  return grouped
}
