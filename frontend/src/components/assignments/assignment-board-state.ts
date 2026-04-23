import type {
  AssignmentBoardCandidate,
  AssignmentBoardPosition,
  AssignmentBoardSlot,
  PublicationState,
} from "@/lib/types"

export function isAssignmentBoardPositionUnderstaffed(
  position: Pick<AssignmentBoardPosition, "required_headcount" | "assignments">,
) {
  return position.assignments.length < position.required_headcount
}

export function getVisibleNonCandidateQualified(
  _slot: AssignmentBoardSlot,
  position: AssignmentBoardPosition,
  showAllQualified: boolean,
): AssignmentBoardCandidate[] {
  if (!showAllQualified) {
    return []
  }

  const excludedUserIDs = new Set<number>()
  for (const candidate of position.candidates) {
    excludedUserIDs.add(candidate.user_id)
  }
  for (const assignment of position.assignments) {
    excludedUserIDs.add(assignment.user_id)
  }

  return position.non_candidate_qualified.filter(
    (candidate) => !excludedUserIDs.has(candidate.user_id),
  )
}

export function isAssignmentBoardMutable(state: PublicationState) {
  return (
    state === "ASSIGNING" || state === "PUBLISHED" || state === "ACTIVE"
  )
}
