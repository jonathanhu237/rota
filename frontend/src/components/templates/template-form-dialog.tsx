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
import { Textarea } from "@/components/ui/textarea"

import {
  createTemplateSchema,
  type TemplateFormValues,
} from "./template-schemas"

type TemplateFormDialogProps = {
  open: boolean
  isPending: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (values: TemplateFormValues) => void
}

export function TemplateFormDialog({
  open,
  isPending,
  onOpenChange,
  onSubmit,
}: TemplateFormDialogProps) {
  const { t, i18n } = useTranslation()
  const formSchema = createTemplateSchema(t)

  const {
    register,
    handleSubmit,
    reset,
    trigger,
    formState: { errors },
  } = useForm<TemplateFormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      name: "",
      description: "",
    },
  })

  useEffect(() => {
    if (open) {
      reset({
        name: "",
        description: "",
      })
    }
  }, [open, reset])

  const revalidateVisibleErrors = useEffectEvent(() => {
    const errorFields = Object.keys(errors) as (keyof TemplateFormValues)[]
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
          <DialogTitle>{t("templates.form.createTitle")}</DialogTitle>
          <DialogDescription>
            {t("templates.form.createDescription")}
          </DialogDescription>
        </DialogHeader>
        <form
          className="grid gap-4"
          onSubmit={handleSubmit((values) => onSubmit(values))}
        >
          <div className="grid gap-2">
            <Label htmlFor="template-name">{t("templates.name")}</Label>
            <Input id="template-name" {...register("name")} />
            {errors.name && (
              <p className="text-sm text-destructive">{errors.name.message}</p>
            )}
          </div>
          <div className="grid gap-2">
            <Label htmlFor="template-description">
              {t("templates.descriptionLabel")}
            </Label>
            <Textarea
              id="template-description"
              rows={5}
              {...register("description")}
            />
            {errors.description && (
              <p className="text-sm text-destructive">
                {errors.description.message}
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
                ? t("templates.form.submittingCreate")
                : t("templates.form.submitCreate")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
