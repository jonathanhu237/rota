import type {
  AssignmentBoardAssignment,
  AssignmentBoardSlot,
} from "@/lib/types"

export type DraftAssignOp = {
  id: string
  kind: "assign"
  slotID: number
  positionID: number
  userID: number
  userName: string
  userEmail: string
  isUnqualified: boolean
  error?: string
}

export type DraftUnassignOp = {
  id: string
  kind: "unassign"
  assignmentID: number
  userID: number
  userName: string
  slotID: number
  positionID: number
  error?: string
}

export type DraftOp = DraftAssignOp | DraftUnassignOp

export type DraftState = {
  ops: DraftOp[]
}

export type DraftUserInput = {
  userID: number
  name: string
  email?: string
}

export type DraftAssignmentInput = DraftUserInput & {
  assignmentID: number
  slotID: number
  positionID: number
  sourceOpID?: string
}

export type DraftCellInput = {
  slotID: number
  positionID: number
  isUnqualified?: boolean
}

export type ProjectedAssignment = AssignmentBoardAssignment & {
  isDraft?: boolean
  isUnqualified?: boolean
  draftOpID?: string
  error?: string
}

export type ProjectedAssignmentBoardPosition = Omit<
  AssignmentBoardSlot["positions"][number],
  "assignments"
> & {
  assignments: ProjectedAssignment[]
}

export type ProjectedAssignmentBoardSlot = Omit<
  AssignmentBoardSlot,
  "positions"
> & {
  positions: ProjectedAssignmentBoardPosition[]
}

export const emptyDraftState: DraftState = { ops: [] }

export function discardDrafts(): DraftState {
  return emptyDraftState
}

export function getBoardCellKey(slotID: number, positionID: number) {
  return `${slotID}:${positionID}`
}

export function enqueueAssign(
  state: DraftState,
  input: DraftUserInput & DraftCellInput,
): DraftState {
  return appendOps(state, [
    {
      id: nextDraftOpID(state, "assign"),
      kind: "assign",
      slotID: input.slotID,
      positionID: input.positionID,
      userID: input.userID,
      userName: input.name,
      userEmail: input.email ?? "",
      isUnqualified: input.isUnqualified ?? false,
    },
  ])
}

export function enqueueUnassign(
  state: DraftState,
  input: DraftAssignmentInput,
): DraftState {
  if (input.sourceOpID) {
    return removeDraftOp(state, input.sourceOpID)
  }

  return appendOps(state, [
    {
      id: nextDraftOpID(state, "unassign"),
      kind: "unassign",
      assignmentID: input.assignmentID,
      userID: input.userID,
      userName: input.name,
      slotID: input.slotID,
      positionID: input.positionID,
    },
  ])
}

export function enqueueMove(
  state: DraftState,
  from: DraftAssignmentInput,
  to: DraftCellInput,
): DraftState {
  if (from.slotID === to.slotID && from.positionID === to.positionID) {
    return state
  }

  if (from.sourceOpID) {
    const withoutSourceAssign = removeDraftOp(state, from.sourceOpID)
    return enqueueAssign(withoutSourceAssign, {
      userID: from.userID,
      name: from.name,
      email: from.email,
      slotID: to.slotID,
      positionID: to.positionID,
      isUnqualified: to.isUnqualified,
    })
  }

  return appendOps(state, [
    makeUnassignOp(state, 0, from),
    makeAssignOp(state, 1, from, to),
  ])
}

