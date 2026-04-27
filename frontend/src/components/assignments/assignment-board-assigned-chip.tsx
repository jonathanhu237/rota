import { useDraggable } from "@dnd-kit/core"
import { CSS } from "@dnd-kit/utilities"
import { AlertTriangle, X } from "lucide-react"
import { useTranslation } from "react-i18next"

import type { AssignmentBoardDragSource } from "@/components/assignments/assignment-board-dnd"
import type { ProjectedAssignment } from "@/components/assignments/draft-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

export function AssignmentChip({
  assignment,
  disabled,
  isReadOnly,
  label,
  positionID,
  slotID,
  weekday,
  onClick,
}: {
  assignment: ProjectedAssignment
  disabled: boolean
  isReadOnly: boolean
  label: string
  positionID: number
  slotID: number
  weekday: number
  onClick: () => void
}) {
  const { t } = useTranslation()
  const { attributes, listeners, setNodeRef, transform } = useDraggable({
    id: `assigned:${slotID}:${weekday}:${positionID}:${assignment.assignment_id}:${assignment.user_id}`,
    data: {
      kind: "assigned",
      assignment,
      slotID,
      weekday,
      positionID,
    } satisfies AssignmentBoardDragSource,
    disabled,
  })
  const style = { transform: CSS.Translate.toString(transform) }

  return (
    <span ref={setNodeRef} style={style} {...attributes} {...listeners}>
      <Button
        type="button"
        size="sm"
        variant={assignment.isUnqualified ? "destructive" : "secondary"}
        className={cn(
          "h-auto min-h-7 gap-1.5 whitespace-normal text-left",
          assignment.isDraft && "ring-1 ring-primary/30",
          assignment.isRemoved && "line-through opacity-70",
        )}
        disabled={disabled || isReadOnly}
        title={assignment.email}
        onClick={onClick}
      >
        {assignment.isUnqualified && (
          <AlertTriangle className="size-3.5" aria-hidden="true" />
        )}
        <span>{label}</span>
        {assignment.isDraft && (
          <Badge variant="outline">{t("assignments.drafts.added")}</Badge>
        )}
        {assignment.isRemoved && (
          <Badge variant="outline">{t("assignments.drafts.toRemove")}</Badge>
        )}
        {!assignment.isDraft && !assignment.isRemoved && !isReadOnly && (
          <X className="size-3" aria-hidden="true" />
        )}
      </Button>
    </span>
  )
}
