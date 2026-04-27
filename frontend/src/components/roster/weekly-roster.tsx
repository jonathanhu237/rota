import { useMemo } from "react"
import { MoreHorizontal } from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  pivotRosterIntoGridCells,
  type RosterCellTotals,
  type ScheduledRosterCell,
} from "@/components/roster/roster-grid-cells"
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
  RosterAssignment,
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
  onShiftAction?: (
    shift: WeeklyRosterOwnShift,
    action: WeeklyRosterShiftAction,
  ) => void
}

export function WeeklyRoster({
  weekdays,
  currentUserID,
  publication,
  onShiftAction,
}: WeeklyRosterProps) {
  const { t } = useTranslation()
  const todayWeekday = useMemo(() => {
    const day = new Date().getDay()
    return day === 0 ? 7 : day
  }, [])
  const grid = useMemo(() => pivotRosterIntoGridCells(weekdays), [weekdays])

  const canPropose =
    publication?.state === "PUBLISHED" &&
    typeof currentUserID === "number" &&
    typeof onShiftAction === "function"

  if (grid.timeBlocks.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">{t("roster.emptyWeek")}</p>
    )
  }

  return (
    <div className="overflow-x-auto">
      <div
        className="grid min-w-[1090px] gap-px rounded-xl border bg-border"
        style={{
          gridTemplateColumns: "110px repeat(7, minmax(140px, 1fr))",
        }}
      >
        {/* Header row */}
        <div className="bg-card" />
        {grid.weekdays.map((weekday) => (
          <WeekdayHeader
            key={weekday}
            weekday={weekday}
            isToday={weekday === todayWeekday}
          />
        ))}

        {/* Body rows */}
        {grid.timeBlocks.map((timeBlock) => (
          <div key={timeBlock.index} className="contents">
            <div className="flex items-start bg-card px-3 py-3 text-xs font-medium text-muted-foreground">
              {timeBlock.start_time}–{timeBlock.end_time}
            </div>
            {grid.cells[timeBlock.index].map((cell) => {
              if (cell.kind === "off-schedule") {
                return (
                  <OffScheduleCell
                    key={`${timeBlock.index}:${cell.weekday}`}
                  />
                )
              }
              return (
                <ScheduledCell
                  key={`${timeBlock.index}:${cell.weekday}`}
                  cell={cell}
                  isToday={cell.weekday === todayWeekday}
                  currentUserID={currentUserID}
                  canPropose={canPropose}
                  onShiftAction={onShiftAction}
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
          {t("roster.today")}
        </Badge>
      )}
    </div>
  )
}

function OffScheduleCell() {
  const { t } = useTranslation()
  return (
    <div
      role="presentation"
      aria-label={t("roster.offSchedule")}
      className="flex items-center justify-center bg-muted/40 text-muted-foreground"
    >
      <span aria-hidden="true">—</span>
    </div>
  )
}

function ScheduledCell({
  cell,
  isToday,
  currentUserID,
  canPropose,
  onShiftAction,
}: {
  cell: ScheduledRosterCell
  isToday: boolean
  currentUserID?: number
  canPropose: boolean
  onShiftAction?: (
    shift: WeeklyRosterOwnShift,
    action: WeeklyRosterShiftAction,
  ) => void
}) {
  const { t } = useTranslation()

  return (
    <div
      data-testid={`roster-cell-${cell.timeBlockIndex}-${cell.weekday}`}
      className={cn(
        "flex flex-col gap-2 p-2",
        statusBackground(cell.totals.status),
        isToday && "ring-1 ring-inset ring-primary/30",
      )}
    >
      <div className="flex items-center gap-1.5">
        <span
          className={cn("size-2 rounded-full", statusDot(cell.totals.status))}
          aria-hidden="true"
        />
        <span className="text-xs font-medium">
          {t("roster.cell.summary", {
            assigned: cell.totals.assigned,
            required: cell.totals.required,
          })}
        </span>
      </div>

      <div className="flex flex-col gap-2">
        {cell.positions.map((positionEntry) => (
          <PositionGroup
            key={positionEntry.position.id}
            positionName={positionEntry.position.name}
            requiredHeadcount={positionEntry.required_headcount}
            assignments={positionEntry.assignments}
            currentUserID={currentUserID}
            canPropose={canPropose}
            onSelfAction={
              onShiftAction
                ? (assignment, action) =>
                    onShiftAction(
                      {
                        assignmentID: assignment.assignment_id,
                        weekday: cell.weekday,
                        occurrenceDate: cell.occurrence_date,
                        slot: cell.slot,
                        position: positionEntry.position,
                      },
                      action,
                    )
                : undefined
            }
          />
        ))}
      </div>
    </div>
  )
}

function PositionGroup({
  positionName,
  requiredHeadcount,
  assignments,
  currentUserID,
  canPropose,
  onSelfAction,
}: {
  positionName: string
  requiredHeadcount: number
  assignments: RosterAssignment[]
  currentUserID?: number
  canPropose: boolean
  onSelfAction?: (
    assignment: RosterAssignment,
    action: WeeklyRosterShiftAction,
  ) => void
}) {
  const { t } = useTranslation()
  const emptyCount = Math.max(requiredHeadcount - assignments.length, 0)

  return (
    <div className="flex flex-col gap-1">
      <span className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
        {positionName}
      </span>
      <div className="flex flex-col gap-1">
        {assignments.map((assignment) => (
          <AssignmentChip
            key={`${assignment.assignment_id}-${assignment.user_id}`}
            assignment={assignment}
            isMine={
              typeof currentUserID === "number" &&
              assignment.user_id === currentUserID
            }
            canPropose={canPropose}
            onAction={onSelfAction}
          />
        ))}
        {Array.from({ length: emptyCount }).map((_, index) => (
          <div
            key={`empty-${index}`}
            className="rounded-md border border-dashed border-muted-foreground/40 bg-background/40 px-2 py-1 text-xs text-muted-foreground"
          >
            {t("roster.cell.empty")}
          </div>
        ))}
      </div>
    </div>
  )
}

function AssignmentChip({
  assignment,
  isMine,
  canPropose,
  onAction,
}: {
  assignment: RosterAssignment
  isMine: boolean
  canPropose: boolean
  onAction?: (
    assignment: RosterAssignment,
    action: WeeklyRosterShiftAction,
  ) => void
}) {
  const { t } = useTranslation()
  const showMenu = isMine && canPropose && typeof onAction === "function"

  return (
    <div
      className={cn(
        "flex items-center justify-between gap-1 rounded-md border px-2 py-1 text-xs",
        isMine
          ? "border-primary/40 bg-primary/10 text-primary"
          : "border-border bg-background",
      )}
    >
      <span className="truncate">{assignment.name}</span>
      {showMenu && (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button
                size="icon-xs"
                variant="ghost"
                className="size-5"
                aria-label={t("requests.actions.openMenu")}
              />
            }
          >
            <MoreHorizontal />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem
              onClick={() => onAction?.(assignment, { type: "swap" })}
            >
              {t("requests.actions.proposeSwap")}
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => onAction?.(assignment, { type: "give_direct" })}
            >
              {t("requests.actions.giveDirect")}
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => onAction?.(assignment, { type: "give_pool" })}
            >
              {t("requests.actions.givePool")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  )
}

function statusBackground(status: RosterCellTotals["status"]) {
  switch (status) {
    case "full":
      return "bg-emerald-50 dark:bg-emerald-950/30"
    case "partial":
      return "bg-amber-50 dark:bg-amber-950/30"
    case "empty":
      return "bg-red-50 dark:bg-red-950/30"
  }
}

function statusDot(status: RosterCellTotals["status"]) {
  switch (status) {
    case "full":
      return "bg-emerald-500"
    case "partial":
      return "bg-amber-500"
    case "empty":
      return "bg-red-500"
  }
}
