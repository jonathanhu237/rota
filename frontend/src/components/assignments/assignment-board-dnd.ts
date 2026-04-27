import {
  enqueueAdd,
  enqueueMove,
  enqueueRemove,
  removeDraftOp,
  type DraftAssignmentInput,
  type DraftState,
  type DraftUserInput,
  type ProjectedAssignment,
} from "@/components/assignments/draft-state"
import type {
  AssignmentBoardCandidate,
  AssignmentBoardSlot,
} from "@/lib/types"

export type AssignmentBoardSelection = {
  slotID: number
  weekday: number
}

export type AssignmentBoardDragSource =
  | {
      kind: "assigned"
      assignment: ProjectedAssignment
      slotID: number
      weekday: number
      positionID: number
    }
  | {
      kind: "candidate"
      candidate: AssignmentBoardCandidate
      slotID: number
      weekday: number
      positionID: number
    }

export type AssignmentBoardDropTarget = {
  kind: "cell"
  slotID: number
  weekday: number
}

export function resolveAssignmentBoardDrop({
  slots,
  draftState,
  selection,
  source,
  target,
}: {
  slots: AssignmentBoardSlot[]
  draftState: DraftState
  selection: AssignmentBoardSelection | null
  source: AssignmentBoardDragSource
  target: AssignmentBoardDropTarget
}) {
  const targetSlot = findBoardSlot(slots, target.slotID, target.weekday)
  if (!targetSlot) {
    return draftState
  }

  const targetCell = getTargetCell(slots, target, source)

  if (
    selection?.slotID === target.slotID &&
    selection.weekday === target.weekday
  ) {
    return stageSourceAsClick(draftState, source, targetCell)
  }

  if (source.kind === "assigned") {
    return enqueueMove(
      draftState,
      assignmentToDraftInput(
        source.assignment,
        source.slotID,
        source.weekday,
        source.positionID,
      ),
      targetCell,
    )
  }

  return enqueueAdd(draftState, candidateToDraftUser(source.candidate), targetCell)
}

function stageSourceAsClick(
  draftState: DraftState,
  source: AssignmentBoardDragSource,
  targetCell: {
    slotID: number
    weekday: number
    positionID: number
    isUnqualified: boolean
  },
) {
  if (source.kind === "candidate") {
    return enqueueAdd(draftState, candidateToDraftUser(source.candidate), {
      ...targetCell,
      positionID: source.positionID,
    })
  }

  if (source.assignment.draftOpID) {
    return removeDraftOp(draftState, source.assignment.draftOpID)
  }

  return enqueueRemove(
    draftState,
    assignmentToDraftInput(
      source.assignment,
      source.slotID,
      source.weekday,
      source.positionID,
    ),
  )
}

function getTargetCell(
  slots: AssignmentBoardSlot[],
  target: AssignmentBoardDropTarget,
  source: AssignmentBoardDragSource,
) {
  const userID =
    source.kind === "assigned"
      ? source.assignment.user_id
      : source.candidate.user_id
  const hasPosition = cellHasPosition(
    slots,
    target.slotID,
    target.weekday,
    source.positionID,
  )

  return {
    slotID: target.slotID,
    weekday: target.weekday,
    positionID: source.positionID,
    isUnqualified:
      !hasPosition ||
      !isUserQualifiedForCell(
        slots,
        target.slotID,
        target.weekday,
        source.positionID,
        userID,
      ),
  }
}

function assignmentToDraftInput(
  assignment: ProjectedAssignment,
  slotID: number,
  weekday: number,
  positionID: number,
): DraftAssignmentInput {
  return {
    assignmentID: assignment.assignment_id,
    userID: assignment.user_id,
    name: assignment.name,
    email: assignment.email,
    slotID,
    weekday,
    positionID,
    sourceOpID: assignment.draftOpID,
  }
}

function candidateToDraftUser(
  candidate: AssignmentBoardCandidate,
): DraftUserInput {
  return {
    userID: candidate.user_id,
    name: candidate.name,
    email: candidate.email,
  }
}

function findBoardSlot(
  slots: AssignmentBoardSlot[],
  slotID: number,
  weekday: number,
) {
  return slots.find(
    (entry) => entry.slot.id === slotID && entry.slot.weekday === weekday,
  )
}

function cellHasPosition(
  slots: AssignmentBoardSlot[],
  slotID: number,
  weekday: number,
  positionID: number,
) {
  return Boolean(findBoardPosition(slots, slotID, weekday, positionID))
}

function findBoardPosition(
  slots: AssignmentBoardSlot[],
  slotID: number,
  weekday: number,
  positionID: number,
) {
  return findBoardSlot(slots, slotID, weekday)?.positions.find(
    (entry) => entry.position.id === positionID,
  )
}

function isUserQualifiedForCell(
  slots: AssignmentBoardSlot[],
  slotID: number,
  weekday: number,
  positionID: number,
  userID: number,
) {
  const positionEntry = findBoardPosition(
    slots,
    slotID,
    weekday,
    positionID,
  )
  if (!positionEntry) {
    return false
  }

  return [
    ...positionEntry.candidates,
    ...positionEntry.non_candidate_qualified,
    ...positionEntry.assignments,
  ].some((candidate) => candidate.user_id === userID)
}
