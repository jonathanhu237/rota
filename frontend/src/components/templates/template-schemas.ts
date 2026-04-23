import { z } from "zod/v3"

const timePattern = /^\d{2}:\d{2}$/

export function createTemplateSchema(
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, t("templates.validation.nameRequired"))
      .max(100, t("templates.validation.nameTooLong")),
    description: z
      .string()
      .trim()
      .max(500, t("templates.validation.descriptionTooLong")),
  })
}

export function createTemplateShiftSchema(
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  return z
    .object({
      weekday: z
        .number({
          invalid_type_error: t("templates.validation.weekdayRequired"),
        })
        .int()
        .min(1, t("templates.validation.invalidWeekday"))
        .max(7, t("templates.validation.invalidWeekday")),
      start_time: z
        .string()
        .trim()
        .regex(timePattern, t("templates.validation.invalidShiftTime")),
      end_time: z
        .string()
        .trim()
        .regex(timePattern, t("templates.validation.invalidShiftTime")),
      position_id: z
        .number({
          invalid_type_error: t("templates.validation.positionRequired"),
        })
        .int()
        .min(1, t("templates.validation.positionRequired")),
      required_headcount: z
        .number({
          invalid_type_error: t("templates.validation.invalidHeadcount"),
        })
        .int()
        .min(1, t("templates.validation.invalidHeadcount")),
    })
    .superRefine((value, ctx) => {
      if (value.end_time <= value.start_time) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["end_time"],
          message: t("templates.validation.invalidShiftTime"),
        })
      }
    })
}

export function createTemplateSlotSchema(
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  return z
    .object({
      weekday: z
        .number({
          invalid_type_error: t("templates.validation.weekdayRequired"),
        })
        .int()
        .min(1, t("templates.validation.invalidWeekday"))
        .max(7, t("templates.validation.invalidWeekday")),
      start_time: z
        .string()
        .trim()
        .regex(timePattern, t("templates.validation.invalidShiftTime")),
      end_time: z
        .string()
        .trim()
        .regex(timePattern, t("templates.validation.invalidShiftTime")),
    })
    .superRefine((value, ctx) => {
      if (value.end_time <= value.start_time) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["end_time"],
          message: t("templates.validation.invalidShiftTime"),
        })
      }
    })
}

export function createTemplateSlotPositionSchema(
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  return z.object({
    position_id: z
      .number({
        invalid_type_error: t("templates.validation.positionRequired"),
      })
      .int()
      .min(1, t("templates.validation.positionRequired")),
    required_headcount: z
      .number({
        invalid_type_error: t("templates.validation.invalidHeadcount"),
      })
      .int()
      .min(1, t("templates.validation.invalidHeadcount")),
  })
}

export type TemplateFormValues = z.infer<ReturnType<typeof createTemplateSchema>>
export type TemplateShiftFormValues = z.infer<
  ReturnType<typeof createTemplateShiftSchema>
>
export type TemplateSlotFormValues = z.infer<
  ReturnType<typeof createTemplateSlotSchema>
>
export type TemplateSlotPositionFormValues = z.infer<
  ReturnType<typeof createTemplateSlotPositionSchema>
>
