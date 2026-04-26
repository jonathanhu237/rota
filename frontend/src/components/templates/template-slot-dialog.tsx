import { useEffect, useEffectEvent } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm, useWatch } from "react-hook-form"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
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
import type { TemplateSlot } from "@/lib/types"

import {
  createTemplateSlotSchema,
  type TemplateSlotFormValues,
} from "./template-schemas"

type TemplateSlotDialogProps = {
  mode: "create" | "edit"
  open: boolean
  slot?: TemplateSlot | null
  isPending: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (values: TemplateSlotFormValues) => void
}

const weekdayOptions = [1, 2, 3, 4, 5, 6, 7] as const
const defaultWeekdays = [1, 2, 3, 4, 5]

const weekdayKeyMap: Record<number, string> = {
  1: "templates.weekday.mon",
  2: "templates.weekday.tue",
  3: "templates.weekday.wed",
  4: "templates.weekday.thu",
  5: "templates.weekday.fri",
  6: "templates.weekday.sat",
  7: "templates.weekday.sun",
}

export function TemplateSlotDialog({
  mode,
  open,
  slot,
  isPending,
  onOpenChange,
  onSubmit,
}: TemplateSlotDialogProps) {
  const { t, i18n } = useTranslation()
  const formSchema = createTemplateSlotSchema(t)

  const {
    register,
    handleSubmit,
    reset,
    setValue,
    control,
    trigger,
    formState: { errors },
  } = useForm<TemplateSlotFormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      weekdays: slot?.weekdays ?? defaultWeekdays,
      start_time: slot?.start_time ?? "09:00",
      end_time: slot?.end_time ?? "10:00",
    },
  })
  const selectedWeekdays = useWatch({ control, name: "weekdays" }) ?? []

  useEffect(() => {
    reset({
      weekdays: slot?.weekdays ?? defaultWeekdays,
      start_time: slot?.start_time ?? "09:00",
      end_time: slot?.end_time ?? "10:00",
    })
  }, [open, reset, slot])

  const revalidateVisibleErrors = useEffectEvent(() => {
    const errorFields = Object.keys(errors) as (keyof TemplateSlotFormValues)[]
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
              ? t("templates.slotDialog.createTitle")
              : t("templates.slotDialog.editTitle")}
          </DialogTitle>
          <DialogDescription>
            {mode === "create"
              ? t("templates.slotDialog.createDescription")
              : t("templates.slotDialog.editDescription")}
          </DialogDescription>
        </DialogHeader>
        <form
          className="grid gap-4"
          onSubmit={handleSubmit((values) => onSubmit(values))}
        >
          <div className="grid gap-2">
            <Label>{t("templates.slot.weekday")}</Label>
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
              {weekdayOptions.map((weekday) => (
                <label
                  key={weekday}
                  className="flex items-center gap-2 rounded-md border px-3 py-2 text-sm"
                >
                  <Checkbox
                    checked={selectedWeekdays.includes(weekday)}
                    onChange={(event) => {
                      const next = event.currentTarget.checked
                        ? [...selectedWeekdays, weekday]
                        : selectedWeekdays.filter((value) => value !== weekday)
                      setValue("weekdays", next, {
                        shouldDirty: true,
                        shouldValidate: true,
                      })
                    }}
                  />
                  <span>{t(weekdayKeyMap[weekday])}</span>
                </label>
              ))}
            </div>
            {errors.weekdays && (
              <p className="text-sm text-destructive">
                {errors.weekdays.message}
              </p>
            )}
          </div>
          <div className="grid gap-2 sm:grid-cols-2">
            <div className="grid gap-2">
              <Label htmlFor="template-slot-start">
                {t("templates.slot.startTime")}
              </Label>
              <Input
                id="template-slot-start"
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
              <Label htmlFor="template-slot-end">
                {t("templates.slot.endTime")}
              </Label>
              <Input
                id="template-slot-end"
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
                  ? t("templates.slotDialog.submittingCreate")
                  : t("templates.slotDialog.submittingEdit")
                : mode === "create"
                  ? t("templates.slotDialog.submitCreate")
                  : t("templates.slotDialog.submitEdit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
