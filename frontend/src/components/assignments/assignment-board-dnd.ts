import {
  applyDraftToBoard,
  enqueueAdd,
  enqueueMove,
  enqueueReplace,
  enqueueSwap,
  getBoardCellKey,
  type DraftAssignmentInput,
  type DraftState,
  type DraftUserInput,
  type ProjectedAssignment,
} from "@/components/assignments/draft-state"
import type {
  AssignmentBoardCandidate,
  AssignmentBoardSlot,
} from "@/lib/types"

export type AssignmentBoardDragSource =
  | {
      kind: "assignment"
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

export type AssignmentBoardDropTarget =
  | {
      kind: "cell"
      slotID: number
      weekday: number
      positionID: number
    }
  | {
      kind: "assignment"
      assignment: ProjectedAssignment
      slotID: number
      weekday: number
      positionID: number
    }

export function resolveAssignmentBoardDrop({
  slots,
  draftState,
  source,
  target,
}: {
  slots: AssignmentBoardSlot[]
  draftState: DraftState
  source: AssignmentBoardDragSource
  target: AssignmentBoardDropTarget
}) {
  const projected = applyDraftToBoard(slots, draftState)
  const targetCell = findBoardPosition(
    slots,
    target.slotID,
    target.weekday,
    target.positionID,
  )

  if (!targetCell) {
    return draftState
  }

  const targetAssignments =
    projected.get(
      getBoardCellKey(target.slotID, target.weekday, target.positionID),
    ) ?? []
  const hasOpenHeadcount =
    targetAssignments.length < targetCell.position.required_headcount

  if (source.kind === "assignment") {
    if (
      source.slotID === target.slotID &&
      source.weekday === target.weekday &&
      source.positionID === target.positionID
    ) {
      return draftState
    }

    const dragged = assignmentToDraftInput(
      source.assignment,
      source.slotID,
      source.weekday,
      source.positionID,
    )

    if (target.kind === "assignment" && !hasOpenHeadcount) {
      return enqueueSwap(
        draftState,
        dragged,
        assignmentToDraftInput(
          target.assignment,
          target.slotID,
          target.weekday,
          target.positionID,
        ),
        {
          slotID: target.slotID,
          weekday: target.weekday,
          positionID: target.positionID,
          isUnqualified: !isUserQualifiedForCell(
            slots,
            target.slotID,
            target.weekday,
            target.positionID,
            source.assignment.user_id,
          ),
        },
        {
          slotID: source.slotID,
          weekday: source.weekday,
          positionID: source.positionID,
          isUnqualified: !isUserQualifiedForCell(
            slots,
            source.slotID,
            source.weekday,
            source.positionID,
            target.assignment.user_id,
          ),
        },
      )
    }

    if (!hasOpenHeadcount) {
      return draftState
    }

    return enqueueMove(draftState, dragged, {
      slotID: target.slotID,
      weekday: target.weekday,
      positionID: target.positionID,
      isUnqualified: !isUserQualifiedForCell(
        slots,
        target.slotID,
        target.weekday,
        target.positionID,
        source.assignment.user_id,
      ),
    })
  }

  if (target.kind === "assignment") {
    return enqueueReplace(
      draftState,
      assignmentToDraftInput(
        target.assignment,
        target.slotID,
        target.weekday,
        target.positionID,
      ),
      candidateToDraftUser(source.candidate),
      {
        slotID: target.slotID,
        weekday: target.weekday,
        positionID: target.positionID,
        isUnqualified: !isUserQualifiedForCell(
          slots,
          target.slotID,
          target.weekday,
          target.positionID,
          source.candidate.user_id,
        ),
      },
    )
  }

  if (!hasOpenHeadcount) {
    return draftState
  }

  return enqueueAdd(draftState, candidateToDraftUser(source.candidate), {
    slotID: target.slotID,
    weekday: target.weekday,
    positionID: target.positionID,
    isUnqualified: !isUserQualifiedForCell(
      slots,
      target.slotID,
      target.weekday,
      target.positionID,
      source.candidate.user_id,
    ),
  })
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

function findBoardPosition(
  slots: AssignmentBoardSlot[],
  slotID: number,
  weekday: number,
  positionID: number,
) {
  const slotEntry = slots.find(
    (entry) => entry.slot.id === slotID && entry.slot.weekday === weekday,
  )
  const positionEntry = slotEntry?.positions.find(
    (entry) => entry.position.id === positionID,
  )

  if (!slotEntry || !positionEntry) {
    return null
  }

  return {
    slot: slotEntry,
    position: positionEntry,
  }
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
  )?.position
  if (!positionEntry) {
    return false
  }

  return [
    ...positionEntry.candidates,
    ...positionEntry.non_candidate_qualified,
    ...positionEntry.assignments,
  ].some((candidate) => candidate.user_id === userID)
}
