import { useMemo, useState, type ReactNode } from "react"
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragOverEvent,
  type DragStartEvent,
} from "@dnd-kit/core"
import { CSS } from "@dnd-kit/utilities"
import { AlertTriangle } from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  getVisibleNonCandidateQualified,
  isAssignmentBoardPositionUnderstaffed,
} from "@/components/assignments/assignment-board-state"
import {
  resolveAssignmentBoardDrop,
  type AssignmentBoardDragSource,
  type AssignmentBoardDropTarget,
} from "@/components/assignments/assignment-board-dnd"
import {
  DraftConfirmDialog,
  type DraftConfirmWarning,
} from "@/components/assignments/draft-confirm-dialog"
import {
  applyDraftToBoard,
  applyDraftToSlots,
  clearDraftOpError,
  computeUserHours,
  discardDrafts,
  emptyDraftState,
  enqueueAdd,
  formatHours,
  getBoardCellKey,
  isCellChangedFromServer,
  removeFirstDraftOp,
  type DraftOp,
  type DraftState,
  type DraftUserInput,
  type ProjectedAssignment,
} from "@/components/assignments/draft-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Switch } from "@/components/ui/switch"
import { cn } from "@/lib/utils"
import type {
  AssignmentBoardCandidate,
  AssignmentBoardPosition,
  AssignmentBoardSlot,
} from "@/lib/types"

import {
  assignmentBoardWeekdays,
  groupAssignmentBoardSlotsByWeekday,
} from "./group-assignment-board-shifts"

const weekdayKeys = {
  1: "templates.weekday.mon",
  2: "templates.weekday.tue",
  3: "templates.weekday.wed",
  4: "templates.weekday.thu",
  5: "templates.weekday.fri",
  6: "templates.weekday.sat",
  7: "templates.weekday.sun",
} as const

type AssignmentBoardProps = {
  slots: AssignmentBoardSlot[]
  isPending: boolean
  isReadOnly: boolean
  onAssign: (
    userID: number,
    slotID: number,
    positionID: number,
  ) => void | Promise<void>
  onUnassign: (assignmentID: number) => void | Promise<void>
  onDraftAssign?: (
    userID: number,
    slotID: number,
    positionID: number,
  ) => Promise<void>
  onDraftUnassign?: (assignmentID: number) => Promise<void>
  onDraftRefresh?: () => void | Promise<void>
  initialDraftState?: DraftState
}

type DropPreview = {
  cellKey: string
  isUnqualified: boolean
}

