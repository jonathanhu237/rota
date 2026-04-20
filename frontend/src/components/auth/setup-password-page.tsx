import { useEffect, useEffectEvent } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQuery } from "@tanstack/react-query"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { getTranslatedApiError } from "@/lib/api-error"
import {
  previewSetupToken as previewSetupTokenQuery,
  setupPassword as setupPasswordQuery,
} from "@/lib/queries"
import type { SetupPasswordInput } from "@/lib/queries"
import type { SetupTokenPreview } from "@/lib/types"
import {
  createSetupPasswordSchema,
  type SetupPasswordFormValues,
} from "./auth-schemas"

type SetupPasswordPageProps = {
  token?: string
  previewSetupToken?: (token: string) => Promise<SetupTokenPreview>
  submitSetupPassword?: (input: SetupPasswordInput) => Promise<unknown>
  onSuccess?: () => void
}

export function SetupPasswordPage({
  token,
  previewSetupToken = previewSetupTokenQuery,
  submitSetupPassword = setupPasswordQuery,
  onSuccess,
}: SetupPasswordPageProps) {
  const { t, i18n } = useTranslation()
  const formSchema = createSetupPasswordSchema(t)
  const {
    register,
    handleSubmit,
    trigger,
    formState: { errors },
  } = useForm<SetupPasswordFormValues>({
    resolver: zodResolver(formSchema),
  })

  const previewQuery = useQuery({
    queryKey: ["auth", "setup-token", token],
    queryFn: async () => previewSetupToken(token ?? ""),
    enabled: Boolean(token),
    retry: false,
  })

  const setupMutation = useMutation({
    mutationFn: async (values: SetupPasswordFormValues) =>
      submitSetupPassword({
        token: token ?? "",
        password: values.password,
      }),
    onSuccess: () => {
      onSuccess?.()
    },
  })

  const revalidateVisibleErrors = useEffectEvent(() => {
    const errorFields = Object.keys(errors) as (keyof SetupPasswordFormValues)[]
    if (errorFields.length > 0) {
      void trigger(errorFields)
    }
  })

  useEffect(() => {
    revalidateVisibleErrors()
  }, [i18n.language])

  const previewErrorMessage = !token
    ? t("setupPassword.errors.INVALID_LINK")
    : previewQuery.error
      ? getTranslatedApiError(
          t,
          previewQuery.error,
          "setupPassword.errors",
          "setupPassword.unexpectedError",
        )
      : null

  const submitErrorMessage = setupMutation.error
    ? getTranslatedApiError(
        t,
        setupMutation.error,
        "setupPassword.errors",
        "setupPassword.unexpectedError",
      )
    : null

  const preview = previewQuery.data

  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>{t("setupPassword.title")}</CardTitle>
          <CardDescription>
            {preview?.purpose === "password_reset"
              ? t("setupPassword.resetDescription")
              : t("setupPassword.invitationDescription")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {!token || previewQuery.isError ? (
            <div className="grid gap-4">
              <p className="text-sm text-destructive">{previewErrorMessage}</p>
              <a
                className="text-sm font-medium text-primary underline underline-offset-4"
                href="/login"
              >
                {t("setupPassword.backToLogin")}
              </a>
            </div>
          ) : previewQuery.isLoading || !preview ? (
            <p className="text-sm text-muted-foreground">
              {t("setupPassword.loading")}
            </p>
          ) : (
            <form
              className="grid gap-4"
              onSubmit={handleSubmit((values) => setupMutation.mutate(values))}
            >
              <div className="grid gap-2">
                <Label htmlFor="setup-password-email">
                  {t("setupPassword.email")}
                </Label>
                <Input
                  id="setup-password-email"
                  value={preview.email}
                  readOnly
                  disabled
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="setup-password-password">
                  {t("setupPassword.password")}
                </Label>
                <Input
                  id="setup-password-password"
                  type="password"
                  {...register("password")}
                />
                {errors.password && (
                  <p className="text-sm text-destructive">
                    {errors.password.message}
                  </p>
                )}
              </div>
              <div className="grid gap-2">
                <Label htmlFor="setup-password-confirm">
                  {t("setupPassword.confirmPassword")}
                </Label>
                <Input
                  id="setup-password-confirm"
                  type="password"
                  {...register("confirmPassword")}
                />
                {errors.confirmPassword && (
                  <p className="text-sm text-destructive">
                    {errors.confirmPassword.message}
                  </p>
                )}
              </div>
              {submitErrorMessage && (
                <p className="text-sm text-destructive">{submitErrorMessage}</p>
              )}
              <Button type="submit" disabled={setupMutation.isPending}>
                {setupMutation.isPending
                  ? t("setupPassword.submitting")
                  : t("setupPassword.submit")}
              </Button>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
