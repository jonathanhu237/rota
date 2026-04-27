import { useMemo } from "react"
import { useTranslation } from "react-i18next"

import type { AssignmentBoardSelection } from "@/components/assignments/assignment-board-dnd"
import {
  pivotIntoGridCells,
  type ScheduledGridCell,
} from "@/components/assignments/assignment-board-grid-cells"
import type { ProjectedAssignmentBoardSlot } from "@/components/assignments/draft-state"
import { weekdayKeys } from "@/components/assignments/assignment-board-side-panel-utils"

export function SummaryView({
  projectedSlots,
  onSelectionChange,
}: {
  projectedSlots: ProjectedAssignmentBoardSlot[]
  onSelectionChange: (selection: AssignmentBoardSelection) => void
}) {
  const { t } = useTranslation()
  const grid = useMemo(() => pivotIntoGridCells(projectedSlots), [projectedSlots])
  const scheduledCells = grid.cells
    .flat()
    .filter((cell): cell is ScheduledGridCell => cell.kind === "scheduled")
  const assigned = scheduledCells.reduce(
    (total, cell) => total + cell.totals.assigned,
    0,
  )
  const required = scheduledCells.reduce(
    (total, cell) => total + cell.totals.required,
    0,
  )
  const gaps = scheduledCells
    .filter((cell) => cell.totals.assigned < cell.totals.required)
    .sort((left, right) => {
      if (left.weekday !== right.weekday) {
        return left.weekday - right.weekday
      }

      if (left.slot.start_time !== right.slot.start_time) {
        return left.slot.start_time.localeCompare(right.slot.start_time)
      }

      return left.slot.end_time.localeCompare(right.slot.end_time)
    })

  return (
    <aside className="grid gap-4 rounded-lg border bg-card p-4">
      <div className="grid gap-1">
        <h3 className="font-medium">{t("assignments.summary.title")}</h3>
        <p className="text-sm text-muted-foreground">
          {t("assignments.headcount", { assigned, required })}
        </p>
      </div>

      <div className="grid gap-2">
        <div className="text-sm font-medium">
          {t("assignments.summary.coverageGaps", { count: gaps.length })}
        </div>
        {gaps.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {t("assignments.summary.noGaps")}
          </p>
        ) : (
          <div className="grid gap-2">
            {gaps.map((cell) => (
              <button
                key={`${cell.slotID}:${cell.weekday}`}
                type="button"
                className="rounded-md border bg-background px-3 py-2 text-left text-sm transition hover:bg-muted focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                onClick={() =>
                  onSelectionChange({
                    slotID: cell.slotID,
                    weekday: cell.weekday,
                  })
                }
              >
                <span className="font-medium">
                  {t(weekdayKeys[cell.weekday])}{" "}
                  {t("assignments.shiftSummary", {
                    startTime: cell.slot.start_time,
                    endTime: cell.slot.end_time,
                  })}
                </span>
                <span className="ml-2 text-muted-foreground">
                  {t("assignments.headcount", {
                    assigned: cell.totals.assigned,
                    required: cell.totals.required,
                  })}
                </span>
              </button>
            ))}
          </div>
        )}
      </div>
    </aside>
  )
}
