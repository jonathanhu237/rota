import type { AssignmentBoardShift } from "@/lib/types"

export function isAssignmentBoardShiftUnderstaffed(shift: AssignmentBoardShift) {
  return shift.assignments.length < shift.shift.required_headcount
}
