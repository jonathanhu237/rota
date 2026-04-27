import { useDraggable, useDroppable } from "@dnd-kit/core"
import { CSS } from "@dnd-kit/utilities"
import { AlertTriangle, X } from "lucide-react"
import { useTranslation } from "react-i18next"

import type { Employee } from "@/components/assignments/assignment-board-directory"
import type {
  AssignmentBoardDragSource,
  AssignmentBoardDropTarget,
} from "@/components/assignments/assignment-board-dnd"
import type { ProjectedAssignment } from "@/components/assignments/draft-state"
import { Badge } from "@/components/ui/badge"
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
            filledBy.isDraft && "ring-1 ring-primary/30",
            filledBy.isRemoved && "line-through opacity-70",
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
            {filledBy.isUnqualified && (
              <AlertTriangle className="size-3.5" aria-hidden />
            )}
            <span className="truncate">{filledLabel ?? filledBy.name}</span>
          </span>
          {filledBy.isDraft && (
            <Badge variant="outline">{t("assignments.drafts.added")}</Badge>
          )}
          {filledBy.isRemoved ? (
            <Badge variant="outline">{t("assignments.drafts.toRemove")}</Badge>
          ) : (
            !isReadOnly && <X className="size-3" aria-hidden />
          )}
        </Button>
      )}
    </div>
  )
}

function getDragClassName({
  draggingUserID,
  directory,
  positionID,
}: {
  draggingUserID: number | null
  directory: Map<number, Employee>
  positionID: number
}) {
  if (draggingUserID === null) {
    return "border-border"
  }

  return directory.get(draggingUserID)?.position_ids.has(positionID)
    ? "border-emerald-500 bg-emerald-50 dark:border-emerald-800 dark:bg-emerald-950/25"
    : "border-amber-500 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/25"
}
