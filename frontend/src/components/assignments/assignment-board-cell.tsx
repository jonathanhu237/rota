import { useTranslation } from "react-i18next"

import type { Employee } from "@/components/assignments/assignment-board-directory"
import { AssignmentBoardSeat } from "@/components/assignments/assignment-board-seat"
import {
  type GridCell,
  type ScheduledGridCell,
} from "@/components/assignments/assignment-board-grid-cells"
import {
  computeUserHours,
  formatHours,
  type DraftState,
  type ProjectedAssignment,
} from "@/components/assignments/draft-state"
import { Badge } from "@/components/ui/badge"
import type { AssignmentBoardSlot } from "@/lib/types"
import { cn } from "@/lib/utils"

export function AssignmentBoardCell({
  cell,
  serverSlots,
  renderDraftState,
  disabled,
  isReadOnly,
  draggingUserID,
  directory,
  onUnassignClick,
  onCancelDraft,
}: {
  cell: GridCell
  serverSlots: AssignmentBoardSlot[]
  renderDraftState: DraftState
  disabled: boolean
  isReadOnly: boolean
  draggingUserID: number | null
  directory: Map<number, Employee>
  onUnassignClick: (
    assignment: ProjectedAssignment,
    slotID: number,
    weekday: number,
    positionID: number,
  ) => void
  onCancelDraft: (draftOpID: string) => void
}) {
  const { t } = useTranslation()

  if (cell.kind === "off-schedule") {
    return (
      <div
        aria-disabled="true"
        className="flex min-h-28 items-center justify-center rounded-md border border-dashed bg-muted/40 text-muted-foreground"
      >
        &mdash;
      </div>
    )
  }

  const cellUserIDs = getCellUserIDs(cell)

  return (
    <section
      className={cn(
        "grid min-h-28 gap-2 rounded-md border bg-background p-2 transition",
        getStatusClassName(cell.totals.status),
      )}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-xs font-medium">
          {t("assignments.headcount", {
            assigned: cell.totals.assigned,
            required: cell.totals.required,
          })}
        </span>
        <Badge
          variant={cell.totals.status === "empty" ? "destructive" : "secondary"}
          className={cn(
            cell.totals.status === "full" &&
              "bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300",
            cell.totals.status === "partial" &&
              "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
          )}
        >
          {t(`assignments.status.${cell.totals.status}`)}
        </Badge>
      </div>

      <div className="grid gap-2">
        {cell.positions.map((positionEntry) => {
          const assignments = getVisibleAssignments({
            serverSlots,
            renderDraftState,
            cell,
            positionID: positionEntry.position.id,
            projectedAssignments: positionEntry.assignments,
          })
          const regularAssignments = assignments.slice(
            0,
            positionEntry.required_headcount,
          )
          const overflowAssignments = assignments.slice(
            positionEntry.required_headcount,
          )

          return (
            <div
              key={positionEntry.position.id}
              className="grid gap-1.5"
              data-testid="assignment-position-seats"
            >
              {Array.from({ length: positionEntry.required_headcount }).map(
                (_, index) => (
                  <AssignmentBoardSeat
                    key={`${positionEntry.position.id}:${index}`}
                    slotID={cell.slotID}
                    weekday={cell.weekday}
                    positionID={positionEntry.position.id}
                    headcountIndex={index}
                    positionName={positionEntry.position.name}
                    filledBy={regularAssignments[index] ?? null}
                    filledLabel={
                      regularAssignments[index]
                        ? formatUserLabel(
                            t,
                            regularAssignments[index].name,
                            computeUserHours(
                              serverSlots,
                              renderDraftState,
                              regularAssignments[index].user_id,
                            ),
                          )
                        : undefined
                    }
                    cellUserIDs={cellUserIDs}
                    draggingUserID={draggingUserID}
                    directory={directory}
                    disabled={disabled}
                    isReadOnly={isReadOnly}
                    onUnassignClick={(assignment) =>
                      onUnassignClick(
                        assignment,
                        cell.slotID,
                        cell.weekday,
                        positionEntry.position.id,
                      )
                    }
                    onCancelDraft={onCancelDraft}
                  />
                ),
              )}

              {overflowAssignments.length > 0 && (
                <div className="grid gap-1 border-t border-dashed pt-1">
                  <div className="text-[10px] font-medium text-muted-foreground">
                    {t("assignments.seat.overflow")}
                  </div>
                  {overflowAssignments.map((assignment, index) => (
                    <AssignmentBoardSeat
                      key={`overflow:${assignment.assignment_id}:${assignment.user_id}`}
                      slotID={cell.slotID}
                      weekday={cell.weekday}
                      positionID={positionEntry.position.id}
                      headcountIndex={
                        positionEntry.required_headcount + index
                      }
                      positionName={positionEntry.position.name}
                      filledBy={assignment}
                      filledLabel={formatUserLabel(
                        t,
                        assignment.name,
                        computeUserHours(
                          serverSlots,
                          renderDraftState,
                          assignment.user_id,
                        ),
                      )}
                      cellUserIDs={cellUserIDs}
                      draggingUserID={draggingUserID}
                      directory={directory}
                      disabled={disabled}
                      isReadOnly={isReadOnly}
                      onUnassignClick={(filledAssignment) =>
                        onUnassignClick(
                          filledAssignment,
                          cell.slotID,
                          cell.weekday,
                          positionEntry.position.id,
                        )
                      }
                      onCancelDraft={onCancelDraft}
                    />
                  ))}
                </div>
              )}
            </div>
          )
        })}
      </div>
    </section>
  )
}

