import { useDraggable, useDroppable } from "@dnd-kit/core"
import { CSS } from "@dnd-kit/utilities"
import { AlertTriangle, Undo2, X } from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  slotWeekdayKey,
  type Employee,
} from "@/components/assignments/assignment-board-directory"
import type {
  AssignmentBoardDragSource,
  AssignmentBoardDropTarget,
} from "@/components/assignments/assignment-board-dnd"
import type { ProjectedAssignment } from "@/components/assignments/draft-state"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

export function AssignmentBoardSeat({
  slotID,
  weekday,
  positionID,
  headcountIndex,
  positionName,
  filledBy,
  filledLabel,
  cellUserIDs,
  draggingUserID,
  directory,
  disabled,
  isReadOnly,
  onUnassignClick,
  onCancelDraft,
}: {
  slotID: number
  weekday: number
  positionID: number
  headcountIndex: number
  positionName: string
  filledBy: ProjectedAssignment | null
  filledLabel?: string
  cellUserIDs: number[]
  draggingUserID: number | null
  directory: Map<number, Employee>
  disabled: boolean
  isReadOnly: boolean
  onUnassignClick: (assignment: ProjectedAssignment) => void
  onCancelDraft: (draftOpID: string) => void
}) {
  const { t } = useTranslation()
  const { isOver, setNodeRef: setDroppableRef } = useDroppable({
    id: `seat:${slotID}:${weekday}:${positionID}:${headcountIndex}`,
    data: {
      kind: "seat",
      slotID,
      weekday,
      positionID,
      headcountIndex,
      filledBy,
      cellUserIDs,
    } satisfies AssignmentBoardDropTarget,
    disabled,
  })
  const isDraggable = Boolean(filledBy && !filledBy.isRemoved)
  const {
    attributes,
    listeners,
    setNodeRef: setDraggableRef,
    transform,
    isDragging,
  } = useDraggable({
    id: filledBy
      ? `assigned:${slotID}:${weekday}:${positionID}:${filledBy.assignment_id}:${filledBy.user_id}`
      : `empty:${slotID}:${weekday}:${positionID}:${headcountIndex}`,
    data: filledBy
      ? ({
          kind: "assigned",
        assignment: filledBy,
        slotID,
        weekday,
        positionID,
      } satisfies AssignmentBoardDragSource)
      : undefined,
    disabled: disabled || !isDraggable,
  })
  const style = { transform: CSS.Translate.toString(transform) }
  const dragClassName = getDragClassName({
    draggingUserID,
    directory,
    slotID,
    weekday,
    positionID,
  })

  return (
    <div
      ref={setDroppableRef}
      data-testid="assignment-seat"
      className={cn(
        "grid min-h-14 gap-1 rounded-md border bg-background p-2 transition",
        dragClassName,
        isOver && "ring-2 ring-primary/40",
        isDragging && "opacity-50",
      )}
    >
      <div className="truncate text-[11px] font-medium text-muted-foreground">
        {positionName}
      </div>

      {!filledBy ? (
        <div className="truncate text-xs text-muted-foreground">
          {t("assignments.seat.empty")} · {positionName}
        </div>
      ) : (
        <Button
          ref={isDraggable ? setDraggableRef : undefined}
          style={isDraggable ? style : undefined}
          type="button"
          size="sm"
          variant={filledBy.isUnqualified ? "destructive" : "secondary"}
          className={cn(
            "h-auto min-h-7 justify-between gap-1.5 whitespace-normal px-2 text-left",
            filledBy.isRemoved && "opacity-70",
          )}
          disabled={disabled || isReadOnly}
          title={filledBy.email}
          onClick={() => {
            if (filledBy.isRemoved && filledBy.draftOpID) {
              onCancelDraft(filledBy.draftOpID)
              return
            }

            onUnassignClick(filledBy)
          }}
          {...(isDraggable ? attributes : {})}
          {...(isDraggable ? listeners : {})}
        >
          <span className="inline-flex min-w-0 items-center gap-1">
            {filledBy.isDraft && !filledBy.isRemoved && (
              <span
                className="size-1.5 shrink-0 rounded-full bg-primary"
                aria-hidden="true"
                data-testid="assignment-draft-dot"
              />
            )}
            {filledBy.isUnqualified && (
              <AlertTriangle
                className="size-3.5 text-red-500"
                role="img"
                aria-label={t("assignments.drafts.unqualifiedAria")}
              />
            )}
            {filledBy.isUnsubmitted && !filledBy.isUnqualified && (
              <AlertTriangle
                className="size-3.5 text-amber-500"
                role="img"
                aria-label={t("assignments.drafts.unsubmittedAria")}
              />
            )}
            <span
              className={cn(
                "truncate",
                filledBy.isRemoved && "line-through text-muted-foreground",
              )}
            >
              {filledLabel ?? filledBy.name}
            </span>
          </span>
          {filledBy.isRemoved ? (
            <Undo2
              className="size-3 shrink-0"
              aria-label={t("assignments.drafts.undoRemove")}
            />
          ) : (
            !isReadOnly && (
              <X
                className="size-3 shrink-0"
                aria-label={t("assignments.drafts.remove")}
              />
            )
          )}
        </Button>
      )}
    </div>
  )
}

function getDragClassName({
  draggingUserID,
  directory,
  slotID,
  weekday,
  positionID,
}: {
  draggingUserID: number | null
  directory: Map<number, Employee>
  slotID: number
  weekday: number
  positionID: number
}) {
  if (draggingUserID === null) {
    return "border-border"
  }

  const draggedUser = directory.get(draggingUserID)
  if (!draggedUser) {
    return "border-border"
  }

  if (!draggedUser.position_ids.has(positionID)) {
    return "border-red-500 bg-red-50 dark:border-red-800 dark:bg-red-950/25"
  }

  if (!draggedUser.submittedSlots.has(slotWeekdayKey(slotID, weekday))) {
    return "border-amber-500 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/25"
  }

  return "border-emerald-500 bg-emerald-50 dark:border-emerald-800 dark:bg-emerald-950/25"
}