export function enqueueSwap(
  state: DraftState,
  dragged: DraftAssignmentInput,
  target: DraftAssignmentInput,
  draggedTarget: DraftCellInput,
  targetTarget: DraftCellInput,
): DraftState {
  if (
    dragged.assignmentID === target.assignmentID &&
    dragged.slotID === target.slotID &&
    dragged.positionID === target.positionID
  ) {
    return state
  }

  let nextState = state
  const opsToAppend: DraftOp[] = []

  if (dragged.sourceOpID) {
    nextState = removeDraftOp(nextState, dragged.sourceOpID)
  } else {
    opsToAppend.push(makeUnassignOp(nextState, opsToAppend.length, dragged))
  }

  if (target.sourceOpID) {
    nextState = removeDraftOp(nextState, target.sourceOpID)
  } else {
    opsToAppend.push(makeUnassignOp(nextState, opsToAppend.length, target))
  }

  opsToAppend.push(
    makeAssignOp(nextState, opsToAppend.length, dragged, draggedTarget),
    makeAssignOp(nextState, opsToAppend.length + 1, target, targetTarget),
  )

  return appendOps(nextState, opsToAppend)
}

export function enqueueReplace(
  state: DraftState,
  outgoing: DraftAssignmentInput,
  incoming: DraftUserInput,
  to: DraftCellInput,
): DraftState {
  let nextState = state
  const opsToAppend: DraftOp[] = []

  if (outgoing.sourceOpID) {
    nextState = removeDraftOp(nextState, outgoing.sourceOpID)
  } else {
    opsToAppend.push(makeUnassignOp(nextState, 0, outgoing))
  }

  opsToAppend.push(makeAssignOp(nextState, opsToAppend.length, incoming, to))

  return appendOps(nextState, opsToAppend)
}

export function enqueueAdd(
  state: DraftState,
  incoming: DraftUserInput,
  to: DraftCellInput,
): DraftState {
  return enqueueAssign(state, {
    ...incoming,
    slotID: to.slotID,
    positionID: to.positionID,
    isUnqualified: to.isUnqualified,
  })
}

export function removeDraftOp(state: DraftState, opID: string): DraftState {
  return {
    ops: state.ops.filter((op) => op.id !== opID),
  }
}

export function removeFirstDraftOp(state: DraftState): DraftState {
  return {
    ops: state.ops.slice(1),
  }
}

export function markDraftOpError(
  state: DraftState,
  opID: string,
  error: string,
): DraftState {
  return {
    ops: state.ops.map((op) =>
      op.id === opID
        ? {
            ...op,
            error,
          }
        : op,
    ),
  }
}

export function clearDraftOpError(op: DraftOp): DraftOp {
  if (!op.error) {
    return op
  }

  return {
    ...op,
    error: undefined,
  }
}

export function applyDraftToBoard(
  serverSnapshot: AssignmentBoardSlot[],
  draftState: DraftState,
): Map<string, ProjectedAssignment[]> {
  const projected = new Map<string, ProjectedAssignment[]>()

  for (const slotEntry of serverSnapshot) {
    for (const positionEntry of slotEntry.positions) {
      projected.set(
        getBoardCellKey(slotEntry.slot.id, positionEntry.position.id),
        positionEntry.assignments.map((assignment) => ({ ...assignment })),
      )
    }
  }

  for (const op of draftState.ops) {
    if (op.error) {
      break
    }

    const cellKey = getBoardCellKey(op.slotID, op.positionID)
    const assignments = projected.get(cellKey) ?? []

    if (op.kind === "unassign") {
      projected.set(
        cellKey,
        assignments.filter((assignment) =>
          op.assignmentID > 0
            ? assignment.assignment_id !== op.assignmentID
            : assignment.user_id !== op.userID,
        ),
      )
      continue
    }

    if (assignments.some((assignment) => assignment.user_id === op.userID)) {
      projected.set(
        cellKey,
        assignments.map((assignment) =>
          assignment.user_id === op.userID
            ? {
                ...assignment,
                isDraft: true,
                isUnqualified: op.isUnqualified,
                draftOpID: op.id,
                error: op.error,
              }
            : assignment,
        ),
      )
      continue
    }

    projected.set(cellKey, [
      ...assignments,
      {
        assignment_id: draftAssignmentID(op.id),
        user_id: op.userID,
        name: op.userName,
        email: op.userEmail,
        isDraft: true,
        isUnqualified: op.isUnqualified,
        draftOpID: op.id,
        error: op.error,
      },
    ])
  }

  return projected
}

