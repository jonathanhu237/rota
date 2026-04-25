import { useEffect, useEffectEvent, useMemo, useRef } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Controller, useForm, useWatch } from "react-hook-form"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { useToast } from "@/components/ui/toast"
import { getTranslatedApiError } from "@/lib/api-error"
import {
  createShiftChangeRequest,
  shiftChangeRequestsQueryOptions,
  unreadNotificationsQueryOptions,
} from "@/lib/queries"
import type {
  PublicationMember,
  PublicationPosition,
  PublicationSlot,
  RosterWeekday,
} from "@/lib/types"

import { findShiftsForMember } from "./roster-utils"
import {
  createSwapSchema,
  type SwapFormValues,
} from "./shift-change-schemas"

export type SwapDialogMyShift = {
  assignmentID: number
  weekday: number
  occurrenceDate: string
  slot: PublicationSlot
  position: PublicationPosition
}

type SwapDialogProps = {
  open: boolean
  publicationID: number
  myShift: SwapDialogMyShift | null
  members: PublicationMember[]
  rosterWeekdays: RosterWeekday[]
  onOpenChange: (open: boolean) => void
}

const weekdayKeyMap: Record<number, string> = {
  1: "templates.weekday.mon",
  2: "templates.weekday.tue",
  3: "templates.weekday.wed",
  4: "templates.weekday.thu",
  5: "templates.weekday.fri",
  6: "templates.weekday.sat",
  7: "templates.weekday.sun",
}

const selectClassName =
  "border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex h-10 w-full rounded-md border px-3 py-2 text-sm focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"

