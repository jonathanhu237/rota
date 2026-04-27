import { useDroppable } from "@dnd-kit/core"
import { useTranslation } from "react-i18next"

import type { AssignmentBoardDropTarget } from "@/components/assignments/assignment-board-dnd"
import {
  getGridCellKey,
  type GridCell,
} from "@/components/assignments/assignment-board-grid-cells"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

export type GridDropPreview = {
  slotID: number
  weekday: number
  isUnqualified: boolean
}

export function AssignmentBoardCell({
  cell,
  disabled,
  dropPreview,
  isSelected,
  onSelect,
}: {
  cell: GridCell
  disabled: boolean
  dropPreview: GridDropPreview | null
  isSelected: boolean
  onSelect: (slotID: number, weekday: number) => void
}) {
  const { t } = useTranslation()
  const isScheduled = cell.kind === "scheduled"
  const droppableData = isScheduled
    ? ({
        kind: "cell",
        slotID: cell.slotID,
        weekday: cell.weekday,
      } satisfies AssignmentBoardDropTarget)
    : undefined
  const { isOver, setNodeRef } = useDroppable({
    id: isScheduled
      ? `grid-cell:${getGridCellKey(cell.slotID, cell.weekday)}`
      : `off-schedule:${cell.timeBlockIndex}:${cell.weekday}`,
    data: droppableData,
    disabled: !isScheduled || disabled,
  })

  if (!isScheduled) {
    return (
      <div
        ref={setNodeRef}
        aria-disabled="true"
        className="flex min-h-20 items-center justify-center rounded-md border border-dashed bg-muted/40 text-muted-foreground"
      >
        &mdash;
      </div>
    )
  }

  const isDropTarget =
    dropPreview?.slotID === cell.slotID && dropPreview.weekday === cell.weekday

  return (
    <button
      ref={setNodeRef}
      type="button"
      aria-pressed={isSelected}
      disabled={disabled}
      className={cn(
        "grid min-h-20 w-full content-center gap-2 rounded-md border bg-background px-3 py-2 text-left transition",
        "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
        getStatusClassName(cell.totals.status),
        isSelected && "ring-2 ring-primary",
        (isOver || isDropTarget) &&
          !dropPreview?.isUnqualified &&
          "border-emerald-400 bg-emerald-50 dark:border-emerald-800 dark:bg-emerald-950/25",
        (isOver || isDropTarget) &&
          dropPreview?.isUnqualified &&
          "border-destructive/60 bg-destructive/10",
      )}
      onClick={() => onSelect(cell.slotID, cell.weekday)}
    >
      <span className="text-xs font-medium">
        {t("assignments.headcount", {
          assigned: cell.totals.assigned,
          required: cell.totals.required,
        })}
      </span>
      <Badge
        variant={cell.totals.status === "empty" ? "destructive" : "secondary"}
        className={cn(
          "w-fit",
          cell.totals.status === "full" &&
            "bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300",
          cell.totals.status === "partial" &&
            "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
        )}
      >
        {t(`assignments.status.${cell.totals.status}`)}
      </Badge>
    </button>
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