export function AssignmentBoard({
  slots,
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
  const [showAllQualified, setShowAllQualified] = useState(false)
  const [draftState, setDraftState] = useState<DraftState>(initialDraftState)
  const [committedDraftState, setCommittedDraftState] =
    useState<DraftState>(emptyDraftState)
  const [dropPreview, setDropPreview] = useState<DropPreview | null>(null)
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
  const groupedSlots = groupAssignmentBoardSlotsByWeekday(projectedSlots)
  const projectedBoard = useMemo(
    () => applyDraftToBoard(slots, renderDraftState),
    [slots, renderDraftState],
  )
  const warningEntries = useMemo(
    () => getDraftConfirmWarnings(slots, draftState, t),
    [draftState, slots, t],
  )
  const isDraftDisabled = isReadOnly || isPending || isSubmittingDraft

  function formatUserLabel(name: string, hours: number) {
    const formattedHours = formatHours(hours)
    const translated = t("assignments.drafts.userHoursLabel", {
      user: name,
      hours: formattedHours,
    })

    return translated === "assignments.drafts.userHoursLabel"
      ? `${name} (${formattedHours}h)`
      : translated
  }

  const handleDragOver = (event: DragOverEvent) => {
    const source = getDragSourceData(event.active.data.current)
    const target = getDropTargetData(event.over?.data.current)

    if (!source || !target) {
      setDropPreview(null)
      return
    }

    const userID =
      source.kind === "assignment"
        ? source.assignment.user_id
        : source.candidate.user_id
    setDropPreview({
      cellKey: getBoardCellKey(target.slotID, target.positionID),
      isUnqualified: !isUserQualifiedForCell(
        slots,
        target.slotID,
        target.positionID,
        userID,
      ),
    })
  }

  const handleDragEnd = (event: DragEndEvent) => {
    const source = getDragSourceData(event.active.data.current)
    const target = getDropTargetData(event.over?.data.current)
    setDropPreview(null)
    setActiveDragLabel(null)

    if (!source || !target || isDraftDisabled) {
      return
    }

    setDraftState((currentState) =>
      resolveAssignmentBoardDrop({
        slots,
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
      return
    }

    setActiveDragLabel(
      source.kind === "assignment"
        ? source.assignment.name
        : source.candidate.name,
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
        sensors={sensors}
        onDragStart={handleDragStart}
        onDragOver={handleDragOver}
        onDragCancel={() => {
          setDropPreview(null)
          setActiveDragLabel(null)
        }}
        onDragEnd={handleDragEnd}
      >
        <div className="grid gap-4">
          <div className="flex items-center justify-between rounded-xl border border-dashed bg-muted/30 px-4 py-3">
            <div className="grid gap-1">
              <div className="text-sm font-medium">
                {t("publications.assignmentBoard.showAllQualified")}
              </div>
            </div>
            <Switch
              aria-label={t("publications.assignmentBoard.showAllQualified")}
              checked={showAllQualified}
              onCheckedChange={setShowAllQualified}
            />
          </div>

          <div className="grid gap-4 xl:grid-cols-2">
            {assignmentBoardWeekdays.map((weekday) => (
              <section key={weekday} className="rounded-xl border bg-card">
                <header className="border-b bg-muted/40 px-4 py-3">
                  <h3 className="font-medium">{t(weekdayKeys[weekday])}</h3>
                </header>
                <div className="grid gap-4 p-4">
                  {groupedSlots[weekday].length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                      {t("assignments.emptyWeekday")}
                    </p>
                  ) : (
                    groupedSlots[weekday].map((slotEntry) => {
                      const slotUnderstaffed = slotEntry.positions.some(
                        (position) =>
                          isAssignmentBoardPositionUnderstaffed(position),
                      )

                      return (
                        <article
                          key={slotEntry.slot.id}
                          className={cn(
                            "grid gap-4 rounded-xl border p-4",
                            slotUnderstaffed &&
                              "border-amber-300 bg-amber-50/60 dark:border-amber-900 dark:bg-amber-950/20",
                          )}
                        >
                          <div className="flex flex-wrap items-start justify-between gap-3">
                            <div className="grid gap-1">
                              <div className="font-medium">
                                {t("assignments.shiftSummary", {
                                  startTime: slotEntry.slot.start_time,
                                  endTime: slotEntry.slot.end_time,
                                })}
                              </div>
                              <div className="text-sm text-muted-foreground">
                                {t("assignments.headcount", {
                                  assigned: slotEntry.positions.reduce(
                                    (count, position) =>
                                      count + position.assignments.length,
                                    0,
                                  ),
                                  required: slotEntry.positions.reduce(
                                    (count, position) =>
                                      count + position.required_headcount,
                                    0,
                                  ),
                                })}
                              </div>
                            </div>
                            <Badge variant="secondary">
                              {slotEntry.positions.length}
                            </Badge>
                          </div>

                          <div className="grid gap-3 lg:grid-cols-2">
                            {slotEntry.positions.map((positionEntry) => {
                              const cellKey = getBoardCellKey(
                                slotEntry.slot.id,
                                positionEntry.position.id,
                              )
                              const projectedAssignments =
                                projectedBoard.get(cellKey) ?? []
                              const changed = isCellChangedFromServer(
                                slots,
                                projectedAssignments,
                                slotEntry.slot.id,
                                positionEntry.position.id,
                              )
                              const understaffed =
                                isAssignmentBoardPositionUnderstaffed(
                                  positionEntry,
                                )
                              const visibleNonCandidateQualified =
                                getVisibleNonCandidateQualified(
                                  slotEntry,
                                  positionEntry,
                                  showAllQualified,
                                )
                              const assignedUserIDs = new Set(
                                positionEntry.assignments.map(
                                  (assignment) => assignment.user_id,
                                ),
                              )
                              const visibleCandidates =
                                positionEntry.candidates.filter(
                                  (candidate) =>
                                    !assignedUserIDs.has(candidate.user_id),
                                )
                              const visibleQualified =
                                visibleNonCandidateQualified.filter(
                                  (candidate) =>
                                    !assignedUserIDs.has(candidate.user_id),
                                )
                              const hasVisibleQualifiedOptions =
                                visibleCandidates.length > 0 ||
                                visibleQualified.length > 0

                              return (
                                <DroppablePositionCell
                                  key={`${slotEntry.slot.id}-${positionEntry.position.id}`}
                                  disabled={isDraftDisabled}
                                  dropPreview={dropPreview}
                                  isChanged={changed}
                                  isUnderstaffed={understaffed}
                                  position={positionEntry}
                                  slotID={slotEntry.slot.id}
                                >
                                  <div className="flex flex-wrap items-start justify-between gap-3">
                                    <div className="grid gap-1">
                                      <div className="font-medium">
                                        {positionEntry.position.name}
                                      </div>
                                      <div className="text-sm text-muted-foreground">
                                        {t("assignments.headcount", {
                                          assigned:
                                            positionEntry.assignments.length,
                                          required:
                                            positionEntry.required_headcount,
                                        })}
                                      </div>
                                    </div>
                                    <div className="flex flex-wrap items-center gap-2">
                                      <Badge
                                        variant={
                                          understaffed
                                            ? "destructive"
                                            : "secondary"
                                        }
                                      >
                                        {t("assignments.headcount", {
                                          assigned:
                                            positionEntry.assignments.length,
                                          required:
                                            positionEntry.required_headcount,
                                        })}
                                      </Badge>
                                      {changed && (
                                        <Badge variant="outline">
                                          {t("assignments.drafts.changed")}
                                        </Badge>
                                      )}
                                      {understaffed && (
                                        <Badge variant="outline">
                                          {t("assignments.understaffed")}
                                        </Badge>
                                      )}
                                    </div>
                                  </div>

                                  <div className="grid gap-2">
                                    <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                      {t("assignments.candidates")}
                                    </div>
                                    {!hasVisibleQualifiedOptions ? (
                                      <p className="text-sm text-muted-foreground">
                                        {t("assignments.emptyCandidates")}
                                      </p>
                                    ) : (
                                      <div className="grid gap-2">
                                        {visibleCandidates.length > 0 && (
                                          <div className="flex flex-wrap gap-2">
                                            {visibleCandidates.map(
                                              (candidate) => (
                                                <DraggableCandidateButton
                                                  key={`${slotEntry.slot.id}-${positionEntry.position.id}-${candidate.user_id}`}
                                                  candidate={candidate}
                                                  disabled={isDraftDisabled}
                                                  label={formatUserLabel(
                                                    candidate.name,
                                                    getCandidatePreviewHours(
                                                      slots,
                                                      renderDraftState,
                                                      candidate,
                                                      slotEntry.slot.id,
                                                      positionEntry.position.id,
                                                    ),
                                                  )}
                                                  positionID={
                                                    positionEntry.position.id
                                                  }
                                                  slotID={slotEntry.slot.id}
                                                  onAssign={onAssign}
                                                />
                                              ),
                                            )}
                                          </div>
                                        )}

                                        {visibleQualified.length > 0 && (
                                          <div className="flex flex-wrap gap-2 border-t border-dashed pt-2">
                                            {visibleQualified.map(
                                              (candidate) => (
                                                <DraggableCandidateButton
                                                  key={`qualified-${slotEntry.slot.id}-${positionEntry.position.id}-${candidate.user_id}`}
                                                  candidate={candidate}
                                                  disabled={isDraftDisabled}
                                                  label={formatUserLabel(
                                                    candidate.name,
                                                    getCandidatePreviewHours(
                                                      slots,
                                                      renderDraftState,
                                                      candidate,
                                                      slotEntry.slot.id,
                                                      positionEntry.position.id,
                                                    ),
                                                  )}
                                                  positionID={
                                                    positionEntry.position.id
                                                  }
                                                  slotID={slotEntry.slot.id}
                                                  variant="qualified"
                                                  onAssign={onAssign}
                                                />
                                              ),
                                            )}
                                          </div>
                                        )}
                                      </div>
                                    )}
                                  </div>

                                  <div className="grid gap-2">
                                    <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                      {t("assignments.assigned")}
                                    </div>
                                    {positionEntry.assignments.length === 0 ? (
                                      <p className="text-sm text-muted-foreground">
                                        {t("assignments.emptyAssignments")}
                                      </p>
                                    ) : (
                                      <div className="flex flex-wrap gap-2">
                                        {positionEntry.assignments.map(
                                          (assignment) => (
                                            <DraggableAssignmentButton
                                              key={`${slotEntry.slot.id}-${positionEntry.position.id}-${assignment.assignment_id}-${assignment.user_id}`}
                                              assignment={assignment}
                                              disabled={isDraftDisabled}
                                              isReadOnly={isReadOnly}
                                              label={formatUserLabel(
                                                assignment.name,
                                                computeUserHours(
                                                  slots,
                                                  renderDraftState,
                                                  assignment.user_id,
                                                ),
                                              )}
                                              positionID={
                                                positionEntry.position.id
                                              }
                                              slotID={slotEntry.slot.id}
                                              onRemoveDraftAssignment={(
                                                opID,
                                              ) =>
                                                setDraftState((currentState) => ({
                                                  ops: currentState.ops.filter(
                                                    (op) => op.id !== opID,
                                                  ),
                                                }))
                                              }
                                              onUnassign={onUnassign}
                                            />
                                          ),
                                        )}
                                      </div>
                                    )}
                                  </div>
                                </DroppablePositionCell>
                              )
                            })}
                          </div>
                        </article>
                      )
                    })
                  )}
                </div>
              </section>
            ))}
          </div>

          <footer className="flex flex-col gap-3 rounded-xl border bg-card px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
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

function DroppablePositionCell({
  children,
  disabled,
  dropPreview,
  isChanged,
  isUnderstaffed,
  position,
  slotID,
}: {
  children: ReactNode
  disabled: boolean
  dropPreview: DropPreview | null
  isChanged: boolean
  isUnderstaffed: boolean
  position: AssignmentBoardPosition
  slotID: number
}) {
  const cellKey = getBoardCellKey(slotID, position.position.id)
  const { setNodeRef } = useDroppable({
    id: `cell:${cellKey}`,
    data: {
      kind: "cell",
      slotID,
      positionID: position.position.id,
    } satisfies AssignmentBoardDropTarget,
    disabled,
  })
  const isDropTarget = dropPreview?.cellKey === cellKey

  return (
    <section
      ref={setNodeRef}
      className={cn(
        "grid gap-4 rounded-xl border p-4 transition-colors",
        isUnderstaffed &&
          "border-amber-300 bg-amber-50/60 dark:border-amber-900 dark:bg-amber-950/20",
        isChanged &&
          "border-primary/40 bg-primary/5 dark:border-primary/50 dark:bg-primary/10",
        isDropTarget &&
          !dropPreview.isUnqualified &&
          "border-emerald-400 bg-emerald-50 dark:border-emerald-800 dark:bg-emerald-950/25",
        isDropTarget &&
          dropPreview.isUnqualified &&
          "border-destructive/60 bg-destructive/10",
      )}
    >
      {children}
    </section>
  )
}

function DraggableCandidateButton({
  candidate,
  disabled,
  label,
  positionID,
  slotID,
  variant = "candidate",
  onAssign,
}: {
  candidate: AssignmentBoardCandidate
  disabled: boolean
  label: string
  positionID: number
  slotID: number
  variant?: "candidate" | "qualified"
  onAssign: (
    userID: number,
    slotID: number,
    positionID: number,
  ) => void | Promise<void>
}) {
  const { attributes, listeners, setNodeRef, transform } = useDraggable({
    id: `candidate:${slotID}:${positionID}:${candidate.user_id}`,
    data: {
      kind: "candidate",
      candidate,
      slotID,
      positionID,
    } satisfies AssignmentBoardDragSource,
    disabled,
  })
  const style = {
    transform: CSS.Translate.toString(transform),
  }

  return (
    <span ref={setNodeRef} style={style} {...attributes} {...listeners}>
      <Button
        type="button"
        size="sm"
        variant="outline"
        className={cn(
          variant === "qualified" &&
            "h-auto items-start border-dashed px-3 py-2 text-left",
        )}
        disabled={disabled}
        onClick={() => void onAssign(candidate.user_id, slotID, positionID)}
        title={candidate.email}
      >
        <span>{label}</span>
        {variant === "qualified" && (
          <span className="text-[10px] font-normal text-muted-foreground">
            <QualifiedHint />
          </span>
        )}
      </Button>
    </span>
  )
}

function QualifiedHint() {
  const { t } = useTranslation()
  return t("publications.assignmentBoard.didNotSubmitAvailability")
}

function DraggableAssignmentButton({
  assignment,
  disabled,
  isReadOnly,
  label,
  positionID,
  slotID,
  onRemoveDraftAssignment,
  onUnassign,
}: {
  assignment: ProjectedAssignment
  disabled: boolean
  isReadOnly: boolean
  label: string
  positionID: number
  slotID: number
  onRemoveDraftAssignment: (opID: string) => void
  onUnassign: (assignmentID: number) => void | Promise<void>
}) {
  const dropID = `assignment-target:${slotID}:${positionID}:${assignment.assignment_id}:${assignment.user_id}`
  const dragID = `assignment:${slotID}:${positionID}:${assignment.assignment_id}:${assignment.user_id}`
  const { setNodeRef: setDroppableRef } = useDroppable({
    id: dropID,
    data: {
      kind: "assignment",
      assignment,
      slotID,
      positionID,
    } satisfies AssignmentBoardDropTarget,
    disabled,
  })
  const { attributes, listeners, setNodeRef: setDraggableRef, transform } =
    useDraggable({
      id: dragID,
      data: {
        kind: "assignment",
        assignment,
        slotID,
        positionID,
      } satisfies AssignmentBoardDragSource,
      disabled,
    })
  const setNodeRef = (node: HTMLElement | null) => {
    setDroppableRef(node)
    setDraggableRef(node)
  }
  const style = {
    transform: CSS.Translate.toString(transform),
  }

  if (isReadOnly) {
    return (
      <span ref={setNodeRef} style={style} {...attributes} {...listeners}>
        <Badge
          variant="secondary"
          className={cn(
            "px-3 py-1",
            assignment.isDraft && "bg-primary/10 text-primary",
            assignment.isUnqualified && "bg-destructive/10 text-destructive",
          )}
          title={assignment.email}
        >
          <AssignmentLabel assignment={assignment} label={label} />
        </Badge>
      </span>
    )
  }

  return (
    <span ref={setNodeRef} style={style} {...attributes} {...listeners}>
      <Button
        type="button"
        size="sm"
        variant={assignment.isUnqualified ? "destructive" : "secondary"}
        className={cn(assignment.isDraft && "ring-1 ring-primary/30")}
        disabled={disabled}
        onClick={() => {
          if (assignment.draftOpID) {
            onRemoveDraftAssignment(assignment.draftOpID)
            return
          }

          void onUnassign(assignment.assignment_id)
        }}
        title={assignment.email}
      >
        <AssignmentLabel assignment={assignment} label={label} />
      </Button>
    </span>
  )
}

function AssignmentLabel({
  assignment,
  label,
}: {
  assignment: ProjectedAssignment
  label: string
}) {
  return (
    <>
      {assignment.isUnqualified && (
        <AlertTriangle className="size-3.5" aria-hidden="true" />
      )}
      <span>{label}</span>
    </>
  )
}

function getCandidatePreviewHours(
  slots: AssignmentBoardSlot[],
  draftState: DraftState,
  candidate: AssignmentBoardCandidate,
  slotID: number,
  positionID: number,
) {
  const previewDraftState = enqueueAdd(
    draftState,
    candidateToDraftUser(candidate),
    {
      slotID,
      positionID,
      isUnqualified: !isUserQualifiedForCell(
        slots,
        slotID,
        positionID,
        candidate.user_id,
      ),
    },
  )
  return computeUserHours(slots, previewDraftState, candidate.user_id)
}

function getDraftConfirmWarnings(
  slots: AssignmentBoardSlot[],
  draftState: DraftState,
  t: (key: string, options?: Record<string, unknown>) => string,
): DraftConfirmWarning[] {
  return draftState.ops
    .filter((op) => op.kind === "assign" && op.isUnqualified)
    .map((op) => {
      const slotEntry = slots.find((entry) => entry.slot.id === op.slotID)
      const positionEntry = slotEntry?.positions.find(
        (entry) => entry.position.id === op.positionID,
      )
      const shiftLabel = slotEntry
        ? t("assignments.shiftSummary", {
            startTime: slotEntry.slot.start_time,
            endTime: slotEntry.slot.end_time,
          })
        : ""
      const weekdayLabel = slotEntry
        ? t(weekdayKeys[slotEntry.slot.weekday as keyof typeof weekdayKeys])
        : ""

      return {
        id: op.id,
        userName: op.userName,
        slotLabel: [weekdayLabel, shiftLabel].filter(Boolean).join(" "),
        positionName: positionEntry?.position.name ?? "",
      }
    })
}

function getDragSourceData(value: unknown): AssignmentBoardDragSource | null {
  if (!value || typeof value !== "object" || !("kind" in value)) {
    return null
  }

  const data = value as AssignmentBoardDragSource
  return data.kind === "assignment" || data.kind === "candidate" ? data : null
}

function getDropTargetData(value: unknown): AssignmentBoardDropTarget | null {
  if (!value || typeof value !== "object" || !("kind" in value)) {
    return null
  }

  const data = value as AssignmentBoardDropTarget
  return data.kind === "assignment" || data.kind === "cell" ? data : null
}

function candidateToDraftUser(candidate: AssignmentBoardCandidate): DraftUserInput {
  return {
    userID: candidate.user_id,
    name: candidate.name,
    email: candidate.email,
  }
}

function findBoardPosition(
  slots: AssignmentBoardSlot[],
  slotID: number,
  positionID: number,
) {
  const slotEntry = slots.find((entry) => entry.slot.id === slotID)
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
  positionID: number,
  userID: number,
) {
  const positionEntry = findBoardPosition(slots, slotID, positionID)?.position
  if (!positionEntry) {
    return false
  }

  return [
    ...positionEntry.candidates,
    ...positionEntry.non_candidate_qualified,
    ...positionEntry.assignments,
  ].some((candidate) => candidate.user_id === userID)
}

async function replayDraftOp(
  op: DraftOp,
  handlers: {
    onAssign: (
      userID: number,
      slotID: number,
      positionID: number,
    ) => void | Promise<void>
    onUnassign: (assignmentID: number) => void | Promise<void>
  },
) {
  if (op.kind === "assign") {
    await handlers.onAssign(op.userID, op.slotID, op.positionID)
    return
  }

  if (op.assignmentID < 0) {
    return
  }

  await handlers.onUnassign(op.assignmentID)
}

function getDraftOpUserName(op: DraftOp) {
  return op.kind === "assign" ? op.userName : op.userName
}

function getDraftSubmitErrorMessage(error: unknown) {
  if (error instanceof Error && error.message) {
    return error.message
  }

  return "Unable to submit this draft operation."
}
