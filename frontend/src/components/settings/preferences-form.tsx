import { useEffect, useMemo } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"

import { updateOwnProfileMutation } from "@/components/settings/settings-api"
import {
  createPreferencesSchema,
  type PreferencesFormValues,
} from "@/components/settings/settings-schemas"
import { useTheme } from "@/components/theme-context"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { useToast } from "@/components/ui/toast"
import { applyLanguagePreference, normalizeLanguage } from "@/i18n"
import type { User } from "@/lib/types"

const preferencesSchema = createPreferencesSchema()
const languageOptions = [
  { value: "zh", labelKey: "settings.preferences.languageZh" },
  { value: "en", labelKey: "settings.preferences.languageEn" },
] as const
const themeOptions = [
  { value: "system", labelKey: "settings.preferences.themeSystem" },
  { value: "light", labelKey: "settings.preferences.themeLight" },
  { value: "dark", labelKey: "settings.preferences.themeDark" },
] as const

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
    handleSubmit,
    control,
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
      <div className="grid max-w-sm gap-2">
        <Label id="language-preference-label">
          {t("settings.preferences.language")}
        </Label>
        <Controller
          control={control}
          name="language_preference"
          render={({ field }) => (
            <Select
              items={languageOptions.map((option) => ({
                label: t(option.labelKey),
                value: option.value,
              }))}
              value={field.value}
              onValueChange={(value) => field.onChange(value)}
            >
              <SelectTrigger
                aria-labelledby="language-preference-label"
                aria-invalid={Boolean(errors.language_preference)}
                className="w-full"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {languageOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {t(option.labelKey)}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          )}
        />
        {errors.language_preference && (
          <p className="text-sm text-destructive">
            {errors.language_preference.message}
          </p>
        )}
      </div>

      <div className="grid max-w-sm gap-2">
        <Label id="theme-preference-label">
          {t("settings.preferences.theme")}
        </Label>
        <Controller
          control={control}
          name="theme_preference"
          render={({ field }) => (
            <Select
              items={themeOptions.map((option) => ({
                label: t(option.labelKey),
                value: option.value,
              }))}
              value={field.value}
              onValueChange={(value) => field.onChange(value)}
            >
              <SelectTrigger
                aria-labelledby="theme-preference-label"
                aria-invalid={Boolean(errors.theme_preference)}
                className="w-full"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {themeOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {t(option.labelKey)}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          )}
        />
        {errors.theme_preference && (
          <p className="text-sm text-destructive">
            {errors.theme_preference.message}
          </p>
        )}
      </div>

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
