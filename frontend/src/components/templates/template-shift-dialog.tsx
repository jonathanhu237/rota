import { useEffect, useEffectEvent } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
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
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import type { Position, TemplateShift } from "@/lib/types"

import {
  createTemplateShiftSchema,
  type TemplateShiftFormValues,
} from "./template-schemas"

type TemplateShiftDialogProps = {
  mode: "create" | "edit"
  open: boolean
  initialWeekday?: number
  positions: Position[]
  shift?: TemplateShift | null
  isPending: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (values: TemplateShiftFormValues) => void
}

const weekdayOptions = [1, 2, 3, 4, 5, 6, 7] as const

export function TemplateShiftDialog({
  mode,
  open,
  initialWeekday,
  positions,
  shift,
  isPending,
  onOpenChange,
  onSubmit,
}: TemplateShiftDialogProps) {
  const { t, i18n } = useTranslation()
  const formSchema = createTemplateShiftSchema(t)

  const {
    register,
    handleSubmit,
    reset,
    trigger,
    formState: { errors },
  } = useForm<TemplateShiftFormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      weekday: shift?.weekday ?? initialWeekday ?? 1,
      start_time: shift?.start_time ?? "09:00",
      end_time: shift?.end_time ?? "10:00",
      position_id: shift?.position_id ?? 0,
      required_headcount: shift?.required_headcount ?? 1,
    },
  })

  useEffect(() => {
    reset({
      weekday: shift?.weekday ?? initialWeekday ?? 1,
      start_time: shift?.start_time ?? "09:00",
      end_time: shift?.end_time ?? "10:00",
      position_id: shift?.position_id ?? 0,
      required_headcount: shift?.required_headcount ?? 1,
    })
  }, [initialWeekday, open, reset, shift])

  const revalidateVisibleErrors = useEffectEvent(() => {
    const errorFields = Object.keys(errors) as (keyof TemplateShiftFormValues)[]
    if (errorFields.length > 0) {
      void trigger(errorFields)
    }
  })

  useEffect(() => {
    revalidateVisibleErrors()
  }, [i18n.language])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {mode === "create"
              ? t("templates.shiftDialog.createTitle")
              : t("templates.shiftDialog.editTitle")}
          </DialogTitle>
          <DialogDescription>
            {mode === "create"
              ? t("templates.shiftDialog.createDescription")
              : t("templates.shiftDialog.editDescription")}
          </DialogDescription>
        </DialogHeader>
        <form
          className="grid gap-4"
          onSubmit={handleSubmit((values) => onSubmit(values))}
        >
          <div className="grid gap-2">
            <Label htmlFor="template-shift-weekday">
              {t("templates.shift.weekday")}
            </Label>
            <select
              className="border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex h-10 w-full rounded-md border px-3 py-2 text-sm focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none"
              id="template-shift-weekday"
              {...register("weekday", { valueAsNumber: true })}
            >
              {weekdayOptions.map((weekday) => (
                <option key={weekday} value={weekday}>
                  {t(weekdayKeyMap[weekday])}
                </option>
              ))}
            </select>
            {errors.weekday && (
              <p className="text-sm text-destructive">
                {errors.weekday.message}
              </p>
            )}
          </div>
          <div className="grid gap-2 sm:grid-cols-2">
            <div className="grid gap-2">
              <Label htmlFor="template-shift-start">
                {t("templates.shift.startTime")}
              </Label>
              <Input
                id="template-shift-start"
                type="time"
                {...register("start_time")}
              />
              {errors.start_time && (
                <p className="text-sm text-destructive">
                  {errors.start_time.message}
                </p>
              )}
            </div>
            <div className="grid gap-2">
              <Label htmlFor="template-shift-end">
                {t("templates.shift.endTime")}
              </Label>
              <Input
                id="template-shift-end"
                type="time"
                {...register("end_time")}
              />
              {errors.end_time && (
                <p className="text-sm text-destructive">
                  {errors.end_time.message}
                </p>
              )}
            </div>
          </div>
          <div className="grid gap-2">
            <Label htmlFor="template-shift-position">
              {t("templates.shift.position")}
            </Label>
            <select
              className="border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex h-10 w-full rounded-md border px-3 py-2 text-sm focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none"
              id="template-shift-position"
              {...register("position_id", { valueAsNumber: true })}
            >
              <option value={0}>{t("templates.shiftDialog.selectPosition")}</option>
              {positions.map((position) => (
                <option key={position.id} value={position.id}>
                  {position.name}
                </option>
              ))}
            </select>
            {errors.position_id && (
              <p className="text-sm text-destructive">
                {errors.position_id.message}
              </p>
            )}
          </div>
          <div className="grid gap-2">
            <Label htmlFor="template-shift-headcount">
              {t("templates.shift.requiredHeadcount")}
            </Label>
            <Input
              id="template-shift-headcount"
              type="number"
              min={1}
              {...register("required_headcount", { valueAsNumber: true })}
            />
            {errors.required_headcount && (
              <p className="text-sm text-destructive">
                {errors.required_headcount.message}
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
            <Button type="submit" disabled={isPending}>
              {isPending
                ? mode === "create"
                  ? t("templates.shiftDialog.submittingCreate")
                  : t("templates.shiftDialog.submittingEdit")
                : mode === "create"
                  ? t("templates.shiftDialog.submitCreate")
                  : t("templates.shiftDialog.submitEdit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
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
