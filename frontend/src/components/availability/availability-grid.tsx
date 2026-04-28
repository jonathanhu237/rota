import { useMemo } from "react"
import { useTranslation } from "react-i18next"

import {
  pivotAvailabilityIntoGridCells,
  type AvailabilityTimeBlock,
  type QualifiedAvailabilityCell,
} from "@/components/availability/availability-grid-cells"
import { Badge } from "@/components/ui/badge"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"
import type {
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

type AvailabilityGridProps = {
  shifts: QualifiedShift[]
  selectedSlots: SlotRef[]
  isPending: boolean
  onToggle: (slotID: number, weekday: number, checked: boolean) => void
}

function getSlotWeekdayKey(slotID: number, weekday: number) {
  return `${slotID}:${weekday}`
}

export function AvailabilityGrid({
  shifts,
  selectedSlots,
  isPending,
  onToggle,
}: AvailabilityGridProps) {
  const { t } = useTranslation()
  const grid = useMemo(() => pivotAvailabilityIntoGridCells(shifts), [shifts])
  const todayWeekday = useMemo(() => {
    const day = new Date().getDay()
    return day === 0 ? 7 : day
  }, [])
  const selectedSlotSet = useMemo(
    () =>
      new Set(
        selectedSlots.map((slot) =>
          getSlotWeekdayKey(slot.slot_id, slot.weekday),
        ),
      ),
    [selectedSlots],
  )

  return (
    <div className="overflow-x-auto">
      <div
        role="grid"
        aria-label={t("availability.gridTitle")}
        className="grid min-w-[1090px] gap-px rounded-xl border bg-border"
        style={{
          gridTemplateColumns: "110px repeat(7, minmax(120px, 1fr))",
        }}
      >
        <div className="bg-card" />
        {grid.weekdays.map((weekday) => (
          <WeekdayHeader
            key={weekday}
            weekday={weekday}
            isToday={weekday === todayWeekday}
          />
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
                  <OffScheduleCell
                    key={`${timeBlock.index}:${cell.weekday}`}
                    timeBlockIndex={timeBlock.index}
                    weekday={cell.weekday}
                  />
                )
              }

              return (
                <QualifiedCell
                  key={`${timeBlock.index}:${cell.weekday}`}
                  cell={cell}
                  timeBlock={timeBlock}
                  checked={selectedSlotSet.has(
                    getSlotWeekdayKey(cell.slot_id, cell.weekday),
                  )}
                  isPending={isPending}
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

function WeekdayHeader({
  weekday,
  isToday,
}: {
  weekday: number
  isToday: boolean
}) {
  const { t } = useTranslation()

  return (
    <div
      role="columnheader"
      data-testid={`availability-weekday-header-${weekday}`}
      className={cn(
        "flex flex-col items-center justify-center gap-1 bg-card px-2 py-3",
        isToday && "bg-primary/10 text-primary",
      )}
    >
      <span className="text-sm font-medium">
        {t(weekdayKeys[weekday as keyof typeof weekdayKeys])}
      </span>
      {isToday && (
        <Badge variant="default" className="px-2 py-0 text-[10px]">
          {t("availability.today")}
        </Badge>
      )}
    </div>
  )
}

function QualifiedCell({
  cell,
  timeBlock,
  checked,
  isPending,
  onToggle,
}: {
  cell: QualifiedAvailabilityCell
  timeBlock: AvailabilityTimeBlock
  checked: boolean
  isPending: boolean
  onToggle: (slotID: number, weekday: number, checked: boolean) => void
}) {
  const { t } = useTranslation()
  const weekdayLabel = t(weekdayKeys[cell.weekday as keyof typeof weekdayKeys])
  const timeBlockLabel = formatTimeBlock(t, timeBlock)
  const compositionSummary = formatComposition(t, cell.composition)
  const accessibleLabel = `${weekdayLabel} ${timeBlockLabel} ${compositionSummary}`

  return (
    <div
      role="gridcell"
      data-testid={`availability-cell-${cell.timeBlockIndex}-${cell.weekday}`}
      className="flex min-h-14 items-center justify-center bg-card px-2 py-3"
    >
      <Tooltip>
        <TooltipTrigger
          render={
            <label className="inline-flex size-8 items-center justify-center rounded-sm" />
          }
        >
          <Checkbox
            aria-label={accessibleLabel}
            checked={checked}
            disabled={isPending}
            onChange={(event) =>
              onToggle(cell.slot_id, cell.weekday, event.currentTarget.checked)
            }
          />
        </TooltipTrigger>
        <TooltipContent>
          {t("availability.shift.composition", {
            summary: compositionSummary,
          })}
        </TooltipContent>
      </Tooltip>
    </div>
  )
}

function OffScheduleCell({
  timeBlockIndex,
  weekday,
}: {
  timeBlockIndex: number
  weekday: number
}) {
  const { t } = useTranslation()

  return (
    <div
      role="gridcell"
      data-testid={`availability-cell-${timeBlockIndex}-${weekday}`}
      aria-label={t("availability.offSchedule")}
      className="flex min-h-14 items-center justify-center border border-dashed bg-muted/40 text-muted-foreground"
    >
      <span aria-hidden="true">—</span>
    </div>
  )
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
