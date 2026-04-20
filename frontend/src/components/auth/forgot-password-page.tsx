import { useEffect, useEffectEvent, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
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
import { createForgotPasswordSchema, type ForgotPasswordFormValues } from "./auth-schemas"
import { requestPasswordReset as defaultRequestPasswordReset } from "@/lib/queries"

type ForgotPasswordPageProps = {
  requestPasswordReset?: (email: string) => Promise<void>
}

export function ForgotPasswordPage({
  requestPasswordReset = defaultRequestPasswordReset,
}: ForgotPasswordPageProps) {
  const { t, i18n } = useTranslation()
  const [isSubmitted, setIsSubmitted] = useState(false)
  const [isPending, setIsPending] = useState(false)

  const forgotPasswordSchema = createForgotPasswordSchema(t)

  const {
    register,
    handleSubmit,
    trigger,
    formState: { errors },
  } = useForm<ForgotPasswordFormValues>({
    resolver: zodResolver(forgotPasswordSchema),
  })

  const submitRequest = async ({ email }: ForgotPasswordFormValues) => {
    setIsPending(true)
    try {
      await requestPasswordReset(email)
    } catch {
      // Always show the generic success state to avoid account enumeration.
    } finally {
      setIsPending(false)
      setIsSubmitted(true)
    }
  }

  const revalidateVisibleErrors = useEffectEvent(() => {
    const errorFields = Object.keys(errors) as (keyof ForgotPasswordFormValues)[]
    if (errorFields.length > 0) {
      void trigger(errorFields)
    }
  })

  useEffect(() => {
    revalidateVisibleErrors()
  }, [i18n.language])

  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>{t("forgotPassword.title")}</CardTitle>
          <CardDescription>{t("forgotPassword.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          {isSubmitted ? (
            <div className="grid gap-4">
              <p className="text-sm text-muted-foreground">
                {t("forgotPassword.success")}
              </p>
              <a
                className="text-sm font-medium text-primary underline underline-offset-4"
                href="/login"
              >
                {t("forgotPassword.backToLogin")}
              </a>
            </div>
          ) : (
            <form onSubmit={handleSubmit(submitRequest)} className="grid gap-4">
              <div className="grid gap-2">
                <Label htmlFor="forgot-password-email">
                  {t("forgotPassword.email")}
                </Label>
                <Input
                  id="forgot-password-email"
                  type="email"
                  placeholder={t("forgotPassword.emailPlaceholder")}
                  {...register("email")}
                />
                {errors.email && (
                  <p className="text-sm text-destructive">
                    {errors.email.message}
                  </p>
                )}
              </div>
              <Button type="submit" disabled={isPending}>
                {isPending
                  ? t("forgotPassword.submitting")
                  : t("forgotPassword.submit")}
              </Button>
              <a
                className="text-sm font-medium text-primary underline underline-offset-4"
                href="/login"
              >
                {t("forgotPassword.backToLogin")}
              </a>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
