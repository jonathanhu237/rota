import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"

import type { Employee } from "@/components/assignments/assignment-board-directory"
import { AssignmentBoardEmployeeRow } from "@/components/assignments/assignment-board-employee-row"
import { pivotIntoGridCells } from "@/components/assignments/assignment-board-grid-cells"
import {
  computeUserHours,
  type DraftState,
  type ProjectedAssignmentBoardSlot,
} from "@/components/assignments/draft-state"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import type { AssignmentBoardSlot } from "@/lib/types"

type SortMode = "hours" | "name"

export function AssignmentBoardSidePanel({
  slots,
  projectedSlots,
  renderDraftState,
  directory,
  disabled,
}: {
  slots: AssignmentBoardSlot[]
  projectedSlots: ProjectedAssignmentBoardSlot[]
  renderDraftState: DraftState
  directory: Map<number, Employee>
  disabled: boolean
}) {
  const { t } = useTranslation()
  const [search, setSearch] = useState("")
  const [sortMode, setSortMode] = useState<SortMode>("hours")
  const positionNames = useMemo(() => derivePositionNames(slots), [slots])
  const gapCount = useMemo(() => getGapCount(projectedSlots), [projectedSlots])
  const employees = useMemo(() => {
    const normalizedSearch = search.trim().toLowerCase()

    return [...directory.values()]
      .map((employee) => ({
        employee,
        totalHours: computeUserHours(slots, renderDraftState, employee.user_id),
      }))
      .filter(({ employee }) =>
        employee.name.toLowerCase().includes(normalizedSearch),
      )
      .sort((left, right) => {
        if (sortMode === "name") {
          return left.employee.name.localeCompare(right.employee.name)
        }

        if (left.totalHours !== right.totalHours) {
          return left.totalHours - right.totalHours
        }

        return left.employee.name.localeCompare(right.employee.name)
      })
  }, [directory, renderDraftState, search, slots, sortMode])

  return (
    <aside className="flex max-h-[760px] flex-col rounded-lg border bg-card">
      <header className="grid gap-3 border-b p-4">
        <div className="grid gap-1">
          <h3 className="font-medium">{t("assignments.directory.title")}</h3>
          <p className="text-sm text-muted-foreground">
            {gapCount > 0
              ? t("assignments.directory.gaps", { count: gapCount })
              : t("assignments.directory.noGaps")}
          </p>
        </div>

        <Input
          value={search}
          aria-label={t("assignments.directory.search")}
          placeholder={t("assignments.directory.search")}
          onChange={(event) => setSearch(event.target.value)}
        />

        <div className="flex gap-2">
          <Button
            type="button"
            size="sm"
            variant={sortMode === "hours" ? "default" : "outline"}
            onClick={() => setSortMode("hours")}
          >
            {t("assignments.directory.sortByHours")}
          </Button>
          <Button
            type="button"
            size="sm"
            variant={sortMode === "name" ? "default" : "outline"}
            onClick={() => setSortMode("name")}
          >
            {t("assignments.directory.sortByName")}
          </Button>
        </div>
      </header>

      <div className="grid gap-2 overflow-y-auto p-4">
        {employees.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {t("assignments.directory.empty")}
          </p>
        ) : (
          employees.map(({ employee, totalHours }) => (
            <AssignmentBoardEmployeeRow
              key={employee.user_id}
              employee={employee}
              totalHours={totalHours}
              positionNames={[...employee.position_ids]
                .map((positionID) => positionNames.get(positionID) ?? `#${positionID}`)
                .sort((left, right) => left.localeCompare(right))}
              disabled={disabled}
            />
          ))
        )}
      </div>
    </aside>
  )
}

function derivePositionNames(slots: AssignmentBoardSlot[]) {
  const positionNames = new Map<number, string>()
  for (const slotEntry of slots) {
    for (const positionEntry of slotEntry.positions) {
      positionNames.set(positionEntry.position.id, positionEntry.position.name)
    }
  }
  return positionNames
}

function getGapCount(slots: ProjectedAssignmentBoardSlot[]) {
  return pivotIntoGridCells(slots)
    .cells.flat()
    .filter(
      (cell) =>
        cell.kind === "scheduled" && cell.totals.assigned < cell.totals.required,
    ).length
}
