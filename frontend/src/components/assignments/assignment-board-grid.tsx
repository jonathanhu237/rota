import { useMemo } from "react"
import { useTranslation } from "react-i18next"

import {
  AssignmentBoardCell,
  type GridDropPreview,
} from "@/components/assignments/assignment-board-cell"
import {
  pivotIntoGridCells,
  type GridCell,
} from "@/components/assignments/assignment-board-grid-cells"
import type { AssignmentBoardSelection } from "@/components/assignments/assignment-board-dnd"
import type { AssignmentBoardSlot } from "@/lib/types"
import { cn } from "@/lib/utils"

const weekdayKeys: Record<number, string> = {
  1: "templates.weekday.mon",
  2: "templates.weekday.tue",
  3: "templates.weekday.wed",
  4: "templates.weekday.thu",
  5: "templates.weekday.fri",
  6: "templates.weekday.sat",
  7: "templates.weekday.sun",
}

export function AssignmentBoardGrid({
  slots,
  disabled,
  dropPreview,
  selection,
  onSelectionChange,
}: {
  slots: AssignmentBoardSlot[]
  disabled: boolean
  dropPreview: GridDropPreview | null
  selection: AssignmentBoardSelection | null
  onSelectionChange: (selection: AssignmentBoardSelection | null) => void
}) {
  const { t } = useTranslation()
  const grid = useMemo(() => pivotIntoGridCells(slots), [slots])

  return (
    <div className="overflow-x-auto rounded-lg border bg-card">
      <table className="w-full min-w-[920px] border-collapse">
        <thead>
          <tr className="border-b bg-muted/40">
            <th className="w-32 px-3 py-3 text-left text-xs font-medium text-muted-foreground">
              {t("assignments.grid.time")}
            </th>
            {grid.weekdays.map((weekday) => (
              <th
                key={weekday}
                className="px-2 py-3 text-left text-xs font-medium text-muted-foreground"
              >
                {t(weekdayKeys[weekday])}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {grid.timeBlocks.map((timeBlock, rowIndex) => (
            <tr
              key={`${timeBlock.start_time}-${timeBlock.end_time}`}
              className={cn(rowIndex > 0 && "border-t")}
            >
              <th className="px-3 py-3 align-middle text-sm font-medium">
                {t("assignments.shiftSummary", {
                  startTime: timeBlock.start_time,
                  endTime: timeBlock.end_time,
                })}
              </th>
              {grid.cells[rowIndex].map((cell) => (
                <td
                  key={getCellKey(cell)}
                  className="w-[12.5%] px-2 py-2 align-middle"
                >
                  <AssignmentBoardCell
                    cell={cell}
                    disabled={disabled}
                    dropPreview={dropPreview}
                    isSelected={
                      cell.kind === "scheduled" &&
                      selection?.slotID === cell.slotID &&
                      selection.weekday === cell.weekday
                    }
                    onSelect={(slotID, weekday) => {
                      if (
                        selection?.slotID === slotID &&
                        selection.weekday === weekday
                      ) {
                        onSelectionChange(null)
                        return
                      }

                      onSelectionChange({ slotID, weekday })
                    }}
                  />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function getCellKey(cell: GridCell) {
  if (cell.kind === "scheduled") {
    return `${cell.slotID}:${cell.weekday}`
  }

  return `off:${cell.timeBlockIndex}:${cell.weekday}`
}
