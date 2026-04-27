import { useDraggable } from "@dnd-kit/core"
import { CSS } from "@dnd-kit/utilities"
import { useTranslation } from "react-i18next"

import type { AssignmentBoardDragSource } from "@/components/assignments/assignment-board-dnd"
import { Button } from "@/components/ui/button"
import type { AssignmentBoardCandidate } from "@/lib/types"

export function CandidateChip({
  candidate,
  disabled,
  isQualifiedOnly,
  label,
  positionID,
  slotID,
  weekday,
  onClick,
}: {
  candidate: AssignmentBoardCandidate
  disabled: boolean
  isQualifiedOnly: boolean
  label: string
  positionID: number
  slotID: number
  weekday: number
  onClick: () => void
}) {
  const { t } = useTranslation()
  const { attributes, listeners, setNodeRef, transform } = useDraggable({
    id: `candidate:${slotID}:${weekday}:${positionID}:${candidate.user_id}`,
    data: {
      kind: "candidate",
      candidate,
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
        variant="outline"
        className="h-auto min-h-7 whitespace-normal text-left"
        disabled={disabled}
        title={candidate.email}
        onClick={onClick}
      >
        <span>{label}</span>
        {isQualifiedOnly && (
          <span className="text-[10px] font-normal text-muted-foreground">
            {t("publications.assignmentBoard.didNotSubmitAvailability")}
          </span>
        )}
      </Button>
    </span>
  )
}
