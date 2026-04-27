import { useEffect, useMemo } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"

import { updateOwnProfileMutation } from "@/components/settings/settings-api"
import {
  createPreferencesSchema,
  type PreferencesFormValues,
} from "@/components/settings/settings-schemas"
import { useTheme } from "@/components/theme-context"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { useToast } from "@/components/ui/toast"
import { applyLanguagePreference, normalizeLanguage } from "@/i18n"
import type { User } from "@/lib/types"

const preferencesSchema = createPreferencesSchema()

export function PreferencesForm({ user }: { user: User }) {
  const { t, i18n } = useTranslation()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const { setThemePreference } = useTheme()
  const defaultValues = useMemo(
    () =>
      ({
        language_preference:
          user.language_preference ?? normalizeLanguage(i18n.resolvedLanguage),
        theme_preference: user.theme_preference ?? "system",
      }) satisfies PreferencesFormValues,
    [i18n.resolvedLanguage, user.language_preference, user.theme_preference],
  )
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<PreferencesFormValues>({
    resolver: zodResolver(preferencesSchema),
    defaultValues,
  })
  const mutation = useMutation({
    mutationFn: (values: PreferencesFormValues) =>
      updateOwnProfileMutation.mutationFn(values),
    onSuccess: (updatedUser, values) => {
      queryClient.setQueryData(["auth", "me"], updatedUser)
      void applyLanguagePreference(values.language_preference)
      setThemePreference(values.theme_preference)
      toast({
        variant: "default",
        description: t("settings.preferences.saved"),
      })
    },
  })

  useEffect(() => {
    reset(defaultValues)
  }, [defaultValues, reset])

  return (
    <form
      className="grid gap-5"
      onSubmit={handleSubmit((values) => mutation.mutate(values))}
    >
      <fieldset className="grid gap-3">
        <legend className="text-sm font-medium">
          {t("settings.preferences.language")}
        </legend>
        <label className="flex items-center gap-2 text-sm">
          <input
            type="radio"
            value="zh"
            className="size-4"
            {...register("language_preference")}
          />
          <span>{t("settings.preferences.languageZh")}</span>
        </label>
        <label className="flex items-center gap-2 text-sm">
          <input
            type="radio"
            value="en"
            className="size-4"
            {...register("language_preference")}
          />
          <span>{t("settings.preferences.languageEn")}</span>
        </label>
        {errors.language_preference && (
          <p className="text-sm text-destructive">
            {errors.language_preference.message}
          </p>
        )}
      </fieldset>

      <fieldset className="grid gap-3">
        <legend className="text-sm font-medium">
          {t("settings.preferences.theme")}
        </legend>
        <Label className="flex items-center gap-2 text-sm font-normal">
          <input
            type="radio"
            value="system"
            className="size-4"
            {...register("theme_preference")}
          />
          <span>{t("settings.preferences.themeSystem")}</span>
        </Label>
        <Label className="flex items-center gap-2 text-sm font-normal">
          <input
            type="radio"
            value="light"
            className="size-4"
            {...register("theme_preference")}
          />
          <span>{t("settings.preferences.themeLight")}</span>
        </Label>
        <Label className="flex items-center gap-2 text-sm font-normal">
          <input
            type="radio"
            value="dark"
            className="size-4"
            {...register("theme_preference")}
          />
          <span>{t("settings.preferences.themeDark")}</span>
        </Label>
        {errors.theme_preference && (
          <p className="text-sm text-destructive">
            {errors.theme_preference.message}
          </p>
        )}
      </fieldset>

      <div>
        <Button type="submit" disabled={mutation.isPending}>
          {mutation.isPending
            ? t("settings.common.saving")
            : t("settings.common.save")}
        </Button>
      </div>
    </form>
  )
}
