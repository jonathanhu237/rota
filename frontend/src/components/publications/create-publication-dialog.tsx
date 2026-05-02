import { useEffect, useEffectEvent } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"

import { DateTimePicker } from "@/components/date-time-picker"
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
import type { Template } from "@/lib/types"

import {
  createPublicationSchema,
  type PublicationFormValues,
} from "./publication-schemas"

type CreatePublicationDialogProps = {
  open: boolean
  templates: Template[]
  isPending: boolean
  isTemplatesLoading: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (values: PublicationFormValues) => void
}

export function CreatePublicationDialog({
  open,
  templates,
  isPending,
  isTemplatesLoading,
  onOpenChange,
  onSubmit,
}: CreatePublicationDialogProps) {
  const { t, i18n } = useTranslation()
  const formSchema = createPublicationSchema(t)

  const {
    register,
    handleSubmit,
    reset,
    trigger,
    control,
    formState: { errors },
  } = useForm<PublicationFormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      template_id: 0,
      name: "",
      submission_start_at: "",
      submission_end_at: "",
      planned_active_from: "",
      planned_active_until: "",
    },
  })

  useEffect(() => {
    if (open) {
      reset({
        template_id: 0,
        name: "",
        submission_start_at: "",
        submission_end_at: "",
        planned_active_from: "",
        planned_active_until: "",
      })
    }
  }, [open, reset])

  const revalidateVisibleErrors = useEffectEvent(() => {
    const errorFields = Object.keys(errors) as (keyof PublicationFormValues)[]
    if (errorFields.length > 0) {
      void trigger(errorFields)
    }
  })

  useEffect(() => {
    revalidateVisibleErrors()
  }, [i18n.language])

  const hasTemplates = templates.length > 0

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("publications.form.createTitle")}</DialogTitle>
          <DialogDescription>
            {t("publications.form.createDescription")}
          </DialogDescription>
        </DialogHeader>
        <form
          className="grid gap-4"
          onSubmit={handleSubmit((values) => onSubmit(values))}
        >
          <div className="grid gap-2">
            <Label htmlFor="publication-template">
              {t("publications.form.template")}
            </Label>
            <select
              id="publication-template"
              className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50"
              disabled={isTemplatesLoading || !hasTemplates}
              {...register("template_id", { valueAsNumber: true })}
            >
              <option value={0}>{t("publications.form.selectTemplate")}</option>
              {templates.map((template) => (
                <option key={template.id} value={template.id}>
                  {template.name}
                </option>
              ))}
            </select>
            {errors.template_id && (
              <p className="text-sm text-destructive">
                {errors.template_id.message}
              </p>
            )}
            {!isTemplatesLoading && !hasTemplates && (
              <p className="text-sm text-muted-foreground">
                {t("publications.form.noTemplates")}
              </p>
            )}
          </div>
          <div className="grid gap-2">
            <Label htmlFor="publication-name">{t("publications.name")}</Label>
            <Input id="publication-name" {...register("name")} />
            {errors.name && (
              <p className="text-sm text-destructive">{errors.name.message}</p>
            )}
          </div>
          <div className="grid gap-2">
            <Label htmlFor="publication-submission-start">
              {t("publications.submissionStartAt")}
            </Label>
            <Controller
              control={control}
              name="submission_start_at"
              render={({ field }) => (
                <DateTimePicker
                  id="publication-submission-start"
                  value={field.value}
                  onChange={field.onChange}
                  placeholder={t("common.selectDate")}
                  timeLabel={`${t("publications.submissionStartAt")} ${t("common.time")}`}
                  aria-invalid={Boolean(errors.submission_start_at)}
                />
              )}
            />
            {errors.submission_start_at && (
              <p className="text-sm text-destructive">
                {errors.submission_start_at.message}
              </p>
            )}
          </div>
          <div className="grid gap-2">
            <Label htmlFor="publication-submission-end">
              {t("publications.submissionEndAt")}
            </Label>
            <Controller
              control={control}
              name="submission_end_at"
              render={({ field }) => (
                <DateTimePicker
                  id="publication-submission-end"
                  value={field.value}
                  onChange={field.onChange}
                  placeholder={t("common.selectDate")}
                  timeLabel={`${t("publications.submissionEndAt")} ${t("common.time")}`}
                  aria-invalid={Boolean(errors.submission_end_at)}
                />
              )}
            />
            {errors.submission_end_at && (
              <p className="text-sm text-destructive">
                {errors.submission_end_at.message}
              </p>
            )}
          </div>
          <div className="grid gap-2">
            <Label htmlFor="publication-planned-active-from">
              {t("publications.plannedActiveFrom")}
            </Label>
            <Controller
              control={control}
              name="planned_active_from"
              render={({ field }) => (
                <DateTimePicker
                  id="publication-planned-active-from"
                  value={field.value}
                  onChange={field.onChange}
                  placeholder={t("common.selectDate")}
                  timeLabel={`${t("publications.plannedActiveFrom")} ${t("common.time")}`}
                  aria-invalid={Boolean(errors.planned_active_from)}
                />
              )}
            />
            {errors.planned_active_from && (
              <p className="text-sm text-destructive">
                {errors.planned_active_from.message}
              </p>
            )}
          </div>
          <div className="grid gap-2">
            <Label htmlFor="publication-planned-active-until">
              {t("publications.plannedActiveUntil")}
            </Label>
            <Controller
              control={control}
              name="planned_active_until"
              render={({ field }) => (
                <DateTimePicker
                  id="publication-planned-active-until"
                  value={field.value}
                  onChange={field.onChange}
                  placeholder={t("common.selectDate")}
                  timeLabel={`${t("publications.plannedActiveUntil")} ${t("common.time")}`}
                  aria-invalid={Boolean(errors.planned_active_until)}
                />
              )}
            />
            {errors.planned_active_until && (
              <p className="text-sm text-destructive">
                {errors.planned_active_until.message}
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
            <Button
              type="submit"
              disabled={isPending || isTemplatesLoading || !hasTemplates}
            >
              {isPending
                ? t("publications.form.submittingCreate")
                : t("publications.form.submitCreate")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