export function SwapDialog({
  open,
  publicationID,
  myShift,
  members,
  rosterWeekdays,
  onOpenChange,
}: SwapDialogProps) {
  const { t, i18n } = useTranslation()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const formSchema = createSwapSchema(t)

  const {
    control,
    handleSubmit,
    reset,
    trigger,
    setValue,
    formState: { errors },
  } = useForm<SwapFormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      counterpart_user_id: 0,
      counterpart_assignment_id: 0,
    },
  })

  const counterpartUserID = useWatch({ control, name: "counterpart_user_id" })

  useEffect(() => {
    if (open) {
      reset({ counterpart_user_id: 0, counterpart_assignment_id: 0 })
    }
  }, [open, reset, myShift?.assignmentID])

  const revalidateVisibleErrors = useEffectEvent(() => {
    const errorFields = Object.keys(errors) as (keyof SwapFormValues)[]
    if (errorFields.length > 0) {
      void trigger(errorFields)
    }
  })

  useEffect(() => {
    revalidateVisibleErrors()
  }, [i18n.language])

  const counterpartShifts = useMemo(() => {
    if (!counterpartUserID) {
      return []
    }
    return findShiftsForMember(rosterWeekdays, counterpartUserID)
  }, [counterpartUserID, rosterWeekdays])

  // Clear the shift choice when the counterpart changes so the picker
  // always reflects the new person's own shifts.
  const previousCounterpart = useRef<number>(0)
  useEffect(() => {
    if (counterpartUserID !== previousCounterpart.current) {
      previousCounterpart.current = counterpartUserID
      setValue("counterpart_assignment_id", 0, { shouldValidate: false })
    }
  }, [counterpartUserID, setValue])

  const mutation = useMutation({
    mutationFn: async (values: SwapFormValues) => {
      if (!myShift) {
        throw new Error("missing assignment")
      }
      const counterpartShift = counterpartShifts.find(
        (shift) => shift.assignmentID === values.counterpart_assignment_id,
      )
      if (!counterpartShift) {
        throw new Error("missing counterpart occurrence")
      }
      return createShiftChangeRequest(publicationID, {
        type: "swap",
        requester_assignment_id: myShift.assignmentID,
        occurrence_date: myShift.occurrenceDate,
        counterpart_user_id: values.counterpart_user_id,
        counterpart_assignment_id: values.counterpart_assignment_id,
        counterpart_occurrence_date: counterpartShift.occurrenceDate,
      })
    },
    onSuccess: async () => {
      toast({
        variant: "default",
        description: t("requests.swapDialog.success"),
      })
      onOpenChange(false)
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: shiftChangeRequestsQueryOptions(publicationID).queryKey,
        }),
        queryClient.invalidateQueries({
          queryKey: unreadNotificationsQueryOptions.queryKey,
        }),
        queryClient.invalidateQueries({ queryKey: ["roster", "current"] }),
      ])
    },
    onError: (error) => {
      toast({
        variant: "destructive",
        description: getTranslatedApiError(
          t,
          error,
          "requests.errors",
          "requests.errors.INTERNAL_ERROR",
        ),
      })
    },
  })

  const submitHandler = handleSubmit((values) => {
    mutation.mutate(values)
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("requests.swapDialog.title")}</DialogTitle>
          <DialogDescription>
            {t("requests.swapDialog.description")}
          </DialogDescription>
        </DialogHeader>
        {myShift && (
          <div className="rounded-lg border bg-muted/40 p-3 text-sm">
            <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              {t("requests.swapDialog.myShiftLabel")}
            </div>
            <div className="mt-1 font-medium">
              {t(weekdayKeyMap[myShift.weekday])} ·{" "}
              {myShift.position.name}
            </div>
            <div className="text-sm text-muted-foreground">
              {t("requests.swapDialog.shiftSummary", {
                startTime: myShift.slot.start_time,
                endTime: myShift.slot.end_time,
              })}
            </div>
            <div className="text-sm text-muted-foreground">
              {t("requests.occurrenceLabel", { date: myShift.occurrenceDate })}
            </div>
          </div>
        )}
        <form className="grid gap-4" onSubmit={submitHandler}>
          <div className="grid gap-2">
            <Label htmlFor="swap-counterpart-user">
              {t("requests.swapDialog.counterpartLabel")}
            </Label>
            <Controller
              control={control}
              name="counterpart_user_id"
              render={({ field }) => (
                <select
                  id="swap-counterpart-user"
                  className={selectClassName}
                  value={field.value === 0 ? "" : String(field.value)}
                  onBlur={field.onBlur}
                  onChange={(event) => {
                    const next = Number(event.target.value)
                    field.onChange(Number.isFinite(next) ? next : 0)
                  }}
                >
                  <option value="">
                    {t("requests.swapDialog.selectCounterpart")}
                  </option>
                  {members.map((member) => (
                    <option key={member.user_id} value={member.user_id}>
                      {member.name}
                    </option>
                  ))}
                </select>
              )}
            />
            {errors.counterpart_user_id && (
              <p className="text-sm text-destructive">
                {errors.counterpart_user_id.message}
              </p>
            )}
          </div>
          <div className="grid gap-2">
            <Label htmlFor="swap-counterpart-shift">
              {t("requests.swapDialog.counterpartShiftLabel")}
            </Label>
            <Controller
              control={control}
              name="counterpart_assignment_id"
              render={({ field }) => (
                <select
                  id="swap-counterpart-shift"
                  className={selectClassName}
                  disabled={!counterpartUserID || counterpartShifts.length === 0}
                  value={field.value === 0 ? "" : String(field.value)}
                  onBlur={field.onBlur}
                  onChange={(event) => {
                    const next = Number(event.target.value)
                    field.onChange(Number.isFinite(next) ? next : 0)
                  }}
                >
                  <option value="">
                    {t("requests.swapDialog.selectCounterpartShift")}
                  </option>
                  {counterpartShifts.map((option) => (
                    <option
                      key={option.assignmentID}
                      value={option.assignmentID}
                    >
                      {t(weekdayKeyMap[option.weekday])} ·{" "}
                      {option.position.name} ·{" "}
                      {t("requests.swapDialog.shiftSummary", {
                        startTime: option.slot.start_time,
                        endTime: option.slot.end_time,
                      })} · {option.occurrenceDate}
                    </option>
                  ))}
                </select>
              )}
            />
            {counterpartUserID > 0 && counterpartShifts.length === 0 && (
              <p className="text-sm text-muted-foreground">
                {t("requests.swapDialog.noCounterpartShifts")}
              </p>
            )}
            {errors.counterpart_assignment_id && (
              <p className="text-sm text-destructive">
                {errors.counterpart_assignment_id.message}
              </p>
            )}
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              {t("common.cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending || !myShift}>
              {mutation.isPending
                ? t("requests.swapDialog.submitting")
                : t("requests.swapDialog.submit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
