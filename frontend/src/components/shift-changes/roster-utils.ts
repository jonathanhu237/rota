import type { RosterShift, RosterWeekday } from "@/lib/types"

export type MemberShiftOption = {
  assignmentID: number
  weekday: number
  shift: RosterShift["shift"]
}

/**
 * Returns every shift in the roster that the given user is currently
 * assigned to, flattened and sorted by weekday then start time.
 *
 * The roster response already carries each assignment (with its
 * assignment_id) grouped by weekday and shift, so we can derive the
 * per-user shift list purely client-side without another API call.
 */
export function findShiftsForMember(
  rosterWeekdays: RosterWeekday[],
  userID: number,
): MemberShiftOption[] {
  const options: MemberShiftOption[] = []

  for (const weekday of rosterWeekdays) {
    for (const shift of weekday.shifts) {
      for (const assignment of shift.assignments) {
        if (assignment.user_id === userID) {
          options.push({
            assignmentID: assignment.assignment_id,
            weekday: weekday.weekday,
            shift: shift.shift,
          })
        }
      }
    }
  }

  options.sort((a, b) => {
    if (a.weekday !== b.weekday) {
      return a.weekday - b.weekday
    }
    return a.shift.start_time.localeCompare(b.shift.start_time)
  })

  return options
}
