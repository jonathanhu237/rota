import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import type { RosterWeekday } from "@/lib/types"

const weekdayKeys = {
  1: "templates.weekday.mon",
  2: "templates.weekday.tue",
  3: "templates.weekday.wed",
  4: "templates.weekday.thu",
  5: "templates.weekday.fri",
  6: "templates.weekday.sat",
  7: "templates.weekday.sun",
} as const

type WeeklyRosterProps = {
  weekdays: RosterWeekday[]
  currentUserID?: number
}

export function WeeklyRoster({
  weekdays,
  currentUserID,
}: WeeklyRosterProps) {
  const { t } = useTranslation()
  const today = new Date().getDay()
  const currentWeekday = today === 0 ? 7 : today

  return (
    <div className="overflow-x-auto">
      <div className="grid min-w-[980px] gap-4 xl:grid-cols-7">
        {weekdays.map((weekday) => (
          <section
            key={weekday.weekday}
            className={cn(
              "rounded-xl border bg-card",
              weekday.weekday === currentWeekday &&
                "border-primary/30 ring-1 ring-primary/20",
            )}
          >
            <header className="flex items-center justify-between gap-2 border-b bg-muted/40 px-4 py-3">
              <h3 className="font-medium">
                {t(weekdayKeys[weekday.weekday as keyof typeof weekdayKeys])}
              </h3>
              {weekday.weekday === currentWeekday && (
                <Badge variant="default">{t("roster.today")}</Badge>
              )}
            </header>
            <div className="grid gap-3 p-4">
              {weekday.shifts.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  {t("roster.emptyWeekday")}
                </p>
              ) : (
                weekday.shifts.map((shift) => (
                  <article key={shift.shift.id} className="grid gap-3 rounded-xl border p-3">
                    <div className="grid gap-1">
                      <div className="font-medium">{shift.shift.position_name}</div>
                      <div className="text-sm text-muted-foreground">
                        {t("roster.shiftSummary", {
                          startTime: shift.shift.start_time,
                          endTime: shift.shift.end_time,
                          required: shift.shift.required_headcount,
                        })}
                      </div>
                    </div>
                    {shift.assignments.length === 0 ? (
                      <p className="text-sm text-muted-foreground">
                        {t("roster.emptyAssignments")}
                      </p>
                    ) : (
                      <div className="grid gap-2">
                        {shift.assignments.map((assignment) => (
                          <div
                            key={`${shift.shift.id}-${assignment.user_id}`}
                            className={cn(
                              "rounded-lg border px-3 py-2 text-sm",
                              assignment.user_id === currentUserID &&
                                "border-primary/40 bg-primary/10 text-primary",
                            )}
                          >
                            {assignment.name}
                          </div>
                        ))}
                      </div>
                    )}
                  </article>
                ))
              )}
            </div>
          </section>
        ))}
      </div>
    </div>
  )
}
