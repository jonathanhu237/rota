import { useQuery } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import { currentUserQueryOptions } from "@/lib/queries"

export const Route = createFileRoute("/_authenticated/")({
  component: DashboardPage,
})

function DashboardPage() {
  const { t } = useTranslation()
  const { data: user } = useQuery(currentUserQueryOptions)

  return (
    <div>
      <h1 className="text-2xl font-bold">
        {t("dashboard.welcome", { name: user?.name })}
      </h1>
      <p className="mt-2 text-muted-foreground">{t("dashboard.description")}</p>
    </div>
  )
}
