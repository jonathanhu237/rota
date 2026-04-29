import { useDraggable } from "@dnd-kit/core"
import { CSS } from "@dnd-kit/utilities"
import { GripVertical } from "lucide-react"
import { useTranslation } from "react-i18next"

import type { Employee } from "@/components/assignments/assignment-board-directory"
import type { AssignmentBoardDragSource } from "@/components/assignments/assignment-board-dnd"
import { formatHours } from "@/components/assignments/draft-state"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

export function AssignmentBoardEmployeeRow({
  employee,
  totalHours,
  positionNames,
  disabled,
  showHours = true,
}: {
  employee: Employee
  totalHours: number
  positionNames: string[]
  disabled: boolean
  showHours?: boolean
}) {
  const { t } = useTranslation()
  const { attributes, listeners, setNodeRef, transform, isDragging } =
    useDraggable({
      id: `directory:${employee.user_id}`,
      data: {
        kind: "directory-employee",
        employee,
      } satisfies AssignmentBoardDragSource,
      disabled,
    })
  const style = { transform: CSS.Translate.toString(transform) }

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      {...listeners}
      tabIndex={disabled ? -1 : 0}
      aria-label={employee.name}
      aria-disabled={disabled}
      data-testid="assignment-directory-row"
      className={cn(
        "grid gap-2 rounded-lg border bg-background p-3 text-left transition",
        "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
        disabled ? "opacity-60" : "cursor-grab active:cursor-grabbing",
        isDragging && "opacity-50",
      )}
    >
      <div className="flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          <GripVertical className="size-4 text-muted-foreground" aria-hidden />
          <div className="min-w-0">
            <div className="truncate text-sm font-medium">{employee.name}</div>
            {showHours ? (
              <div className="text-xs text-muted-foreground">
                {t("assignments.directory.hours", {
                  hours: formatHours(totalHours),
                })}
              </div>
            ) : (
              <div className="text-xs text-muted-foreground">
                {t("assignments.directory.notSubmittedTag")}
              </div>
            )}
          </div>
        </div>
      </div>
      <div className="flex flex-wrap gap-1.5">
        {positionNames.map((positionName) => (
          <Badge key={positionName} variant="outline">
            {positionName}
          </Badge>
        ))}
      </div>
    </div>
  )
}
