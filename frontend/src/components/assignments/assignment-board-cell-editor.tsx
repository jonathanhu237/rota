import {
  useMemo,
  useState,
  type Dispatch,
  type SetStateAction,
} from "react"
import { X } from "lucide-react"
import { useTranslation } from "react-i18next"

import { AssignmentChip } from "@/components/assignments/assignment-board-assigned-chip"
import { CandidateChip } from "@/components/assignments/assignment-board-candidate-chip"
import type { AssignmentBoardSelection } from "@/components/assignments/assignment-board-dnd"
import {
  formatUserLabel,
  weekdayKeys,
} from "@/components/assignments/assignment-board-side-panel-utils"
import {
  applyDraftToBoard,
  computeUserHours,
  enqueueAdd,
  enqueueRemove,
  getBoardCellKey,
  removeDraftOp,
  type DraftState,
  type DraftUserInput,
  type ProjectedAssignment,
  type ProjectedAssignmentBoardPosition,
  type ProjectedAssignmentBoardSlot,
} from "@/components/assignments/draft-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Switch } from "@/components/ui/switch"
import type {
  AssignmentBoardCandidate,
  AssignmentBoardSlot,
} from "@/lib/types"
import { cn } from "@/lib/utils"

type VisiblePosition = ProjectedAssignmentBoardPosition & {
  isSynthetic?: boolean
}

