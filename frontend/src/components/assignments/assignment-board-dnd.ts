import type { Employee } from "@/components/assignments/assignment-board-directory"
import {
  enqueueAdd,
  enqueueRemove,
  type DraftAssignmentInput,
  type DraftState,
  type DraftUserInput,
  type ProjectedAssignment,
} from "@/components/assignments/draft-state"

export type AssignmentBoardDragSource =
  | {
      kind: "directory-employee"
      employee: Employee
    }
  | {
      kind: "assigned"
      assignment: ProjectedAssignment
      slotID: number
      weekday: number
      positionID: number
    }

export type AssignmentBoardDropTarget = {
  kind: "seat"
  slotID: number
  weekday: number
  positionID: number
  headcountIndex: number
  filledBy: ProjectedAssignment | null
  cellUserIDs: number[]
}

export function resolveAssignmentBoardDrop({
  directory,
  draftState,
  source,
  target,
}: {
  directory: Map<number, Employee>
  draftState: DraftState
  source: AssignmentBoardDragSource
  target: AssignmentBoardDropTarget
}) {
  const draggedUser = getSourceUser(source)

  if (target.filledBy?.user_id === draggedUser.userID) {
    return draftState
  }

  if (
    source.kind === "directory-employee" &&
    target.cellUserIDs.includes(draggedUser.userID)
  ) {
    return draftState
  }

  const targetCell = {
    slotID: target.slotID,
    weekday: target.weekday,
    positionID: target.positionID,
    isUnqualified: !isEmployeeQualifiedForPosition(
      directory,
      draggedUser.userID,
      target.positionID,
    ),
  }

  let nextState = draftState

  if (source.kind === "assigned") {
    if (
      source.slotID === target.slotID &&
      source.weekday === target.weekday &&
      source.positionID === target.positionID &&
      source.assignment.assignment_id === target.filledBy?.assignment_id
    ) {
      return draftState
    }

    nextState = enqueueRemove(
      nextState,
      assignmentToDraftInput(
        source.assignment,
        source.slotID,
        source.weekday,
        source.positionID,
      ),
    )
  }

  if (target.filledBy) {
    nextState = enqueueRemove(
      nextState,
      assignmentToDraftInput(
        target.filledBy,
        target.slotID,
        target.weekday,
        target.positionID,
      ),
    )
  }

  return enqueueAdd(nextState, draggedUser, targetCell)
}

export function getDraggedUserID(source: AssignmentBoardDragSource) {
  return source.kind === "directory-employee"
    ? source.employee.user_id
    : source.assignment.user_id
}

function getSourceUser(source: AssignmentBoardDragSource): DraftUserInput {
  if (source.kind === "directory-employee") {
    return {
      userID: source.employee.user_id,
      name: source.employee.name,
      email: source.employee.email,
    }
  }

  return {
    userID: source.assignment.user_id,
    name: source.assignment.name,
    email: source.assignment.email,
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

function isEmployeeQualifiedForPosition(
  directory: Map<number, Employee>,
  userID: number,
  positionID: number,
) {
  return directory.get(userID)?.position_ids.has(positionID) ?? false
}
