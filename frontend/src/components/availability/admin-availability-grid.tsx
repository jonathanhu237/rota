import { useMemo } from "react"
import { useTranslation } from "react-i18next"

import { getSlotWeekdayKey } from "@/components/availability/admin-availability-keys"
import {
  pivotAvailabilityIntoGridCells,
  type AvailabilityTimeBlock,
  type QualifiedAvailabilityCell,
} from "@/components/availability/availability-grid-cells"
import { Badge } from "@/components/ui/badge"
import { Checkbox } from "@/components/ui/checkbox"
import { cn } from "@/lib/utils"
import type {
  AdminAvailabilityCell,
  AdminAvailabilityDetail,
  QualifiedShift,
  QualifiedShiftComposition,
  SlotRef,
} from "@/lib/types"

const weekdayKeys = {
  1: "templates.weekday.mon",
  2: "templates.weekday.tue",
  3: "templates.weekday.wed",
  4: "templates.weekday.thu",
  5: "templates.weekday.fri",
  6: "templates.weekday.sat",
  7: "templates.weekday.sun",
} as const

type AdminAvailabilityGridProps = {
  detail: AdminAvailabilityDetail
  selectedSlots: SlotRef[]
  isPending: boolean
  isReadOnly: boolean
  onToggle: (slotID: number, weekday: number, checked: boolean) => void
}

export function AdminAvailabilityGrid({
  detail,
  selectedSlots,
  isPending,
  isReadOnly,
  onToggle,
}: AdminAvailabilityGridProps) {
  const { t } = useTranslation()
  const shifts = useMemo(() => adminAvailabilityShifts(detail), [detail])
  const grid = useMemo(() => pivotAvailabilityIntoGridCells(shifts), [shifts])
  const selectedSlotSet = useMemo(
    () =>
      new Set(
        selectedSlots.map((slot) =>
          getSlotWeekdayKey(slot.slot_id, slot.weekday),
        ),
      ),
    [selectedSlots],
  )
  const cellLookup = useMemo(() => {
    const lookup = new Map<string, AdminAvailabilityCell>()
    for (const cell of detail.cells) {
      lookup.set(getSlotWeekdayKey(cell.slot_id, cell.weekday), cell)
    }
    return lookup
  }, [detail.cells])

  return (
    <div className="overflow-x-auto">
      <div
        role="grid"
        aria-label={t("adminAvailability.editor.gridTitle")}
        className="grid min-w-[1090px] gap-px rounded-xl border bg-border"
        style={{
          gridTemplateColumns: "110px repeat(7, minmax(120px, 1fr))",
        }}
      >
        <div className="bg-card" />
        {grid.weekdays.map((weekday) => (
          <div
            key={weekday}
            role="columnheader"
            className="flex items-center justify-center bg-card px-2 py-3 text-sm font-medium"
          >
            {t(weekdayKeys[weekday as keyof typeof weekdayKeys])}
          </div>
        ))}

        {grid.timeBlocks.map((timeBlock) => (
          <div key={timeBlock.index} className="contents">
            <div
              role="rowheader"
              className="flex items-start bg-card px-3 py-3 text-xs font-medium text-muted-foreground"
            >
              {formatTimeBlock(t, timeBlock)}
            </div>
            {grid.cells[timeBlock.index].map((cell) => {
              if (cell.kind === "off-schedule") {
                return (
                  <div
                    key={`${timeBlock.index}:${cell.weekday}`}
                    role="gridcell"
                    aria-label={t("availability.offSchedule")}
                    className="flex min-h-20 items-center justify-center border border-dashed bg-muted/40 text-muted-foreground"
                  >
                    <span aria-hidden="true">-</span>
                  </div>
                )
              }

              const key = getSlotWeekdayKey(cell.slot_id, cell.weekday)
              const metadata = cellLookup.get(key)
              const checked = selectedSlotSet.has(key)
              const isEligible = metadata?.eligible ?? false
              const isOriginalSubmitted = metadata?.submitted ?? false

              return (
                <AdminAvailabilityGridCell
                  key={key}
                  cell={cell}
                  timeBlock={timeBlock}
                  checked={checked}
                  isEligible={isEligible}
                  isOriginalSubmitted={isOriginalSubmitted}
                  isPending={isPending}
                  isReadOnly={isReadOnly}
                  onToggle={onToggle}
                />
              )
            })}
          </div>
        ))}
      </div>
    </div>
  )
}