function getVisibleAssignments({
  serverSlots,
  renderDraftState,
  cell,
  positionID,
  projectedAssignments,
}: {
  serverSlots: AssignmentBoardSlot[]
  renderDraftState: DraftState
  cell: ScheduledGridCell
  positionID: number
  projectedAssignments: ProjectedAssignment[]
}) {
  const removedAssignments = renderDraftState.ops.flatMap((op) => {
    if (
      op.kind !== "unassign" ||
      op.slotID !== cell.slotID ||
      op.weekday !== cell.weekday ||
      op.positionID !== positionID
    ) {
      return []
    }

    const original = findOriginalAssignment(serverSlots, {
      slotID: cell.slotID,
      weekday: cell.weekday,
      positionID,
      assignmentID: op.assignmentID,
    })

    if (!original) {
      return []
    }

    return [
      {
        ...original,
        isRemoved: true,
        draftOpID: op.id,
        error: op.error,
      },
    ]
  })

  return [...projectedAssignments, ...removedAssignments].sort((left, right) => {
    if (left.isRemoved !== right.isRemoved) {
      return left.isRemoved ? 1 : -1
    }

    return left.assignment_id - right.assignment_id
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
  return slots
    .find((entry) => entry.slot.id === slotID && entry.slot.weekday === weekday)
    ?.positions.find((entry) => entry.position.id === positionID)
    ?.assignments.find((assignment) => assignment.assignment_id === assignmentID)
}

function getCellUserIDs(cell: ScheduledGridCell) {
  return cell.positions.flatMap((position) =>
    position.assignments.map((assignment) => assignment.user_id),
  )
}

function getStatusClassName(status: "full" | "partial" | "empty") {
  if (status === "full") {
    return "border-emerald-200 bg-emerald-50/50 dark:border-emerald-900 dark:bg-emerald-950/20"
  }

  if (status === "partial") {
    return "border-amber-200 bg-amber-50/50 dark:border-amber-900 dark:bg-amber-950/20"
  }

  return "border-red-200 bg-red-50/50 dark:border-red-900 dark:bg-red-950/20"
}

function formatUserLabel(
  t: (key: string, options?: Record<string, unknown>) => string,
  name: string,
  hours: number,
) {
  const formattedHours = formatHours(hours)
  const translated = t("assignments.drafts.userHoursLabel", {
    user: name,
    hours: formattedHours,
  })

  return translated === "assignments.drafts.userHoursLabel"
    ? `${name} (${formattedHours}h)`
    : translated
}
