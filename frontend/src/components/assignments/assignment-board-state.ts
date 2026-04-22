import type {
  AssignmentBoardCandidate,
  AssignmentBoardShift,
  PublicationState,
} from "@/lib/types"

export function isAssignmentBoardShiftUnderstaffed(shift: AssignmentBoardShift) {
  return shift.assignments.length < shift.shift.required_headcount
}

export function getVisibleNonCandidateQualified(
  shift: AssignmentBoardShift,
  showAllQualified: boolean,
): AssignmentBoardCandidate[] {
  if (!showAllQualified) {
    return []
  }

  const excludedUserIDs = new Set<number>()
  for (const candidate of shift.candidates) {
    excludedUserIDs.add(candidate.user_id)
  }
  for (const assignment of shift.assignments) {
    excludedUserIDs.add(assignment.user_id)
  }

  return shift.non_candidate_qualified.filter(
    (candidate) => !excludedUserIDs.has(candidate.user_id),
  )
}

export function isAssignmentBoardMutable(state: PublicationState) {
  return (
    state === "ASSIGNING" || state === "PUBLISHED" || state === "ACTIVE"
  )
}
