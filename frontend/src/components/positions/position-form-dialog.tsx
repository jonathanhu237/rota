import { useEffect, useEffectEvent } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod/v3"

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
import { Textarea } from "@/components/ui/textarea"
import type { Position } from "@/lib/types"

export type PositionFormValues = {
  name: string
  description: string
}

type PositionFormDialogProps = {
  mode: "create" | "edit"
  open: boolean
  position?: Position | null
  isPending: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (values: PositionFormValues) => void
}

export function PositionFormDialog({
  mode,
  open,
  position,
  isPending,
  onOpenChange,
  onSubmit,
}: PositionFormDialogProps) {
  const { t, i18n } = useTranslation()

  const formSchema = z.object({
    name: z.string().trim().min(1, t("positions.validation.nameRequired")),
    description: z.string(),
  })

  const {
    register,
    handleSubmit,
    reset,
    trigger,
    formState: { errors },
  } = useForm<PositionFormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      name: position?.name ?? "",
      description: position?.description ?? "",
    },
  })

  useEffect(() => {
    reset({
      name: position?.name ?? "",
      description: position?.description ?? "",
    })
  }, [open, position, reset])

  const revalidateVisibleErrors = useEffectEvent(() => {
    const errorFields = Object.keys(errors) as (keyof PositionFormValues)[]
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
              ? t("positions.form.createTitle")
              : t("positions.form.editTitle")}
          </DialogTitle>
          <DialogDescription>
            {mode === "create"
              ? t("positions.form.createDescription")
              : t("positions.form.editDescription")}
          </DialogDescription>
        </DialogHeader>
        <form
          className="grid gap-4"
          onSubmit={handleSubmit((values) => onSubmit(values))}
        >
          <div className="grid gap-2">
            <Label htmlFor="position-name">{t("positions.name")}</Label>
            <Input id="position-name" {...register("name")} />
            {errors.name && (
              <p className="text-sm text-destructive">{errors.name.message}</p>
            )}
          </div>
          <div className="grid gap-2">
            <Label htmlFor="position-description">
              {t("positions.descriptionLabel")}
            </Label>
            <Textarea
              id="position-description"
              rows={5}
              {...register("description")}
            />
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
                  ? t("positions.form.submittingCreate")
                  : t("positions.form.submittingEdit")
                : mode === "create"
                  ? t("positions.form.submitCreate")
                  : t("positions.form.submitEdit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
