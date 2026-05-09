import { useMemo, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { Save, Trash2 } from "lucide-react"
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
  adminAttendanceDayQueryOptions,
  adminAttendanceShiftQueryOptions,
  adminClearArrival,
  adminCreateOvertime,
  adminDeleteOvertime,
  adminUpdateOvertime,
  adminUpsertArrival,
  updateAttendanceSettings,
} from "@/lib/queries"
import type { AttendanceRosterEntry, AttendanceShift } from "@/lib/types"

export const Route = createFileRoute(
  "/_authenticated/publications/$publicationId/attendance",
)({
  component: AdminAttendancePage,
})

export function AdminAttendancePage() {
  const { publicationId } = Route.useParams()
  const publicationID = Number(publicationId)
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { toast } = useToast()
  const [date, setDate] = useState(today())
  const [selectedSlotID, setSelectedSlotID] = useState<number | null>(null)
  const [arrivalDrafts, setArrivalDrafts] = useState<Record<string, string>>({})
  const [overtimeDraft, setOvertimeDraft] = useState({
    userID: "",
    hours: "",
    note: "",
  })
  const [settingsDraft, setSettingsDraft] = useState("")

  const dayQuery = useQuery(adminAttendanceDayQueryOptions(publicationID, date))
  const shifts = useMemo(() => dayQuery.data?.shifts ?? [], [dayQuery.data?.shifts])
  const selectedShiftSummary = useMemo(() => {
    if (selectedSlotID !== null) {
      return shifts.find((shift) => shift.slot_id === selectedSlotID) ?? null
    }
    return shifts[0] ?? null
  }, [selectedSlotID, shifts])
  const selectedShiftQuery = useQuery(
    adminAttendanceShiftQueryOptions(
      publicationID,
      selectedShiftSummary?.slot_id ?? 0,
      selectedShiftSummary?.occurrence_date ?? "",
    ),
  )
  const selectedShift = selectedShiftQuery.data

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({
        queryKey: ["publications", publicationID, "attendance"],
      }),
      queryClient.invalidateQueries({
        queryKey: ["publications", publicationID, "attendance", "shift"],
      }),
      queryClient.invalidateQueries({
        queryKey: ["publications", "detail", publicationID],
      }),
    ])
  }

  const arrivalMutation = useMutation({
    mutationFn: (row: AttendanceRosterEntry) => {
      if (!selectedShift) {
        throw new Error("missing shift")
      }
      return adminUpsertArrival(publicationID, {
        slot_id: selectedShift.slot_id,
        assignment_id: row.assignment_id,
        occurrence_date: selectedShift.occurrence_date,
        user_id: row.user_id,
        arrived_at: new Date(
          arrivalDrafts[rowKey(row)] ??
            toDateTimeLocal(row.record?.arrived_at ?? selectedShift.scheduled_start),
        ).toISOString(),
      })
    },
    onSuccess: async () => {
      await invalidate()
      toast({
        variant: "default",
        description: t("attendance.success.arrivalUpdated"),
      })
    },
    onError: (error) => showAttendanceError(error, t, toast),
  })

  const clearArrivalMutation = useMutation({
    mutationFn: (recordID: number) => adminClearArrival(publicationID, recordID),
    onSuccess: async () => {
      await invalidate()
      toast({
        variant: "default",
        description: t("attendance.success.arrivalCleared"),
      })
    },
    onError: (error) => showAttendanceError(error, t, toast),
  })

  const createOvertimeMutation = useMutation({
    mutationFn: () => {
      if (!selectedShift) {
        throw new Error("missing shift")
      }
      return adminCreateOvertime(publicationID, {
        slot_id: selectedShift.slot_id,
        occurrence_date: selectedShift.occurrence_date,
        user_id: Number(overtimeDraft.userID),
        hours: Number(overtimeDraft.hours),
        note: overtimeDraft.note,
      })
    },
    onSuccess: async () => {
      setOvertimeDraft({ userID: "", hours: "", note: "" })
      await invalidate()
      toast({
        variant: "default",
        description: t("attendance.success.overtimeRecorded"),
      })
    },
    onError: (error) => showAttendanceError(error, t, toast),
  })

  const updateOvertimeMutation = useMutation({
    mutationFn: (input: { recordID: number; hours: number; note: string }) =>
      adminUpdateOvertime(publicationID, input.recordID, {
        hours: input.hours,
        note: input.note,
      }),
    onSuccess: async () => {
      await invalidate()
      toast({
        variant: "default",
        description: t("attendance.success.overtimeUpdated"),
      })
    },
    onError: (error) => showAttendanceError(error, t, toast),
  })

  const deleteOvertimeMutation = useMutation({
    mutationFn: (recordID: number) => adminDeleteOvertime(publicationID, recordID),
    onSuccess: async () => {
      await invalidate()
      toast({
        variant: "default",
        description: t("attendance.success.overtimeDeleted"),
      })
    },
    onError: (error) => showAttendanceError(error, t, toast),
  })

  const settingsMutation = useMutation({
    mutationFn: () =>
      updateAttendanceSettings(
        publicationID,
        Number(
          settingsDraft !== ""
            ? settingsDraft
            : (dayQuery.data?.publication.overtime_entry_window_hours ?? 24),
        ),
      ),
    onSuccess: async () => {
      await invalidate()
      toast({
        variant: "default",
        description: t("attendance.success.settingsUpdated"),
      })
    },
    onError: (error) => showAttendanceError(error, t, toast),
  })

  const overtimeWindowValue =
    settingsDraft ||
    String(dayQuery.data?.publication.overtime_entry_window_hours ?? 24)

  return (
    <div className="grid gap-4">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">
          {t("attendance.adminTitle")}
        </h1>
        <p className="text-sm text-muted-foreground">
          {t("attendance.adminDescription")}
        </p>
      </div>

      <div className="grid gap-4 lg:grid-cols-[280px_1fr]">
        <div className="grid gap-4 content-start">
          <Card>
            <CardHeader>
              <CardTitle>{t("attendance.selectShift")}</CardTitle>
              <CardDescription>{date}</CardDescription>
            </CardHeader>
            <CardContent className="grid gap-3">
              <Input
                type="date"
                value={date}
                onChange={(event) => {
                  setDate(event.target.value)
                  setSelectedSlotID(null)
                }}
              />
              <div className="grid gap-2">
                {shifts.map((shift) => (
                  <Button
                    key={shift.slot_id}
                    type="button"
                    variant={
                      selectedShiftSummary?.slot_id === shift.slot_id
                        ? "default"
                        : "outline"
                    }
                    className="justify-start"
                    onClick={() => setSelectedSlotID(shift.slot_id)}
                  >
                    {t("attendance.summary.shift", {
                      date: shift.occurrence_date,
                      start: toTime(shift.scheduled_start),
                      end: toTime(shift.scheduled_end),
                    })}
                  </Button>
                ))}
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>{t("attendance.settings")}</CardTitle>
            </CardHeader>
            <CardContent>
              <form
                className="grid gap-3"
                onSubmit={(event) => {
                  event.preventDefault()
                  settingsMutation.mutate()
                }}
              >
                <Label htmlFor="attendance-overtime-window">
                  {t("attendance.overtimeWindow")}
                </Label>
                <Input
                  id="attendance-overtime-window"
                  type="number"
                  min={0}
                  max={168}
                  step="0.01"
                  value={overtimeWindowValue}
                  onChange={(event) => setSettingsDraft(event.target.value)}
                />
                <Button type="submit" disabled={settingsMutation.isPending}>
                  <Save className="size-4" />
                  {t("attendance.saveSettings")}
                </Button>
              </form>
            </CardContent>
          </Card>
        </div>

        {selectedShift ? (
          <Card>
            <CardHeader>
              <CardTitle>
                {t("attendance.summary.shift", {
                  date: selectedShift.occurrence_date,
                  start: selectedShift.start_time,
                  end: selectedShift.end_time,
                })}
              </CardTitle>
              <CardDescription>
                {t("attendance.summary.counts", {
                  present: selectedShiftSummary?.present_count ?? 0,
                  late: selectedShiftSummary?.late_count ?? 0,
                  pending: selectedShiftSummary?.pending_count ?? 0,
                  absent: selectedShiftSummary?.absent_count ?? 0,
                })}
              </CardDescription>
            </CardHeader>
            <CardContent className="grid gap-5">
              <div className="grid gap-2">
                {selectedShift.roster.map((row) => (
                  <AdminRosterRow
                    key={`${row.assignment_id}:${row.user_id}`}
                    row={row}
                    shift={selectedShift}
                    value={
                      arrivalDrafts[rowKey(row)] ??
                      toDateTimeLocal(row.record?.arrived_at ?? selectedShift.scheduled_start)
                    }
                    pending={arrivalMutation.isPending || clearArrivalMutation.isPending}
                    onChange={(value) =>
                      setArrivalDrafts((current) => ({
                        ...current,
                        [rowKey(row)]: value,
                      }))
                    }
                    onSave={() => arrivalMutation.mutate(row)}
                    onClear={() => {
                      if (row.record) {
                        clearArrivalMutation.mutate(row.record.id)
                      }
                    }}
                  />
                ))}
              </div>

              {selectedShift.orphan_arrivals.length > 0 && (
                <section className="grid gap-2 border-t pt-4">
                  <h2 className="font-medium">{t("attendance.orphanArrivals")}</h2>
                  {selectedShift.orphan_arrivals.map((record) => (
                    <div
                      key={record.id}
                      className="flex items-center justify-between rounded-lg border border-dashed p-3 text-sm"
                    >
                      <span>
                        {record.user_name} · {new Date(record.arrived_at).toLocaleString()}
                      </span>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() => clearArrivalMutation.mutate(record.id)}
                      >
                        {t("attendance.clearArrival")}
                      </Button>
                    </div>
                  ))}
                </section>
              )}

              <section className="grid gap-3 border-t pt-4">
                <h2 className="font-medium">{t("attendance.overtime")}</h2>
                <form
                  className="grid gap-3 sm:grid-cols-[1fr_120px_2fr_auto] sm:items-end"
                  onSubmit={(event) => {
                    event.preventDefault()
                    createOvertimeMutation.mutate()
                  }}
                >
                  <Input
                    type="number"
                    min={1}
                    placeholder={t("attendance.userId")}
                    value={overtimeDraft.userID}
                    onChange={(event) =>
                      setOvertimeDraft((current) => ({
                        ...current,
                        userID: event.target.value,
                      }))
                    }
                  />
                  <Input
                    type="number"
                    min="0.01"
                    step="0.01"
                    placeholder={t("attendance.hours")}
                    value={overtimeDraft.hours}
                    onChange={(event) =>
                      setOvertimeDraft((current) => ({
                        ...current,
                        hours: event.target.value,
                      }))
                    }
                  />
                  <Textarea
                    placeholder={t("attendance.note")}
                    value={overtimeDraft.note}
                    onChange={(event) =>
                      setOvertimeDraft((current) => ({
                        ...current,
                        note: event.target.value,
                      }))
                    }
                  />
                  <Button type="submit" disabled={createOvertimeMutation.isPending}>
                    {t("attendance.addOvertime")}
                  </Button>
                </form>

                {selectedShift.overtime_records.map((record) => (
                  <form
                    key={record.id}
                    className="grid gap-2 rounded-lg border p-3 sm:grid-cols-[1fr_120px_2fr_auto_auto] sm:items-center"
                    onSubmit={(event) => {
                      event.preventDefault()
                      const form = event.currentTarget
                      const data = new FormData(form)
                      updateOvertimeMutation.mutate({
                        recordID: record.id,
                        hours: Number(data.get("hours")),
                        note: String(data.get("note") ?? ""),
                      })
                    }}
                  >
                    <div className="text-sm">
                      <div className="font-medium">{record.user_name}</div>
                      <div className="text-muted-foreground">
                        #{record.user_id}
                      </div>
                    </div>
                    <Input
                      name="hours"
                      type="number"
                      min="0.01"
                      step="0.01"
                      defaultValue={record.hours}
                    />
                    <Input name="note" defaultValue={record.note} />
                    <Button type="submit" variant="outline" size="sm">
                      {t("attendance.updateOvertime")}
                    </Button>
                    <Button
                      type="button"
                      variant="destructive"
                      size="sm"
                      onClick={() => deleteOvertimeMutation.mutate(record.id)}
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </form>
                ))}
              </section>
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardContent className="p-6 text-sm text-muted-foreground">
              {dayQuery.isLoading ? t("common.loading") : t("attendance.empty")}
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  )
}

