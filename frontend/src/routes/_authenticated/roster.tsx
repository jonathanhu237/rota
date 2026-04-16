import { useQuery } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import { WeeklyRoster } from "@/components/roster/weekly-roster"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { currentUserQueryOptions, rosterCurrentQueryOptions } from "@/lib/queries"

export const Route = createFileRoute("/_authenticated/roster")({
  component: RosterPage,
})

function RosterPage() {
  const { t } = useTranslation()
  const { data: currentUser } = useQuery(currentUserQueryOptions)
  const rosterQuery = useQuery(rosterCurrentQueryOptions)

  if (rosterQuery.isLoading) {
    return (
      <div className="grid gap-4">
        <Skeleton className="h-28 w-full" />
        <Skeleton className="h-[520px] w-full" />
      </div>
    )
  }

  if (rosterQuery.isError) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("roster.title")}</CardTitle>
          <CardDescription>{t("roster.loadError")}</CardDescription>
        </CardHeader>
      </Card>
    )
  }

  const roster = rosterQuery.data
  if (!roster?.publication) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("roster.title")}</CardTitle>
          <CardDescription>{t("roster.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">
            {t("roster.empty")}
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <div className="grid gap-6">
      <Card>
        <CardHeader>
          <CardTitle>{t("roster.title")}</CardTitle>
          <CardDescription>
            {t("roster.descriptionWithPublication", {
              name: roster.publication.name,
            })}
          </CardDescription>
        </CardHeader>
      </Card>
      <WeeklyRoster
        weekdays={roster.weekdays}
        currentUserID={currentUser?.id}
      />
    </div>
  )
}
