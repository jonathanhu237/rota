import { useQuery } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { useRotaTranslation as useTranslation } from "@/features/rota/i18n"

import { CurrentPublicationCard } from "@/features/rota/components/dashboard/current-publication-card"
import { ManageShortcutsCard } from "@/features/rota/components/dashboard/manage-shortcuts-card"
import { RecentLeavesCard } from "@/features/rota/components/dashboard/recent-leaves-card"
import { TodoCard } from "@/features/rota/components/dashboard/todo-card"
import { currentUserQueryOptions } from "@/features/rota/queries"

export const Route = createFileRoute("/_authenticated/rota/")({
  component: DashboardPage,
})

export function DashboardPage() {
  const { api } = Route.useRouteContext()
  const { t } = useTranslation()
  const { data: user } = useQuery(currentUserQueryOptions(api))

  return (
    <div className="grid gap-6">
      <div className="grid gap-1">
        <h1 className="text-2xl font-semibold tracking-normal">
          {(t as any)("dashboard.welcome", { name: user?.name })}
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
        {user?.can_manage && <ManageShortcutsCard />}
      </div>
    </div>
  )
}
