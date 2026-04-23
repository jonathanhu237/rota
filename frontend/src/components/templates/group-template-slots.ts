import type { TemplateSlot } from "@/lib/types"

export function groupTemplateSlotsByWeekday(slots: TemplateSlot[]) {
  const grouped: Record<number, TemplateSlot[]> = {
    1: [],
    2: [],
    3: [],
    4: [],
    5: [],
    6: [],
    7: [],
  }

  for (const slot of slots) {
    if (!(slot.weekday in grouped)) {
      continue
    }

    grouped[slot.weekday].push(slot)
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
