import { useState } from "react"
import { useTranslation } from "react-i18next"

import {
  getVisibleNonCandidateQualified,
  isAssignmentBoardPositionUnderstaffed,
} from "@/components/assignments/assignment-board-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Switch } from "@/components/ui/switch"
import { cn } from "@/lib/utils"
import type { AssignmentBoardSlot } from "@/lib/types"

import {
  assignmentBoardWeekdays,
  groupAssignmentBoardSlotsByWeekday,
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
  slots: AssignmentBoardSlot[]
  isPending: boolean
  isReadOnly: boolean
  onAssign: (userID: number, slotID: number, positionID: number) => void
  onUnassign: (assignmentID: number) => void
}

export function AssignmentBoard({
  slots,
  isPending,
  isReadOnly,
  onAssign,
  onUnassign,
}: AssignmentBoardProps) {
  const { t } = useTranslation()
  const [showAllQualified, setShowAllQualified] = useState(false)
  const groupedSlots = groupAssignmentBoardSlotsByWeekday(slots)

  return (
    <div className="grid gap-4">
      <div className="flex items-center justify-between rounded-xl border border-dashed bg-muted/30 px-4 py-3">
        <div className="grid gap-1">
          <div className="text-sm font-medium">
            {t("publications.assignmentBoard.showAllQualified")}
          </div>
        </div>
        <Switch
          aria-label={t("publications.assignmentBoard.showAllQualified")}
          checked={showAllQualified}
          onCheckedChange={setShowAllQualified}
        />
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        {assignmentBoardWeekdays.map((weekday) => (
          <section key={weekday} className="rounded-xl border bg-card">
            <header className="border-b bg-muted/40 px-4 py-3">
              <h3 className="font-medium">{t(weekdayKeys[weekday])}</h3>
            </header>
            <div className="grid gap-4 p-4">
              {groupedSlots[weekday].length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  {t("assignments.emptyWeekday")}
                </p>
              ) : (
                groupedSlots[weekday].map((slotEntry) => {
                  const slotUnderstaffed = slotEntry.positions.some((position) =>
                    isAssignmentBoardPositionUnderstaffed(position),
                  )

                  return (
                    <article
                      key={slotEntry.slot.id}
                      className={cn(
                        "grid gap-4 rounded-xl border p-4",
                        slotUnderstaffed &&
                          "border-amber-300 bg-amber-50/60 dark:border-amber-900 dark:bg-amber-950/20",
                      )}
                    >
                      <div className="flex flex-wrap items-start justify-between gap-3">
                        <div className="grid gap-1">
                          <div className="font-medium">
                            {t("assignments.shiftSummary", {
                              startTime: slotEntry.slot.start_time,
                              endTime: slotEntry.slot.end_time,
                            })}
                          </div>
                          <div className="text-sm text-muted-foreground">
                            {t("assignments.headcount", {
                              assigned: slotEntry.positions.reduce(
                                (count, position) =>
                                  count + position.assignments.length,
                                0,
                              ),
                              required: slotEntry.positions.reduce(
                                (count, position) =>
                                  count + position.required_headcount,
                                0,
                              ),
                            })}
                          </div>
                        </div>
                        <Badge variant="secondary">
                          {slotEntry.positions.length}
                        </Badge>
                      </div>

                      <div className="grid gap-3 lg:grid-cols-2">
                        {slotEntry.positions.map((positionEntry) => {
                          const understaffed =
                            isAssignmentBoardPositionUnderstaffed(positionEntry)
                          const visibleNonCandidateQualified =
                            getVisibleNonCandidateQualified(
                              slotEntry,
                              positionEntry,
                              showAllQualified,
                            )
                          const hasVisibleQualifiedOptions =
                            positionEntry.candidates.length > 0 ||
                            visibleNonCandidateQualified.length > 0

                          return (
                            <section
                              key={`${slotEntry.slot.id}-${positionEntry.position.id}`}
                              className={cn(
                                "grid gap-4 rounded-xl border p-4",
                                understaffed &&
                                  "border-amber-300 bg-amber-50/60 dark:border-amber-900 dark:bg-amber-950/20",
                              )}
                            >
                              <div className="flex flex-wrap items-start justify-between gap-3">
                                <div className="grid gap-1">
                                  <div className="font-medium">
                                    {positionEntry.position.name}
                                  </div>
                                  <div className="text-sm text-muted-foreground">
                                    {t("assignments.headcount", {
                                      assigned: positionEntry.assignments.length,
                                      required: positionEntry.required_headcount,
                                    })}
                                  </div>
                                </div>
                                <div className="flex flex-wrap items-center gap-2">
                                  <Badge
                                    variant={
                                      understaffed ? "destructive" : "secondary"
                                    }
                                  >
                                    {t("assignments.headcount", {
                                      assigned: positionEntry.assignments.length,
                                      required: positionEntry.required_headcount,
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
                                {!hasVisibleQualifiedOptions ? (
                                  <p className="text-sm text-muted-foreground">
                                    {t("assignments.emptyCandidates")}
                                  </p>
                                ) : (
                                  <div className="grid gap-2">
                                    {positionEntry.candidates.length > 0 && (
                                      <div className="flex flex-wrap gap-2">
                                        {positionEntry.candidates.map((candidate) => (
                                          <Button
                                            key={`${slotEntry.slot.id}-${positionEntry.position.id}-${candidate.user_id}`}
                                            size="sm"
                                            variant="outline"
                                            disabled={isPending || isReadOnly}
                                            onClick={() =>
                                              onAssign(
                                                candidate.user_id,
                                                slotEntry.slot.id,
                                                positionEntry.position.id,
                                              )
                                            }
                                            title={candidate.email}
                                          >
                                            {candidate.name}
                                          </Button>
                                        ))}
                                      </div>
                                    )}

                                    {visibleNonCandidateQualified.length > 0 && (
                                      <div className="flex flex-wrap gap-2 border-t border-dashed pt-2">
                                        {visibleNonCandidateQualified.map((candidate) => (
                                          <Button
                                            key={`qualified-${slotEntry.slot.id}-${positionEntry.position.id}-${candidate.user_id}`}
                                            size="sm"
                                            variant="outline"
                                            className="h-auto items-start border-dashed px-3 py-2 text-left"
                                            disabled={isPending || isReadOnly}
                                            onClick={() =>
                                              onAssign(
                                                candidate.user_id,
                                                slotEntry.slot.id,
                                                positionEntry.position.id,
                                              )
                                            }
                                            title={candidate.email}
                                          >
                                            <span>{candidate.name}</span>
                                            <span className="text-[10px] font-normal text-muted-foreground">
                                              {t(
                                                "publications.assignmentBoard.didNotSubmitAvailability",
                                              )}
                                            </span>
                                          </Button>
                                        ))}
                                      </div>
                                    )}
                                  </div>
                                )}
                              </div>

                              <div className="grid gap-2">
                                <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                  {t("assignments.assigned")}
                                </div>
                                {positionEntry.assignments.length === 0 ? (
                                  <p className="text-sm text-muted-foreground">
                                    {t("assignments.emptyAssignments")}
                                  </p>
                                ) : (
                                  <div className="flex flex-wrap gap-2">
                                    {positionEntry.assignments.map((assignment) =>
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
                                          onClick={() =>
                                            onUnassign(assignment.assignment_id)
                                          }
                                          title={assignment.email}
                                        >
                                          {assignment.name}
                                        </Button>
                                      ),
                                    )}
                                  </div>
                                )}
                              </div>
                            </section>
                          )
                        })}
                      </div>
                    </article>
                  )
                })
              )}
            </div>
          </section>
        ))}
      </div>
    </div>
  )
}
