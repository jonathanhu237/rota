import type { Dispatch, SetStateAction } from "react"

import { CellEditor } from "@/components/assignments/assignment-board-cell-editor"
import type { AssignmentBoardSelection } from "@/components/assignments/assignment-board-dnd"
import { SummaryView } from "@/components/assignments/assignment-board-summary-view"
import type {
  DraftState,
  ProjectedAssignmentBoardSlot,
} from "@/components/assignments/draft-state"
import type { AssignmentBoardSlot } from "@/lib/types"

export function AssignmentBoardSidePanel({
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
  selection: AssignmentBoardSelection | null
  onSelectionChange: (selection: AssignmentBoardSelection | null) => void
  onDraftStateChange: Dispatch<SetStateAction<DraftState>>
}) {
  if (!selection) {
    return (
      <SummaryView
        projectedSlots={projectedSlots}
        onSelectionChange={onSelectionChange}
      />
    )
  }

  return (
    <CellEditor
      slots={slots}
      projectedSlots={projectedSlots}
      renderDraftState={renderDraftState}
      disabled={disabled}
      isReadOnly={isReadOnly}
      selection={selection}
      onSelectionChange={onSelectionChange}
      onDraftStateChange={onDraftStateChange}
    />
  )
}
