import type { AssignmentBoardEmployee } from "@/lib/types"

export type Employee = {
  user_id: number
  name: string
  email: string
  position_ids: Set<number>
  submittedSlots: Set<string>
}

export type DirectoryStats = {
  total: number
  min: number
  avg: number
  max: number
  stddev: number
  zeroCount: number
}

export function deriveEmployeeDirectory(
  employees: AssignmentBoardEmployee[],
): Map<number, Employee> {
  const directory = new Map<number, Employee>()

  for (const employee of employees) {
    directory.set(employee.user_id, {
      user_id: employee.user_id,
      name: employee.name,
      email: employee.email,
      position_ids: new Set(employee.position_ids),
      submittedSlots: new Set(
        employee.submitted_slots.map((slot) =>
          slotWeekdayKey(slot.slot_id, slot.weekday),
        ),
      ),
    })
  }

  return directory
}

export function slotWeekdayKey(slotID: number, weekday: number) {
  return `${slotID}:${weekday}`
}

export function computeDirectoryStats(hours: number[]): DirectoryStats {
  if (hours.length === 0) {
    return { total: 0, min: 0, avg: 0, max: 0, stddev: 0, zeroCount: 0 }
  }

  let min = hours[0]
  let max = hours[0]
  let sum = 0
  let zeroCount = 0
  for (const value of hours) {
    if (value < min) min = value
    if (value > max) max = value
    sum += value
    if (value === 0) zeroCount += 1
  }

  const avg = sum / hours.length
  let varianceSum = 0
  for (const value of hours) {
    const diff = value - avg
    varianceSum += diff * diff
  }
  const stddev = Math.sqrt(varianceSum / hours.length)

  return { total: hours.length, min, avg, max, stddev, zeroCount }
}
