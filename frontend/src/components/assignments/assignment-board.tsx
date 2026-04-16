import { useTranslation } from "react-i18next"

import { isAssignmentBoardShiftUnderstaffed } from "@/components/assignments/assignment-board-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import type { AssignmentBoardShift } from "@/lib/types"

import {
  assignmentBoardWeekdays,
  groupAssignmentBoardShiftsByWeekday,
} from "./group-assignment-board-shifts"

const weekdayKeys = {
  1: "templates.weekday.mon",
  2: "templates.weekday.tue",
  3: "templates.weekday.wed",
  4: "templates.weekday.thu",
  5: "templates.weekday.fri",
  6: "templates.weekday.sat",
  7: "templates.weekday.sun",
} as const

type AssignmentBoardProps = {
  shifts: AssignmentBoardShift[]
  isPending: boolean
  isReadOnly: boolean
  onAssign: (userID: number, templateShiftID: number) => void
  onUnassign: (assignmentID: number) => void
}

export function AssignmentBoard({
  shifts,
  isPending,
  isReadOnly,
  onAssign,
  onUnassign,
}: AssignmentBoardProps) {
  const { t } = useTranslation()
  const groupedShifts = groupAssignmentBoardShiftsByWeekday(shifts)

  return (
    <div className="grid gap-4 xl:grid-cols-2">
      {assignmentBoardWeekdays.map((weekday) => (
        <section key={weekday} className="rounded-xl border bg-card">
          <header className="border-b bg-muted/40 px-4 py-3">
            <h3 className="font-medium">{t(weekdayKeys[weekday])}</h3>
          </header>
          <div className="grid gap-4 p-4">
            {groupedShifts[weekday].length === 0 ? (
              <p className="text-sm text-muted-foreground">
                {t("assignments.emptyWeekday")}
              </p>
            ) : (
              groupedShifts[weekday].map((entry) => {
                const understaffed = isAssignmentBoardShiftUnderstaffed(entry)

                return (
                  <article
                    key={entry.shift.id}
                    className={cn(
                      "grid gap-4 rounded-xl border p-4",
                      understaffed && "border-amber-300 bg-amber-50/60 dark:border-amber-900 dark:bg-amber-950/20",
                    )}
                  >
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div className="grid gap-1">
                        <div className="font-medium">{entry.shift.position_name}</div>
                        <div className="text-sm text-muted-foreground">
                          {t("assignments.shiftSummary", {
                            startTime: entry.shift.start_time,
                            endTime: entry.shift.end_time,
                          })}
                        </div>
                      </div>
                      <div className="flex flex-wrap items-center gap-2">
                        <Badge
                          variant={understaffed ? "destructive" : "secondary"}
                        >
                          {t("assignments.headcount", {
                            assigned: entry.assignments.length,
                            required: entry.shift.required_headcount,
                          })}
                        </Badge>
                        {understaffed && (
                          <Badge variant="outline">
                            {t("assignments.understaffed")}
                          </Badge>
                        )}
                      </div>
                    </div>

                    <div className="grid gap-2">
                      <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                        {t("assignments.candidates")}
                      </div>
                      {entry.candidates.length === 0 ? (
                        <p className="text-sm text-muted-foreground">
                          {t("assignments.emptyCandidates")}
                        </p>
                      ) : (
                        <div className="flex flex-wrap gap-2">
                          {entry.candidates.map((candidate) => (
                            <Button
                              key={`${entry.shift.id}-${candidate.user_id}`}
                              size="sm"
                              variant="outline"
                              disabled={isPending || isReadOnly}
                              onClick={() =>
                                onAssign(candidate.user_id, entry.shift.id)
                              }
                              title={candidate.email}
                            >
                              {candidate.name}
                            </Button>
                          ))}
                        </div>
                      )}
                    </div>

                    <div className="grid gap-2">
                      <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                        {t("assignments.assigned")}
                      </div>
                      {entry.assignments.length === 0 ? (
                        <p className="text-sm text-muted-foreground">
                          {t("assignments.emptyAssignments")}
                        </p>
                      ) : (
                        <div className="flex flex-wrap gap-2">
                          {entry.assignments.map((assignment) =>
                            isReadOnly ? (
                              <Badge
                                key={assignment.assignment_id}
                                variant="secondary"
                                className="px-3 py-1"
                                title={assignment.email}
                              >
                                {assignment.name}
                              </Badge>
                            ) : (
                              <Button
                                key={assignment.assignment_id}
                                size="sm"
                                variant="secondary"
                                disabled={isPending}
                                onClick={() => onUnassign(assignment.assignment_id)}
                                title={assignment.email}
                              >
                                {assignment.name}
                              </Button>
                            ),
                          )}
                        </div>
                      )}
                    </div>
                  </article>
                )
              })
            )}
          </div>
        </section>
      ))}
    </div>
  )
}
