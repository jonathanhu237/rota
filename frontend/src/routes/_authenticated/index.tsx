import { useQuery } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import { CurrentPublicationCard } from "@/components/dashboard/current-publication-card"
import { ManageShortcutsCard } from "@/components/dashboard/manage-shortcuts-card"
import { RecentLeavesCard } from "@/components/dashboard/recent-leaves-card"
import { TodoCard } from "@/components/dashboard/todo-card"
import { currentUserQueryOptions } from "@/lib/queries"

export const Route = createFileRoute("/_authenticated/")({
  component: DashboardPage,
})

export function DashboardPage() {
  const { t } = useTranslation()
  const { data: user } = useQuery(currentUserQueryOptions)

  return (
    <div className="grid gap-6">
      <div className="grid gap-1">
        <h1 className="text-2xl font-semibold tracking-normal">
          {t("dashboard.welcome", { name: user?.name })}
        </h1>
        <p className="text-sm text-muted-foreground">
          {t("dashboard.description")}
        </p>
      </div>

      <div className="grid gap-6">
        <CurrentPublicationCard user={user} />
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <TodoCard />
          <RecentLeavesCard />
        </div>
        {user?.is_admin && <ManageShortcutsCard />}
      </div>
    </div>
  )
}
