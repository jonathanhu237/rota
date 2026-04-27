import { useMemo, useState } from "react"
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  pointerWithin,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from "@dnd-kit/core"
import { useTranslation } from "react-i18next"

import { deriveEmployeeDirectory } from "@/components/assignments/assignment-board-directory"
import { AssignmentBoardGrid } from "@/components/assignments/assignment-board-grid"
import { AssignmentBoardSidePanel } from "@/components/assignments/assignment-board-side-panel"
import {
  getDraggedUserID,
  resolveAssignmentBoardDrop,
  type AssignmentBoardDragSource,
  type AssignmentBoardDropTarget,
} from "@/components/assignments/assignment-board-dnd"
import {
  DraftConfirmDialog,
  type DraftConfirmWarning,
} from "@/components/assignments/draft-confirm-dialog"
import {
  applyDraftToSlots,
  clearDraftOpError,
  discardDrafts,
  emptyDraftState,
  enqueueRemove,
  removeFirstDraftOp,
  removeDraftOp,
  type DraftOp,
  type DraftState,
} from "@/components/assignments/draft-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import type { AssignmentBoardEmployee, AssignmentBoardSlot } from "@/lib/types"

const weekdayKeys: Record<number, string> = {
  1: "templates.weekday.mon",
  2: "templates.weekday.tue",
  3: "templates.weekday.wed",
  4: "templates.weekday.thu",
  5: "templates.weekday.fri",
  6: "templates.weekday.sat",
  7: "templates.weekday.sun",
}

type AssignmentBoardProps = {
  slots: AssignmentBoardSlot[]
  employees: AssignmentBoardEmployee[]
  isPending: boolean
  isReadOnly: boolean
  onAssign: (
    userID: number,
    slotID: number,
    weekday: number,
    positionID: number,
  ) => void | Promise<void>
  onUnassign: (assignmentID: number) => void | Promise<void>
  onDraftAssign?: (
    userID: number,
    slotID: number,
    weekday: number,
    positionID: number,
  ) => Promise<void>
  onDraftUnassign?: (assignmentID: number) => Promise<void>
  onDraftRefresh?: () => void | Promise<void>
  initialDraftState?: DraftState
}

