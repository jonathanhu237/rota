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
import type { Position, TemplateSlotPosition } from "@/lib/types"

import {
  createTemplateSlotPositionSchema,
  type TemplateSlotPositionFormValues,
} from "./template-schemas"

type TemplateSlotPositionDialogProps = {
  mode: "create" | "edit"
  open: boolean
  positions: Position[]
  positionEntry?: TemplateSlotPosition | null
  isPending: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (values: TemplateSlotPositionFormValues) => void
}

const selectClassName =
  "border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex h-10 w-full rounded-md border px-3 py-2 text-sm focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none"

export function TemplateSlotPositionDialog({
  mode,
  open,
  positions,
  positionEntry,
  isPending,
  onOpenChange,
  onSubmit,
}: TemplateSlotPositionDialogProps) {
  const { t, i18n } = useTranslation()
  const formSchema = createTemplateSlotPositionSchema(t)

  const {
    register,
    handleSubmit,
    reset,
    trigger,
    formState: { errors },
  } = useForm<TemplateSlotPositionFormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      position_id: positionEntry?.position_id ?? 0,
      required_headcount: positionEntry?.required_headcount ?? 1,
    },
  })

  useEffect(() => {
    reset({
      position_id: positionEntry?.position_id ?? 0,
      required_headcount: positionEntry?.required_headcount ?? 1,
    })
  }, [open, positionEntry, reset])

  const revalidateVisibleErrors = useEffectEvent(() => {
    const errorFields = Object.keys(errors) as (keyof TemplateSlotPositionFormValues)[]
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
              ? t("templates.positionDialog.createTitle")
              : t("templates.positionDialog.editTitle")}
          </DialogTitle>
          <DialogDescription>
            {mode === "create"
              ? t("templates.positionDialog.createDescription")
              : t("templates.positionDialog.editDescription")}
          </DialogDescription>
        </DialogHeader>
        <form
          className="grid gap-4"
          onSubmit={handleSubmit((values) => onSubmit(values))}
        >
          <div className="grid gap-2">
            <Label htmlFor="template-slot-position-id">
              {t("templates.position.position")}
            </Label>
            <select
              className={selectClassName}
              id="template-slot-position-id"
              {...register("position_id", { valueAsNumber: true })}
            >
              <option value={0}>{t("templates.positionDialog.selectPosition")}</option>
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
            <Label htmlFor="template-slot-position-headcount">
              {t("templates.position.requiredHeadcount")}
            </Label>
            <Input
              id="template-slot-position-headcount"
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
                  ? t("templates.positionDialog.submittingCreate")
                  : t("templates.positionDialog.submittingEdit")
                : mode === "create"
                  ? t("templates.positionDialog.submitCreate")
                  : t("templates.positionDialog.submitEdit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