function AdminAvailabilityGridCell({
  cell,
  timeBlock,
  checked,
  isEligible,
  isOriginalSubmitted,
  isPending,
  isReadOnly,
  onToggle,
}: {
  cell: QualifiedAvailabilityCell
  timeBlock: AvailabilityTimeBlock
  checked: boolean
  isEligible: boolean
  isOriginalSubmitted: boolean
  isPending: boolean
  isReadOnly: boolean
  onToggle: (slotID: number, weekday: number, checked: boolean) => void
}) {
  const { t } = useTranslation()
  const weekdayLabel = t(weekdayKeys[cell.weekday as keyof typeof weekdayKeys])
  const timeBlockLabel = formatTimeBlock(t, timeBlock)
  const compositionSummary = formatComposition(t, cell.composition)
  const disabled = isPending || isReadOnly || (!isEligible && !checked)
  const exception = isOriginalSubmitted && !isEligible

  return (
    <div
      role="gridcell"
      data-testid={`admin-availability-cell-${cell.slot_id}-${cell.weekday}`}
      className={cn(
        "flex min-h-20 flex-col items-center justify-center gap-2 bg-card px-2 py-3",
        !isEligible && "bg-muted/40 text-muted-foreground",
        exception && "bg-destructive/5",
      )}
    >
      <label className="inline-flex items-center gap-2 text-sm">
        <Checkbox
          aria-label={t("adminAvailability.editor.cellLabel", {
            weekday: weekdayLabel,
            time: timeBlockLabel,
            summary: compositionSummary,
          })}
          checked={checked}
          disabled={disabled}
          onChange={(event) =>
            onToggle(cell.slot_id, cell.weekday, event.currentTarget.checked)
          }
        />
        <span className="sr-only">{compositionSummary}</span>
      </label>
      <div className="flex flex-wrap justify-center gap-1">
        {isEligible ? (
          <Badge variant="secondary">
            {t("adminAvailability.editor.eligible")}
          </Badge>
        ) : (
          <Badge variant="outline">
            {t("adminAvailability.editor.ineligible")}
          </Badge>
        )}
        {exception && (
          <Badge variant="destructive">
            {t("adminAvailability.editor.exception")}
          </Badge>
        )}
      </div>
    </div>
  )
}

function adminAvailabilityShifts(
  detail: AdminAvailabilityDetail,
): QualifiedShift[] {
  return detail.slots.map((slot): QualifiedShift => ({
    slot_id: slot.slot.id,
    weekday: slot.slot.weekday,
    start_time: slot.slot.start_time,
    end_time: slot.slot.end_time,
    composition: slot.positions.map((position): QualifiedShiftComposition => ({
      position_id: position.position.id,
      position_name: position.position.name,
      required_headcount: position.required_headcount,
    })),
  }))
}

function formatTimeBlock(
  t: (key: string, options?: Record<string, unknown>) => string,
  timeBlock: AvailabilityTimeBlock,
) {
  return t("availability.shift.timeRange", {
    startTime: timeBlock.start_time,
    endTime: timeBlock.end_time,
  })
}

function formatComposition(
  t: (key: string, options?: Record<string, unknown>) => string,
  composition: QualifiedShiftComposition[],
) {
  return composition
    .map((entry) =>
      t("availability.shift.compositionEntry", {
        position: entry.position_name,
        count: entry.required_headcount,
      }),
    )
    .join(" / ")
}