export function AssignmentBoard({
  slots,
  employees,
  isPending,
  isReadOnly,
  onAssign,
  onUnassign,
  onDraftAssign,
  onDraftUnassign,
  onDraftRefresh,
  initialDraftState = emptyDraftState,
}: AssignmentBoardProps) {
  const { t } = useTranslation()
  const [draftState, setDraftState] = useState<DraftState>(initialDraftState)
  const [committedDraftState, setCommittedDraftState] =
    useState<DraftState>(emptyDraftState)
  const [draggingUserID, setDraggingUserID] = useState<number | null>(null)
  const [activeDragLabel, setActiveDragLabel] = useState<string | null>(null)
  const [isConfirmOpen, setIsConfirmOpen] = useState(false)
  const [isSubmittingDraft, setIsSubmittingDraft] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 4,
      },
    }),
  )

  const renderDraftState = useMemo<DraftState>(
    () => ({
      ops: [...committedDraftState.ops, ...draftState.ops],
    }),
    [committedDraftState.ops, draftState.ops],
  )
  const projectedSlots = useMemo(
    () => applyDraftToSlots(slots, renderDraftState),
    [slots, renderDraftState],
  )
  const directory = useMemo(
    () => deriveEmployeeDirectory(employees),
    [employees],
  )
  const warningEntries = useMemo(
    () => getDraftConfirmWarnings(slots, draftState, t),
    [draftState, slots, t],
  )
  const isDraftDisabled = isReadOnly || isPending || isSubmittingDraft

  const handleDragEnd = (event: DragEndEvent) => {
    const source = getDragSourceData(event.active.data.current)
    const target = getDropTargetData(event.over?.data.current)
    setDraggingUserID(null)
    setActiveDragLabel(null)

    if (!source || !target || isDraftDisabled) {
      return
    }

    setDraftState((currentState) =>
      resolveAssignmentBoardDrop({
        directory,
        draftState: currentState,
        source,
        target,
      }),
    )
    setSubmitError(null)
  }

  const handleDragStart = (event: DragStartEvent) => {
    const source = getDragSourceData(event.active.data.current)
    if (!source) {
      setActiveDragLabel(null)
      setDraggingUserID(null)
      return
    }

    setDraggingUserID(getDraggedUserID(source))
    setActiveDragLabel(
      source.kind === "assigned"
        ? source.assignment.name
        : source.employee.name,
    )
  }

  const handleSubmitClick = () => {
    if (draftState.ops.length === 0 || isSubmittingDraft) {
      return
    }

    if (warningEntries.length > 0) {
      setIsConfirmOpen(true)
      return
    }

    void submitDrafts()
  }

  const submitDrafts = async () => {
    if (draftState.ops.length === 0) {
      return
    }

    setIsConfirmOpen(false)
    setIsSubmittingDraft(true)
    setSubmitError(null)

    let remainingOps = draftState.ops.map(clearDraftOpError)
    const appliedOps: DraftOp[] = []
    setDraftState({ ops: remainingOps })

    for (const op of remainingOps) {
      try {
        await replayDraftOp(op, {
          onAssign: onDraftAssign ?? onAssign,
          onUnassign: onDraftUnassign ?? onUnassign,
        })
      } catch (error) {
        const message = getDraftSubmitErrorMessage(error)
        const failedOps = [
          {
            ...op,
            error: message,
          },
          ...remainingOps.slice(1),
        ]
        setCommittedDraftState({ ops: appliedOps })
        setDraftState({ ops: failedOps })
        setSubmitError(
          t("assignments.drafts.submitFailed", {
            user: getDraftOpUserName(op),
            defaultValue: `Could not submit ${getDraftOpUserName(op)}. Fix the draft or retry.`,
          }),
        )
        await onDraftRefresh?.()
        setCommittedDraftState(emptyDraftState)
        setIsSubmittingDraft(false)
        return
      }

      appliedOps.push(op)
      remainingOps = remainingOps.slice(1)
      setCommittedDraftState({ ops: [...appliedOps] })
      setDraftState(removeFirstDraftOp)
    }

    await onDraftRefresh?.()
    setCommittedDraftState(emptyDraftState)
    setDraftState(discardDrafts())
    setIsSubmittingDraft(false)
  }

  return (
    <>
      <DraftConfirmDialog
        open={isConfirmOpen}
        warnings={warningEntries}
        isPending={isSubmittingDraft}
        onCancel={() => setIsConfirmOpen(false)}
        onConfirm={() => void submitDrafts()}
        onOpenChange={setIsConfirmOpen}
      />
      <DndContext
        collisionDetection={pointerWithin}
        sensors={sensors}
        onDragStart={handleDragStart}
        onDragCancel={() => {
          setDraggingUserID(null)
          setActiveDragLabel(null)
        }}
        onDragEnd={handleDragEnd}
      >
        <div className="grid gap-4">
          <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_24rem]">
            <AssignmentBoardGrid
              slots={projectedSlots}
              serverSlots={slots}
              renderDraftState={renderDraftState}
              disabled={isPending || isSubmittingDraft}
              isReadOnly={isReadOnly}
              draggingUserID={draggingUserID}
              directory={directory}
              onUnassignClick={(assignment, slotID, weekday, positionID) => {
                if (isDraftDisabled) {
                  return
                }

                setDraftState((currentState) =>
                  enqueueRemove(currentState, {
                    assignmentID: assignment.assignment_id,
                    userID: assignment.user_id,
                    name: assignment.name,
                    email: assignment.email,
                    slotID,
                    weekday,
                    positionID,
                    sourceOpID: assignment.draftOpID,
                  }),
                )
                setSubmitError(null)
              }}
              onCancelDraft={(draftOpID) => {
                setDraftState((currentState) =>
                  removeDraftOp(currentState, draftOpID),
                )
                setSubmitError(null)
              }}
            />
            <AssignmentBoardSidePanel
              slots={slots}
              projectedSlots={projectedSlots}
              renderDraftState={renderDraftState}
              directory={directory}
              disabled={isDraftDisabled}
            />
          </div>

          <footer className="flex flex-col gap-3 rounded-lg border bg-card px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="grid gap-1">
              <div className="text-sm font-medium">
                {t("assignments.drafts.pendingCount", {
                  count: draftState.ops.length,
                })}
              </div>
              {submitError && (
                <p className="text-sm text-destructive">{submitError}</p>
              )}
              {draftState.ops.find((op) => op.error) && (
                <p className="text-sm text-destructive">
                  {draftState.ops.find((op) => op.error)?.error}
                </p>
              )}
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                variant="outline"
                disabled={draftState.ops.length === 0 || isSubmittingDraft}
                onClick={() => {
                  setDraftState(discardDrafts())
                  setSubmitError(null)
                }}
              >
                {t("assignments.drafts.discard")}
              </Button>
              <Button
                type="button"
                disabled={draftState.ops.length === 0 || isSubmittingDraft}
                onClick={handleSubmitClick}
              >
                {t("assignments.drafts.submit")}
              </Button>
            </div>
          </footer>
        </div>

        <DragOverlay>
          {activeDragLabel ? (
            <Badge variant="secondary" className="px-3 py-1 shadow-lg">
              {activeDragLabel}
            </Badge>
          ) : null}
        </DragOverlay>
      </DndContext>
    </>
  )
}