export function applyDraftToSlots(
  serverSnapshot: AssignmentBoardSlot[],
  draftState: DraftState,
): ProjectedAssignmentBoardSlot[] {
  const projected = applyDraftToBoard(serverSnapshot, draftState)

  return serverSnapshot.map((slotEntry) => ({
    ...slotEntry,
    positions: slotEntry.positions.map((positionEntry) => ({
      ...positionEntry,
      assignments:
        projected.get(
          getBoardCellKey(slotEntry.slot.id, positionEntry.position.id),
        ) ?? [],
    })),
  }))
}

export function computeUserHours(
  snapshot: AssignmentBoardSlot[],
  draftState: DraftState,
  userID: number,
): number {
  const projected = applyDraftToBoard(snapshot, draftState)
  let totalMinutes = 0

  for (const slotEntry of snapshot) {
    const minutes = getSlotDurationMinutes(
      slotEntry.slot.start_time,
      slotEntry.slot.end_time,
    )

    for (const positionEntry of slotEntry.positions) {
      const assignments =
        projected.get(
          getBoardCellKey(slotEntry.slot.id, positionEntry.position.id),
        ) ?? []

      if (assignments.some((assignment) => assignment.user_id === userID)) {
        totalMinutes += minutes
      }
    }
  }

  return totalMinutes / 60
}

export function formatHours(hours: number) {
  if (Number.isInteger(hours)) {
    return String(hours)
  }

  return hours.toFixed(1)
}

export function isCellChangedFromServer(
  snapshot: AssignmentBoardSlot[],
  projectedAssignments: ProjectedAssignment[],
  slotID: number,
  positionID: number,
) {
  const serverAssignments =
    snapshot
      .find((slotEntry) => slotEntry.slot.id === slotID)
      ?.positions.find((entry) => entry.position.id === positionID)
      ?.assignments ?? []

  return (
    toSortedUserIDs(serverAssignments).join(",") !==
    toSortedUserIDs(projectedAssignments).join(",")
  )
}

function appendOps(state: DraftState, ops: DraftOp[]): DraftState {
  return {
    ops: [...state.ops, ...ops],
  }
}

function makeAssignOp(
  state: DraftState,
  offset: number,
  user: DraftUserInput,
  cell: DraftCellInput,
): DraftAssignOp {
  return {
    id: nextDraftOpID(state, "assign", offset),
    kind: "assign",
    slotID: cell.slotID,
    positionID: cell.positionID,
    userID: user.userID,
    userName: user.name,
    userEmail: user.email ?? "",
    isUnqualified: cell.isUnqualified ?? false,
  }
}

function makeUnassignOp(
  state: DraftState,
  offset: number,
  assignment: DraftAssignmentInput,
): DraftUnassignOp {
  return {
    id: nextDraftOpID(state, "unassign", offset),
    kind: "unassign",
    assignmentID: assignment.assignmentID,
    userID: assignment.userID,
    userName: assignment.name,
    slotID: assignment.slotID,
    positionID: assignment.positionID,
  }
}

function nextDraftOpID(
  state: DraftState,
  kind: DraftOp["kind"],
  offset = 0,
) {
  return `${kind}-${state.ops.length + offset + 1}`
}

function draftAssignmentID(opID: string) {
  let hash = 0
  for (const char of opID) {
    hash = (hash * 31 + char.charCodeAt(0)) % 100000
  }
  return -1 * (hash + 1)
}

function getSlotDurationMinutes(startTime: string, endTime: string) {
  const [startHour, startMinute] = startTime.split(":").map(Number)
  const [endHour, endMinute] = endTime.split(":").map(Number)
  const start = startHour * 60 + startMinute
  const end = endHour * 60 + endMinute
  return end >= start ? end - start : end + 24 * 60 - start
}

function toSortedUserIDs(assignments: Pick<AssignmentBoardAssignment, "user_id">[]) {
  return assignments.map((assignment) => assignment.user_id).sort((a, b) => a - b)
}
