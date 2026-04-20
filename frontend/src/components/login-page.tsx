import { useEffect, useEffectEvent } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation } from "@tanstack/react-query"
import { Link, useNavigate } from "@tanstack/react-router"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod/v3"

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
import api from "@/lib/axios"
import { getTranslatedApiError } from "@/lib/api-error"

type LoginForm = {
  email: string
  password: string
}

export function LoginPage() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()

  const toggleLanguage = () => {
    void i18n.changeLanguage(i18n.resolvedLanguage === "zh" ? "en" : "zh")
  }

  const loginSchema = z.object({
    email: z
      .string()
      .trim()
      .min(1, t("login.emailRequired"))
      .email(t("login.emailInvalid")),
    password: z.string().min(1, t("login.passwordRequired")),
  })

  const {
    register,
    handleSubmit,
    trigger,
    formState: { errors },
  } = useForm<LoginForm>({
    resolver: zodResolver(loginSchema),
  })

  const loginMutation = useMutation({
    mutationFn: (data: LoginForm) => api.post("/auth/login", data),
    onSuccess: () => {
      navigate({ to: "/" })
    },
  })

  const onSubmit = (data: LoginForm) => {
    loginMutation.mutate(data)
  }

  const revalidateVisibleErrors = useEffectEvent(() => {
    const errorFields = Object.keys(errors) as (keyof LoginForm)[]
    if (errorFields.length > 0) {
      void trigger(errorFields)
    }
  })

  useEffect(() => {
    revalidateVisibleErrors()
  }, [i18n.language])

  const errorMessage = loginMutation.error
    ? getTranslatedApiError(
        t,
        loginMutation.error,
        "login.errors",
        "login.unexpectedError",
      )
    : null

  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="fixed top-4 right-4">
        <Button variant="outline" size="sm" onClick={toggleLanguage}>
          {i18n.resolvedLanguage === "zh"
            ? t("common.languages.en")
            : t("common.languages.zh")}
        </Button>
      </div>
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>{t("login.title")}</CardTitle>
          <CardDescription>{t("login.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="grid gap-4">
            <div className="grid gap-2">
              <Label htmlFor="email">{t("login.email")}</Label>
              <Input
                id="email"
                type="email"
                placeholder={t("login.emailPlaceholder")}
                {...register("email")}
              />
              {errors.email && (
                <p className="text-sm text-destructive">
                  {errors.email.message}
                </p>
              )}
            </div>
            <div className="grid gap-2">
              <Label htmlFor="password">{t("login.password")}</Label>
              <Input
                id="password"
                type="password"
                {...register("password")}
              />
              {errors.password && (
                <p className="text-sm text-destructive">
                  {errors.password.message}
                </p>
              )}
            </div>
            {errorMessage && (
              <p className="text-sm text-destructive">{errorMessage}</p>
            )}
            <Button type="submit" disabled={loginMutation.isPending}>
              {loginMutation.isPending
                ? t("login.submitting")
                : t("login.submit")}
            </Button>
            <Link
              className="text-sm font-medium text-primary underline underline-offset-4"
              to="/forgot-password"
            >
              {t("login.forgotPassword")}
            </Link>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