export function CellEditor({
  slots,
  projectedSlots,
  renderDraftState,
  disabled,
  isReadOnly,
  selection,
  onSelectionChange,
  onDraftStateChange,
}: {
  slots: AssignmentBoardSlot[]
  projectedSlots: ProjectedAssignmentBoardSlot[]
  renderDraftState: DraftState
  disabled: boolean
  isReadOnly: boolean
  selection: AssignmentBoardSelection
  onSelectionChange: (selection: AssignmentBoardSelection | null) => void
  onDraftStateChange: Dispatch<SetStateAction<DraftState>>
}) {
  const { t } = useTranslation()
  const [showAllQualified, setShowAllQualified] = useState<
    Record<string, boolean>
  >({})

  const projectedBoard = useMemo(
    () => applyDraftToBoard(slots, renderDraftState),
    [slots, renderDraftState],
  )

  const selectedSlot = findSlot(projectedSlots, selection)

  if (!selectedSlot) {
    return (
      <aside className="rounded-lg border bg-card p-4">
        <p className="text-sm text-muted-foreground">
          {t("assignments.emptyAssignments")}
        </p>
      </aside>
    )
  }

  const visiblePositions = getVisiblePositions({
    slots,
    projectedSlot: selectedSlot,
    draftState: renderDraftState,
    projectedBoard,
  })
  const totals = getSlotTotals(visiblePositions)
  const cellAssignedUserIDs = new Set(
    visiblePositions.flatMap((positionEntry) =>
      positionEntry.assignments
        .filter((assignment) => !assignment.isRemoved)
        .map((assignment) => assignment.user_id),
    ),
  )

  return (
    <aside className="flex max-h-[760px] flex-col rounded-lg border bg-card">
      <header className="sticky top-0 z-10 flex items-start justify-between gap-3 border-b bg-card px-4 py-3">
        <div className="grid gap-1">
          <h3 className="font-medium">
            {t(weekdayKeys[selectedSlot.slot.weekday])}{" "}
            {t("assignments.shiftSummary", {
              startTime: selectedSlot.slot.start_time,
              endTime: selectedSlot.slot.end_time,
            })}
          </h3>
          <p className="text-sm text-muted-foreground">
            {t("assignments.headcount", {
              assigned: totals.assigned,
              required: totals.required,
            })}
          </p>
        </div>
        <Button
          type="button"
          size="icon-sm"
          variant="ghost"
          aria-label={t("assignments.panel.close")}
          onClick={() => onSelectionChange(null)}
        >
          <X className="size-4" aria-hidden="true" />
        </Button>
      </header>

      <div className="grid gap-4 overflow-y-auto p-4">
        {visiblePositions.map((positionEntry) => {
          const cellKey = getBoardCellKey(
            selectedSlot.slot.id,
            selectedSlot.slot.weekday,
            positionEntry.position.id,
          )
          const visibleCandidates = positionEntry.candidates.filter(
            (candidate) => !cellAssignedUserIDs.has(candidate.user_id),
          )
          const visibleQualified = showAllQualified[cellKey]
            ? positionEntry.non_candidate_qualified.filter(
                (candidate) => !cellAssignedUserIDs.has(candidate.user_id),
              )
            : []
          const hasCandidates =
            visibleCandidates.length > 0 || visibleQualified.length > 0

          return (
            <section
              key={cellKey}
              className={cn(
                "grid gap-4 rounded-lg border p-4",
                positionEntry.isSynthetic &&
                  "border-destructive/40 bg-destructive/5",
              )}
            >
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="grid gap-1">
                  <h4 className="font-medium">{positionEntry.position.name}</h4>
                  <p className="text-sm text-muted-foreground">
                    {t("assignments.headcount", {
                      assigned: positionEntry.assignments.filter(
                        (assignment) => !assignment.isRemoved,
                      ).length,
                      required: positionEntry.required_headcount,
                    })}
                  </p>
                </div>
                {positionEntry.isSynthetic && (
                  <Badge variant="destructive">
                    {t("assignments.drafts.warning")}
                  </Badge>
                )}
              </div>

              <div className="grid gap-2">
                <div className="text-xs font-medium uppercase text-muted-foreground">
                  {t("assignments.assigned")}
                </div>
                {positionEntry.assignments.length === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    {t("assignments.emptyAssignments")}
                  </p>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {positionEntry.assignments.map((assignment) => (
                      <AssignmentChip
                        key={`${assignment.assignment_id}-${assignment.user_id}`}
                        assignment={assignment}
                        disabled={disabled}
                        isReadOnly={isReadOnly}
                        label={formatUserLabel(
                          t,
                          assignment.name,
                          computeUserHours(
                            slots,
                            renderDraftState,
                            assignment.user_id,
                          ),
                        )}
                        positionID={positionEntry.position.id}
                        slotID={selectedSlot.slot.id}
                        weekday={selectedSlot.slot.weekday}
                        onClick={() => {
                          if (disabled || isReadOnly) {
                            return
                          }

                          if (assignment.draftOpID) {
                            const draftOpID = assignment.draftOpID
                            onDraftStateChange((currentState) =>
                              removeDraftOp(currentState, draftOpID),
                            )
                            return
                          }

                          onDraftStateChange((currentState) =>
                            enqueueRemove(currentState, {
                              assignmentID: assignment.assignment_id,
                              userID: assignment.user_id,
                              name: assignment.name,
                              email: assignment.email,
                              slotID: selectedSlot.slot.id,
                              weekday: selectedSlot.slot.weekday,
                              positionID: positionEntry.position.id,
                            }),
                          )
                        }}
                      />
                    ))}
                  </div>
                )}
              </div>

              <div className="grid gap-2">
                <div className="flex items-center justify-between gap-3">
                  <div className="text-xs font-medium uppercase text-muted-foreground">
                    {t("assignments.candidates")}
                  </div>
                  {!positionEntry.isSynthetic && (
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-muted-foreground">
                        {t("publications.assignmentBoard.showAllQualified")}
                      </span>
                      <Switch
                        aria-label={t(
                          "publications.assignmentBoard.showAllQualified",
                        )}
                        checked={showAllQualified[cellKey] ?? false}
                        onCheckedChange={(checked) =>
                          setShowAllQualified((current) => ({
                            ...current,
                            [cellKey]: checked,
                          }))
                        }
                      />
                    </div>
                  )}
                </div>

                {!hasCandidates ? (
                  <p className="text-sm text-muted-foreground">
                    {t("assignments.emptyCandidates")}
                  </p>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {[...visibleCandidates, ...visibleQualified].map(
                      (candidate) => (
                        <CandidateChip
                          key={`${candidate.user_id}-${cellKey}`}
                          candidate={candidate}
                          disabled={disabled || isReadOnly}
                          isQualifiedOnly={visibleQualified.some(
                            (entry) => entry.user_id === candidate.user_id,
                          )}
                          label={formatUserLabel(
                            t,
                            candidate.name,
                            getCandidatePreviewHours(
                              slots,
                              renderDraftState,
                              candidate,
                              selectedSlot.slot.id,
                              selectedSlot.slot.weekday,
                              positionEntry.position.id,
                            ),
                          )}
                          positionID={positionEntry.position.id}
                          slotID={selectedSlot.slot.id}
                          weekday={selectedSlot.slot.weekday}
                          onClick={() => {
                            if (disabled || isReadOnly) {
                              return
                            }

                            onDraftStateChange((currentState) =>
                              enqueueAdd(
                                currentState,
                                candidateToDraftUser(candidate),
                                {
                                  slotID: selectedSlot.slot.id,
                                  weekday: selectedSlot.slot.weekday,
                                  positionID: positionEntry.position.id,
                                  isUnqualified: !isUserQualifiedForCell(
                                    slots,
                                    selectedSlot.slot.id,
                                    selectedSlot.slot.weekday,
                                    positionEntry.position.id,
                                    candidate.user_id,
                                  ),
                                },
                              ),
                            )
                          }}
                        />
                      ),
                    )}
                  </div>
                )}
              </div>
            </section>
          )
        })}
      </div>
    </aside>
  )
}

