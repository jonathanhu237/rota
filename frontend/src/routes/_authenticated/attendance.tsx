import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { ClipboardCheck, Clock, Plus } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { useToast } from "@/components/ui/toast"
import { getTranslatedApiError } from "@/lib/api-error"
import {
  leaderAttendanceQueryOptions,
  recordLeaderArrival,
  recordLeaderOvertime,
} from "@/lib/queries"
import type { AttendanceRosterEntry, AttendanceShift } from "@/lib/types"

export const Route = createFileRoute("/_authenticated/attendance")({
  component: AttendancePage,
})

export function AttendancePage() {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const { toast } = useToast()
  const attendanceQuery = useQuery(leaderAttendanceQueryOptions)
  const [arrivalDrafts, setArrivalDrafts] = useState<Record<string, string>>({})
  const [overtimeDrafts, setOvertimeDrafts] = useState<
    Record<string, { userID: string; hours: string; note: string }>
  >({})

  const formatter = new Intl.DateTimeFormat(i18n.resolvedLanguage, {
    dateStyle: "medium",
    timeStyle: "short",
  })

  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: ["attendance", "current"] })
  }

  const arrivalMutation = useMutation({
    mutationFn: recordLeaderArrival,
    onSuccess: async () => {
      await invalidate()
      toast({
        variant: "default",
        description: t("attendance.success.arrivalRecorded"),
      })
    },
    onError: (error) => {
      toast({
        variant: "destructive",
        description: getTranslatedApiError(
          t,
          error,
          "attendance.errors",
          "attendance.errors.INTERNAL_ERROR",
        ),
      })
    },
  })

  const overtimeMutation = useMutation({
    mutationFn: recordLeaderOvertime,
    onSuccess: async (_, variables) => {
      setOvertimeDrafts((current) => ({
        ...current,
        [shiftKey(variables)]: { userID: "", hours: "", note: "" },
      }))
      await invalidate()
      toast({
        variant: "default",
        description: t("attendance.success.overtimeRecorded"),
      })
    },
    onError: (error) => {
      toast({
        variant: "destructive",
        description: getTranslatedApiError(
          t,
          error,
          "attendance.errors",
          "attendance.errors.INTERNAL_ERROR",
        ),
      })
    },
  })

  const shifts = attendanceQuery.data?.shifts ?? []

  return (
    <div className="grid gap-4">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">
          {t("attendance.title")}
        </h1>
        <p className="text-sm text-muted-foreground">
          {t("attendance.leaderDescription")}
        </p>
      </div>

      {attendanceQuery.isLoading ? (
        <Card>
          <CardContent className="p-6 text-sm text-muted-foreground">
            {t("common.loading")}
          </CardContent>
        </Card>
      ) : shifts.length === 0 ? (
        <Card>
          <CardContent className="p-6 text-sm text-muted-foreground">
            {t("attendance.empty")}
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4">
          {shifts.map((shift) => {
            const key = shiftKey(shift)
            const overtimeDraft = overtimeDrafts[key] ?? {
              userID: "",
              hours: "",
              note: "",
            }

            return (
              <Card key={key}>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2 text-lg">
                    <ClipboardCheck className="size-5" />
                    {t("attendance.summary.shift", {
                      date: shift.occurrence_date,
                      start: shift.start_time,
                      end: shift.end_time,
                    })}
                  </CardTitle>
                  <CardDescription>
                    {formatter.format(new Date(shift.scheduled_start))} -{" "}
                    {formatter.format(new Date(shift.scheduled_end))}
                  </CardDescription>
                </CardHeader>
                <CardContent className="grid gap-4">
                  <div className="grid gap-2">
                    {shift.roster.length === 0 ? (
                      <div className="rounded-lg border border-dashed p-3 text-sm text-muted-foreground">
                        {t("attendance.noRoster")}
                      </div>
                    ) : (
                      shift.roster.map((row) => (
                        <LeaderRosterRow
                          key={`${row.assignment_id}:${row.user_id}`}
                          row={row}
                          shift={shift}
                          value={
                            arrivalDrafts[rowKey(row)] ??
                            toDateTimeLocal(shift.scheduled_start)
                          }
                          min={toDateTimeLocal(shift.scheduled_start)}
                          max={toDateTimeLocal(new Date().toISOString())}
                          pending={arrivalMutation.isPending}
                          onChange={(value) =>
                            setArrivalDrafts((current) => ({
                              ...current,
                              [rowKey(row)]: value,
                            }))
                          }
                          onRecord={() =>
                            arrivalMutation.mutate({
                              publication_id: shift.publication_id,
                              slot_id: shift.slot_id,
                              assignment_id: row.assignment_id,
                              occurrence_date: shift.occurrence_date,
                              user_id: row.user_id,
                              arrived_at: new Date(
                                arrivalDrafts[rowKey(row)] ??
                                  toDateTimeLocal(shift.scheduled_start),
                              ).toISOString(),
                            })
                          }
                        />
                      ))
                    )}
                  </div>

                  {shift.overtime_window_open && (
                    <form
                      className="grid gap-3 border-t pt-4 sm:grid-cols-[1fr_120px_2fr_auto] sm:items-end"
                      onSubmit={(event) => {
                        event.preventDefault()
                        overtimeMutation.mutate({
                          publication_id: shift.publication_id,
                          slot_id: shift.slot_id,
                          occurrence_date: shift.occurrence_date,
                          user_id: Number(overtimeDraft.userID),
                          hours: Number(overtimeDraft.hours),
                          note: overtimeDraft.note,
                        })
                      }}
                    >
                      <div className="grid gap-1">
                        <Label htmlFor={`overtime-user-${key}`}>
                          {t("attendance.userId")}
                        </Label>
                        <Input
                          id={`overtime-user-${key}`}
                          type="number"
                          min={1}
                          value={overtimeDraft.userID}
                          onChange={(event) =>
                            setOvertimeDrafts((current) => ({
                              ...current,
                              [key]: {
                                ...overtimeDraft,
                                userID: event.target.value,
                              },
                            }))
                          }
                        />
                      </div>
                      <div className="grid gap-1">
                        <Label htmlFor={`overtime-hours-${key}`}>
                          {t("attendance.hours")}
                        </Label>
                        <Input
                          id={`overtime-hours-${key}`}
                          type="number"
                          min="0.01"
                          step="0.01"
                          value={overtimeDraft.hours}
                          onChange={(event) =>
                            setOvertimeDrafts((current) => ({
                              ...current,
                              [key]: {
                                ...overtimeDraft,
                                hours: event.target.value,
                              },
                            }))
                          }
                        />
                      </div>
                      <div className="grid gap-1">
                        <Label htmlFor={`overtime-note-${key}`}>
                          {t("attendance.note")}
                        </Label>
                        <Textarea
                          id={`overtime-note-${key}`}
                          value={overtimeDraft.note}
                          onChange={(event) =>
                            setOvertimeDrafts((current) => ({
                              ...current,
                              [key]: {
                                ...overtimeDraft,
                                note: event.target.value,
                              },
                            }))
                          }
                        />
                      </div>
                      <Button type="submit" disabled={overtimeMutation.isPending}>
                        <Plus className="size-4" />
                        {t("attendance.addOvertime")}
                      </Button>
                    </form>
                  )}
                </CardContent>
              </Card>
            )
          })}
        </div>
      )}
    </div>
  )
}

