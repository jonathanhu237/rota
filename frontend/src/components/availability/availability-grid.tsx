import { useTranslation } from "react-i18next"

import { groupQualifiedShiftsByWeekday } from "@/components/templates/group-qualified-shifts"
import { Checkbox } from "@/components/ui/checkbox"
import type { QualifiedShift, SlotPositionRef } from "@/lib/types"

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
  selectedSlotPositions: SlotPositionRef[]
  isPending: boolean
  onToggle: (slotID: number, positionID: number, checked: boolean) => void
}

export function AvailabilityGrid({
  shifts,
  selectedSlotPositions,
  isPending,
  onToggle,
}: AvailabilityGridProps) {
  const { t } = useTranslation()
  const groupedShifts = groupQualifiedShiftsByWeekday(shifts)
  const selectedSlotPositionSet = new Set(
    selectedSlotPositions.map((position) =>
      slotPositionKey(position.slot_id, position.position_id),
    ),
  )

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      {Object.entries(groupedShifts).map(([weekday, weekdayShifts]) => {
        const numericWeekday = Number(weekday) as keyof typeof weekdayKeys

        return (
          <section key={weekday} className="rounded-xl border">
            <header className="border-b bg-muted/40 px-4 py-3">
              <h3 className="font-medium">
                {t(weekdayKeys[numericWeekday])}
              </h3>
            </header>
            <div className="grid gap-3 p-4">
              {weekdayShifts.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  {t("availability.noShiftsForWeekday")}
                </p>
              ) : (
                weekdayShifts.map((shift) => (
                  <label
                    key={slotPositionKey(shift.slot_id, shift.position_id)}
                    className="flex items-start gap-3 rounded-lg border p-3 transition-colors hover:bg-muted/20"
                  >
                    <Checkbox
                      checked={selectedSlotPositionSet.has(
                        slotPositionKey(shift.slot_id, shift.position_id),
                      )}
                      className="mt-0.5"
                      disabled={isPending}
                      onChange={(event) =>
                        onToggle(
                          shift.slot_id,
                          shift.position_id,
                          event.currentTarget.checked,
                        )
                      }
                    />
                    <div className="grid gap-1">
                      <div className="font-medium">
                        {t("availability.shift.timeRange", {
                          startTime: shift.start_time,
                          endTime: shift.end_time,
                        })}
                      </div>
                      <div className="text-sm text-muted-foreground">
                        {t("availability.shift.summary", {
                          positionId: shift.position_id,
                          headcount: shift.required_headcount,
                        })}
                      </div>
                    </div>
                  </label>
                ))
              )}
            </div>
          </section>
        )
      })}
    </div>
  )
}

function slotPositionKey(slotID: number, positionID: number) {
  return `${slotID}:${positionID}`
}
