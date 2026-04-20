import { z } from "zod/v3"

export function createForgotPasswordSchema(
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  return z.object({
    email: z
      .string()
      .trim()
      .min(1, t("forgotPassword.validation.emailRequired"))
      .email(t("forgotPassword.validation.emailInvalid")),
  })
}

export function createSetupPasswordSchema(
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  return z
    .object({
      password: z
        .string()
        .min(1, t("setupPassword.validation.passwordRequired"))
        .min(8, t("setupPassword.validation.passwordMin")),
      confirmPassword: z
        .string()
        .min(1, t("setupPassword.validation.confirmPasswordRequired")),
    })
    .refine((values) => values.password === values.confirmPassword, {
      path: ["confirmPassword"],
      message: t("setupPassword.validation.passwordMismatch"),
    })
}

export type ForgotPasswordFormValues = z.infer<
  ReturnType<typeof createForgotPasswordSchema>
>

export type SetupPasswordFormValues = z.infer<
  ReturnType<typeof createSetupPasswordSchema>
>
