import { z } from "zod/v3"

export function createProfileSchema(
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, t("settings.validation.nameRequired"))
      .max(100, t("settings.validation.nameMax")),
  })
}

export function createPasswordSchema(
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  return z
    .object({
      current_password: z
        .string()
        .min(1, t("settings.validation.currentPasswordRequired")),
      new_password: z
        .string()
        .min(1, t("settings.validation.newPasswordRequired"))
        .min(8, t("settings.validation.passwordMin")),
      confirm_password: z
        .string()
        .min(1, t("settings.validation.confirmPasswordRequired")),
    })
    .refine((values) => values.new_password === values.confirm_password, {
      path: ["confirm_password"],
      message: t("settings.validation.passwordMismatch"),
    })
}

export function createEmailChangeSchema(
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  return z.object({
    new_email: z
      .string()
      .trim()
      .min(1, t("settings.validation.emailRequired"))
      .email(t("settings.validation.emailInvalid")),
    current_password: z
      .string()
      .min(1, t("settings.validation.currentPasswordRequired")),
  })
}

export function createPreferencesSchema() {
  return z.object({
    language_preference: z.enum(["zh", "en"]),
    theme_preference: z.enum(["system", "light", "dark"]),
  })
}

export function createBrandingSchema(
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  return z.object({
    product_name: z
      .string()
      .trim()
      .min(1, t("settings.validation.productNameRequired"))
      .refine(
        (value) => codePointLength(value) <= 60,
        t("settings.validation.productNameMax"),
      ),
    organization_name: z
      .string()
      .trim()
      .refine(
        (value) => codePointLength(value) <= 100,
        t("settings.validation.organizationNameMax"),
      ),
    version: z.number().int().positive(),
  })
}

function codePointLength(value: string) {
  return Array.from(value).length
}

export type ProfileFormValues = z.infer<ReturnType<typeof createProfileSchema>>
export type PasswordFormValues = z.infer<ReturnType<typeof createPasswordSchema>>
export type EmailChangeFormValues = z.infer<
  ReturnType<typeof createEmailChangeSchema>
>
export type PreferencesFormValues = z.infer<
  ReturnType<typeof createPreferencesSchema>
>
export type BrandingFormValues = z.infer<ReturnType<typeof createBrandingSchema>>
