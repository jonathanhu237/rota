import { z } from "zod/v3"

export function createSwapSchema(
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  return z.object({
    counterpart_user_id: z
      .number({
        invalid_type_error: t("requests.validation.counterpartUserRequired"),
      })
      .int()
      .min(1, t("requests.validation.counterpartUserRequired")),
    counterpart_assignment_id: z
      .number({
        invalid_type_error: t("requests.validation.counterpartShiftRequired"),
      })
      .int()
      .min(1, t("requests.validation.counterpartShiftRequired")),
  })
}

export type SwapFormValues = z.infer<ReturnType<typeof createSwapSchema>>

export function createGiveDirectSchema(
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  return z.object({
    counterpart_user_id: z
      .number({
        invalid_type_error: t("requests.validation.counterpartUserRequired"),
      })
      .int()
      .min(1, t("requests.validation.counterpartUserRequired")),
  })
}

export type GiveDirectFormValues = z.infer<
  ReturnType<typeof createGiveDirectSchema>
>
