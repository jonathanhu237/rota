import { z } from "zod/v3"

export function createPublicationSchema(
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  return z
    .object({
      template_id: z
        .number({
          invalid_type_error: t("publications.validation.templateRequired"),
        })
        .int()
        .min(1, t("publications.validation.templateRequired")),
      name: z
        .string()
        .trim()
        .min(1, t("publications.validation.nameRequired"))
        .max(100, t("publications.validation.nameTooLong")),
      submission_start_at: z
        .string()
        .trim()
        .min(1, t("publications.validation.submissionStartRequired")),
      submission_end_at: z
        .string()
        .trim()
        .min(1, t("publications.validation.submissionEndRequired")),
      planned_active_from: z
        .string()
        .trim()
        .min(1, t("publications.validation.plannedActiveFromRequired")),
      planned_active_until: z
        .string()
        .trim()
        .min(1, t("publications.validation.plannedActiveUntilRequired")),
    })
    .superRefine((value, ctx) => {
      const submissionStart = Date.parse(value.submission_start_at)
      const submissionEnd = Date.parse(value.submission_end_at)
      const plannedActiveFrom = Date.parse(value.planned_active_from)
      const plannedActiveUntil = Date.parse(value.planned_active_until)

      if (
        Number.isNaN(submissionStart) ||
        Number.isNaN(submissionEnd) ||
        Number.isNaN(plannedActiveFrom) ||
        Number.isNaN(plannedActiveUntil)
      ) {
        return
      }

      if (
        !(
          submissionStart < submissionEnd &&
          submissionEnd <= plannedActiveFrom &&
          plannedActiveFrom < plannedActiveUntil
        )
      ) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["planned_active_until"],
          message: t("publications.validation.invalidWindow"),
        })
      }
    })
}

export type PublicationFormValues = z.infer<
  ReturnType<typeof createPublicationSchema>
>
