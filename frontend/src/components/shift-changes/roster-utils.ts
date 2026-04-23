import type {
  PublicationPosition,
  PublicationSlot,
  RosterWeekday,
} from "@/lib/types"

export type MemberShiftOption = {
  assignmentID: number
  weekday: number
  slot: PublicationSlot
  position: PublicationPosition
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
    for (const slot of weekday.slots) {
      for (const position of slot.positions) {
        for (const assignment of position.assignments) {
          if (assignment.user_id === userID) {
            options.push({
              assignmentID: assignment.assignment_id,
              weekday: weekday.weekday,
              slot: slot.slot,
              position: position.position,
            })
          }
        }
      }
    }
  }

  options.sort((a, b) => {
    if (a.weekday !== b.weekday) {
      return a.weekday - b.weekday
    }
    return a.slot.start_time.localeCompare(b.slot.start_time)
  })

  return options
}