function getDraftConfirmWarnings(
  slots: AssignmentBoardSlot[],
  draftState: DraftState,
  t: (key: string, options?: Record<string, unknown>) => string,
): DraftConfirmWarning[] {
  return draftState.ops
    .filter((op) => op.kind === "assign" && op.isUnqualified)
    .map((op) => {
      const slotEntry = slots.find(
        (entry) =>
          entry.slot.id === op.slotID && entry.slot.weekday === op.weekday,
      )
      const positionEntry = slotEntry?.positions.find(
        (entry) => entry.position.id === op.positionID,
      )
      const fallbackPosition = findPositionName(slots, op.positionID)
      const shiftLabel = slotEntry
        ? t("assignments.shiftSummary", {
            startTime: slotEntry.slot.start_time,
            endTime: slotEntry.slot.end_time,
          })
        : ""
      const weekdayLabel = slotEntry ? t(weekdayKeys[slotEntry.slot.weekday]) : ""

      return {
        id: op.id,
        userName: op.userName,
        slotLabel: [weekdayLabel, shiftLabel].filter(Boolean).join(" "),
        positionName: positionEntry?.position.name ?? fallbackPosition,
      }
    })
}

function getDragSourceData(value: unknown): AssignmentBoardDragSource | null {
  if (!value || typeof value !== "object" || !("kind" in value)) {
    return null
  }

  const data = value as AssignmentBoardDragSource
  return data.kind === "assigned" || data.kind === "directory-employee"
    ? data
    : null
}

function getDropTargetData(value: unknown): AssignmentBoardDropTarget | null {
  if (!value || typeof value !== "object" || !("kind" in value)) {
    return null
  }

  const data = value as AssignmentBoardDropTarget
  return data.kind === "seat" ? data : null
}

function findPositionName(slots: AssignmentBoardSlot[], positionID: number) {
  for (const slotEntry of slots) {
    const positionEntry = slotEntry.positions.find(
      (entry) => entry.position.id === positionID,
    )
    if (positionEntry) {
      return positionEntry.position.name
    }
  }

  return `#${positionID}`
}

async function replayDraftOp(
  op: DraftOp,
  handlers: {
    onAssign: (
      userID: number,
      slotID: number,
      weekday: number,
      positionID: number,
    ) => void | Promise<void>
    onUnassign: (assignmentID: number) => void | Promise<void>
  },
) {
  if (op.kind === "assign") {
    await handlers.onAssign(op.userID, op.slotID, op.weekday, op.positionID)
    return
  }

  if (op.assignmentID < 0) {
    return
  }

  await handlers.onUnassign(op.assignmentID)
}

function getDraftOpUserName(op: DraftOp) {
  return op.userName
}

function getDraftSubmitErrorMessage(error: unknown) {
  if (error instanceof Error && error.message) {
    return error.message
  }

  return "Unable to submit this draft operation."
}