function getVisiblePositions({
  slots,
  projectedSlot,
  draftState,
  projectedBoard,
}: {
  slots: AssignmentBoardSlot[]
  projectedSlot: ProjectedAssignmentBoardSlot
  draftState: DraftState
  projectedBoard: Map<string, ProjectedAssignment[]>
}): VisiblePosition[] {
  const positions: VisiblePosition[] = projectedSlot.positions.map(
    (positionEntry) => ({
      ...positionEntry,
      assignments: [
        ...positionEntry.assignments,
        ...getRemovedAssignments({
          slots,
          draftState,
          slotID: projectedSlot.slot.id,
          weekday: projectedSlot.slot.weekday,
          positionID: positionEntry.position.id,
        }),
      ],
    }),
  )
  const existingPositionIDs = new Set(
    positions.map((positionEntry) => positionEntry.position.id),
  )

  for (const op of draftState.ops) {
    if (
      op.kind !== "assign" ||
      op.slotID !== projectedSlot.slot.id ||
      op.weekday !== projectedSlot.slot.weekday ||
      existingPositionIDs.has(op.positionID)
    ) {
      continue
    }

    positions.push({
      position: {
        id: op.positionID,
        name: findPositionName(slots, op.positionID),
      },
      required_headcount: 0,
      candidates: [],
      non_candidate_qualified: [],
      assignments:
        projectedBoard.get(
          getBoardCellKey(op.slotID, op.weekday, op.positionID),
        ) ?? [],
      isSynthetic: true,
    })
    existingPositionIDs.add(op.positionID)
  }

  return positions
}

function getRemovedAssignments({
  slots,
  draftState,
  slotID,
  weekday,
  positionID,
}: {
  slots: AssignmentBoardSlot[]
  draftState: DraftState
  slotID: number
  weekday: number
  positionID: number
}): ProjectedAssignment[] {
  return draftState.ops.flatMap((op) => {
    if (
      op.kind !== "unassign" ||
      op.slotID !== slotID ||
      op.weekday !== weekday ||
      op.positionID !== positionID
    ) {
      return []
    }

    const assignment = findOriginalAssignment(slots, {
      slotID,
      weekday,
      positionID,
      assignmentID: op.assignmentID,
    })
    if (!assignment) {
      return []
    }

    return [
      {
        ...assignment,
        isRemoved: true,
        draftOpID: op.id,
        error: op.error,
      },
    ]
  })
}

function findOriginalAssignment(
  slots: AssignmentBoardSlot[],
  {
    slotID,
    weekday,
    positionID,
    assignmentID,
  }: {
    slotID: number
    weekday: number
    positionID: number
    assignmentID: number
  },
) {
  return findSlot(slots, { slotID, weekday })?.positions
    .find((entry) => entry.position.id === positionID)
    ?.assignments.find((assignment) => assignment.assignment_id === assignmentID)
}

function findSlot<T extends AssignmentBoardSlot>(
  slots: T[],
  selection: AssignmentBoardSelection,
) {
  return slots.find(
    (entry) =>
      entry.slot.id === selection.slotID &&
      entry.slot.weekday === selection.weekday,
  )
}

function getSlotTotals(positions: VisiblePosition[]) {
  return positions.reduce(
    (totals, position) => ({
      assigned:
        totals.assigned +
        position.assignments.filter((assignment) => !assignment.isRemoved).length,
      required: totals.required + position.required_headcount,
    }),
    { assigned: 0, required: 0 },
  )
}

function getCandidatePreviewHours(
  slots: AssignmentBoardSlot[],
  draftState: DraftState,
  candidate: AssignmentBoardCandidate,
  slotID: number,
  weekday: number,
  positionID: number,
) {
  const previewDraftState = enqueueAdd(
    draftState,
    candidateToDraftUser(candidate),
    {
      slotID,
      weekday,
      positionID,
      isUnqualified: !isUserQualifiedForCell(
        slots,
        slotID,
        weekday,
        positionID,
        candidate.user_id,
      ),
    },
  )
  return computeUserHours(slots, previewDraftState, candidate.user_id)
}

function isUserQualifiedForCell(
  slots: AssignmentBoardSlot[],
  slotID: number,
  weekday: number,
  positionID: number,
  userID: number,
) {
  const positionEntry = findSlot(slots, { slotID, weekday })?.positions.find(
    (entry) => entry.position.id === positionID,
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

function candidateToDraftUser(candidate: AssignmentBoardCandidate): DraftUserInput {
  return {
    userID: candidate.user_id,
    name: candidate.name,
    email: candidate.email,
  }
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
