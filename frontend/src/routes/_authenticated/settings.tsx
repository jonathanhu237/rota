import { useQuery } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import { EmailForm } from "@/components/settings/email-form"
import { PasswordForm } from "@/components/settings/password-form"
import { PreferencesForm } from "@/components/settings/preferences-form"
import { ProfileForm } from "@/components/settings/profile-form"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { currentUserQueryOptions } from "@/lib/queries"

export const Route = createFileRoute("/_authenticated/settings")({
  component: SettingsPage,
})

function SettingsPage() {
  const { t } = useTranslation()
  const { data: user, isLoading } = useQuery(currentUserQueryOptions)

  if (isLoading || !user) {
    return (
      <div className="grid gap-4">
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-48 w-full" />
      </div>
    )
  }

  return (
    <div className="mx-auto grid w-full max-w-3xl gap-6">
      <div className="grid gap-1">
        <h1 className="text-2xl font-semibold tracking-normal">
          {t("settings.title")}
        </h1>
        <p className="text-sm text-muted-foreground">
          {t("settings.description")}
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("settings.profile.title")}</CardTitle>
          <CardDescription>
            {t("settings.profile.description")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <ProfileForm user={user} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("settings.email.title")}</CardTitle>
          <CardDescription>{t("settings.email.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <EmailForm user={user} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("settings.password.title")}</CardTitle>
          <CardDescription>
            {t("settings.password.description")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <PasswordForm />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("settings.preferences.title")}</CardTitle>
          <CardDescription>
            {t("settings.preferences.description")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <PreferencesForm user={user} />
        </CardContent>
      </Card>
    </div>
  )
}