function LeaderRosterRow({
  row,
  shift,
  value,
  min,
  max,
  pending,
  onChange,
  onRecord,
}: {
  row: AttendanceRosterEntry
  shift: AttendanceShift
  value: string
  min: string
  max: string
  pending: boolean
  onChange: (value: string) => void
  onRecord: () => void
}) {
  const { t } = useTranslation()

  return (
    <div className="grid gap-3 rounded-lg border p-3 sm:grid-cols-[1fr_220px_auto] sm:items-center">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <div className="font-medium">{row.user_name}</div>
          <span className="rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground">
            {t(`attendance.status.${row.status}`)}
          </span>
          {row.attendance_responsible && (
            <span className="rounded-md bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary">
              {t("templates.position.attendanceResponsible")}
            </span>
          )}
        </div>
        <div className="text-sm text-muted-foreground">{row.position_name}</div>
      </div>
      {row.record ? (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Clock className="size-4" />
          {new Date(row.record.arrived_at).toLocaleString()}
        </div>
      ) : (
        <>
          <Input
            type="datetime-local"
            value={value}
            min={min}
            max={max}
            disabled={!shift.arrival_window_open}
            onChange={(event) => onChange(event.target.value)}
          />
          <Button
            type="button"
            disabled={!shift.arrival_window_open || pending}
            onClick={onRecord}
          >
            {t("attendance.recordArrival")}
          </Button>
        </>
      )}
    </div>
  )
}

function shiftKey(shift: Pick<AttendanceShift, "slot_id" | "occurrence_date">) {
  return `${shift.slot_id}:${shift.occurrence_date}`
}

function rowKey(row: AttendanceRosterEntry) {
  return `${row.assignment_id}:${row.user_id}`
}

function toDateTimeLocal(value: string) {
  const date = new Date(value)
  const offset = date.getTimezoneOffset()
  const local = new Date(date.getTime() - offset * 60_000)
  return local.toISOString().slice(0, 16)
}