function AdminRosterRow({
  row,
  shift,
  value,
  pending,
  onChange,
  onSave,
  onClear,
}: {
  row: AttendanceRosterEntry
  shift: AttendanceShift
  value: string
  pending: boolean
  onChange: (value: string) => void
  onSave: () => void
  onClear: () => void
}) {
  const { t } = useTranslation()
  return (
    <div className="grid gap-3 rounded-lg border p-3 sm:grid-cols-[1fr_220px_auto_auto] sm:items-center">
      <div>
        <div className="font-medium">{row.user_name}</div>
        <div className="text-sm text-muted-foreground">
          {row.position_name} · {t(`attendance.status.${row.status}`)}
        </div>
      </div>
      <Input type="datetime-local" value={value} onChange={(event) => onChange(event.target.value)} />
      <Button type="button" disabled={pending} onClick={onSave}>
        {t("attendance.setArrival")}
      </Button>
      <Button
        type="button"
        variant="outline"
        disabled={pending || !row.record}
        onClick={onClear}
      >
        {t("attendance.clearArrival")}
      </Button>
      <input type="hidden" value={shift.slot_id} readOnly />
    </div>
  )
}

function showAttendanceError(
  error: unknown,
  t: (key: string) => string,
  toast: ReturnType<typeof useToast>["toast"],
) {
  toast({
    variant: "destructive",
    description: getTranslatedApiError(
      t,
      error,
      "attendance.errors",
      "attendance.errors.INTERNAL_ERROR",
    ),
  })
}

function rowKey(row: AttendanceRosterEntry) {
  return `${row.assignment_id}:${row.user_id}`
}

function today() {
  return new Date().toISOString().slice(0, 10)
}

function toTime(value: string) {
  return value.slice(11, 16)
}

function toDateTimeLocal(value: string) {
  const date = new Date(value)
  const offset = date.getTimezoneOffset()
  const local = new Date(date.getTime() - offset * 60_000)
  return local.toISOString().slice(0, 16)
}
