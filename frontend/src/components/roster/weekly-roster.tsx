import { MoreHorizontal } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"
import type {
  Publication,
  PublicationPosition,
  PublicationSlot,
  RosterWeekday,
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

export type WeeklyRosterShiftAction =
  | { type: "swap" }
  | { type: "give_direct" }
  | { type: "give_pool" }

export type WeeklyRosterOwnShift = {
  assignmentID: number
  weekday: number
  occurrenceDate: string
  slot: PublicationSlot
  position: PublicationPosition
}

type WeeklyRosterProps = {
  weekdays: RosterWeekday[]
  currentUserID?: number
  publication?: Publication | null
  onShiftAction?: (shift: WeeklyRosterOwnShift, action: WeeklyRosterShiftAction) => void
}

export function WeeklyRoster({
  weekdays,
  currentUserID,
  publication,
  onShiftAction,
}: WeeklyRosterProps) {
  const { t } = useTranslation()
  const today = new Date().getDay()
  const currentWeekday = today === 0 ? 7 : today

  const canPropose =
    publication?.state === "PUBLISHED" &&
    typeof currentUserID === "number" &&
    typeof onShiftAction === "function"

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
              {weekday.slots.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  {t("roster.emptyWeekday")}
                </p>
              ) : (
                weekday.slots.map((slotEntry) => (
                  <article
                    key={slotEntry.slot.id}
                    className="grid gap-4 rounded-xl border p-3"
                  >
                    <div className="grid gap-1">
                      <div className="font-medium">
                        {t("roster.shiftSummary", {
                          startTime: slotEntry.slot.start_time,
                          endTime: slotEntry.slot.end_time,
                          required: slotEntry.positions.reduce(
                            (count, position) =>
                              count + position.required_headcount,
                            0,
                          ),
                        })}
                      </div>
                      <div className="text-sm text-muted-foreground">
                        {slotEntry.positions.length}
                      </div>
                    </div>
                    <div className="grid gap-3 lg:grid-cols-2">
                      {slotEntry.positions.map((positionEntry) => (
                        <div
                          key={`${slotEntry.slot.id}-${positionEntry.position.id}`}
                          className="grid gap-3 rounded-xl border border-dashed p-3"
                        >
                          <div className="grid gap-1">
                            <div className="font-medium">
                              {positionEntry.position.name}
                            </div>
                            <div className="text-sm text-muted-foreground">
                              {t("roster.shiftSummary", {
                                startTime: slotEntry.slot.start_time,
                                endTime: slotEntry.slot.end_time,
                                required: positionEntry.required_headcount,
                              })}
                            </div>
                          </div>
                          {positionEntry.assignments.length === 0 ? (
                            <p className="text-sm text-muted-foreground">
                              {t("roster.emptyAssignments")}
                            </p>
                          ) : (
                            <div className="grid gap-2">
                              {positionEntry.assignments.map((assignment) => {
                                const isMine =
                                  typeof currentUserID === "number" &&
                                  assignment.user_id === currentUserID
                                const showActions = canPropose && isMine

                                return (
                                  <div
                                    key={`${slotEntry.slot.id}-${positionEntry.position.id}-${assignment.user_id}`}
                                    className={cn(
                                      "flex items-center justify-between gap-2 rounded-lg border px-3 py-2 text-sm",
                                      isMine &&
                                        "border-primary/40 bg-primary/10 text-primary",
                                    )}
                                  >
                                    <span>{assignment.name}</span>
                                    {showActions && (
                                      <DropdownMenu>
                                        <DropdownMenuTrigger
                                          render={
                                            <Button
                                              size="icon-xs"
                                              variant="ghost"
                                              aria-label={t("requests.actions.openMenu")}
                                            />
                                          }
                                        >
                                          <MoreHorizontal />
                                        </DropdownMenuTrigger>
                                        <DropdownMenuContent align="end">
                                          <DropdownMenuItem
                                            onClick={() =>
                                              onShiftAction!(
                                                {
                                                  assignmentID:
                                                    assignment.assignment_id,
                                                  weekday: weekday.weekday,
                                                  occurrenceDate:
                                                    slotEntry.occurrence_date,
                                                  slot: slotEntry.slot,
                                                  position: positionEntry.position,
                                                },
                                                { type: "swap" },
                                              )
                                            }
                                          >
                                            {t("requests.actions.proposeSwap")}
                                          </DropdownMenuItem>
                                          <DropdownMenuItem
                                            onClick={() =>
                                              onShiftAction!(
                                                {
                                                  assignmentID:
                                                    assignment.assignment_id,
                                                  weekday: weekday.weekday,
                                                  occurrenceDate:
                                                    slotEntry.occurrence_date,
                                                  slot: slotEntry.slot,
                                                  position: positionEntry.position,
                                                },
                                                { type: "give_direct" },
                                              )
                                            }
                                          >
                                            {t("requests.actions.giveDirect")}
                                          </DropdownMenuItem>
                                          <DropdownMenuItem
                                            onClick={() =>
                                              onShiftAction!(
                                                {
                                                  assignmentID:
                                                    assignment.assignment_id,
                                                  weekday: weekday.weekday,
                                                  occurrenceDate:
                                                    slotEntry.occurrence_date,
                                                  slot: slotEntry.slot,
                                                  position: positionEntry.position,
                                                },
                                                { type: "give_pool" },
                                              )
                                            }
                                          >
                                            {t("requests.actions.givePool")}
                                          </DropdownMenuItem>
                                        </DropdownMenuContent>
                                      </DropdownMenu>
                                    )}
                                  </div>
                                )
                              })}
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
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
