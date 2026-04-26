import type { TemplateSlot } from "@/lib/types"

export function sortTemplateSlots(slots: TemplateSlot[]) {
  return [...slots].sort((left, right) => {
    if (left.start_time !== right.start_time) {
      return left.start_time.localeCompare(right.start_time)
    }

    if (left.end_time !== right.end_time) {
      return left.end_time.localeCompare(right.end_time)
    }

    return left.id - right.id
  })
}
