import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, createFileRoute } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { getTranslatedApiError } from "@/lib/api-error"
import { myLeavesQueryOptions } from "@/lib/queries"
import type { Leave, LeaveState } from "@/lib/types"

export const Route = createFileRoute("/_authenticated/my-leaves")({
  component: MyLeavesPage,
})

const pageSize = 10

const stateVariant: Record<
  LeaveState,
  "default" | "secondary" | "outline" | "destructive"
> = {
  pending: "outline",
  completed: "default",
  failed: "destructive",
  cancelled: "secondary",
}

function MyLeavesPage() {
  const { t, i18n } = useTranslation()
  const [page, setPage] = useState(1)
  const leavesQuery = useQuery(myLeavesQueryOptions(page, pageSize))
  const formatter = new Intl.DateTimeFormat(i18n.resolvedLanguage, {
    dateStyle: "medium",
    timeStyle: "short",
  })

  return (
    <div className="grid gap-6">
      <Card>
        <CardHeader>
          <CardTitle>{t("myLeaves.title")}</CardTitle>
          <CardDescription>{t("myLeaves.description")}</CardDescription>
        </CardHeader>
      </Card>

      {leavesQuery.isLoading ? (
        <div className="grid gap-3">
          <Skeleton className="h-28 w-full" />
          <Skeleton className="h-28 w-full" />
        </div>
      ) : leavesQuery.isError ? (
        <Card>
          <CardContent className="pt-4">
            <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
              {getTranslatedApiError(
                t,
                leavesQuery.error,
                "leave.errors",
                "leave.errors.INTERNAL_ERROR",
              )}
            </div>
          </CardContent>
        </Card>
      ) : leavesQuery.data && leavesQuery.data.length > 0 ? (
        <>
          <div className="grid gap-3">
            {leavesQuery.data.map((leave) => (
              <LeaveHistoryCard
                key={leave.id}
                leave={leave}
                formatter={formatter}
              />
            ))}
          </div>
          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={page === 1}
              onClick={() => setPage((current) => Math.max(1, current - 1))}
            >
              {t("myLeaves.previous")}
            </Button>
            <div className="text-sm text-muted-foreground">
              {t("myLeaves.page", { page })}
            </div>
            <Button
              type="button"
              variant="outline"
              disabled={leavesQuery.data.length < pageSize}
              onClick={() => setPage((current) => current + 1)}
            >
              {t("myLeaves.next")}
            </Button>
          </div>
        </>
      ) : (
        <Card>
          <CardContent className="pt-4">
            <div className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">
              {t("myLeaves.empty")}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

function LeaveHistoryCard({
  leave,
  formatter,
}: {
  leave: Leave
  formatter: Intl.DateTimeFormat
}) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant={stateVariant[leave.state]}>
            {t(`leave.state.${leave.state}`)}
          </Badge>
          <Badge variant="outline">{t(`leave.category.${leave.category}`)}</Badge>
        </div>
        <CardTitle className="text-base">
          {leave.request.occurrence_date} ·{" "}
          {t(`leave.type.${leave.request.type}`)}
        </CardTitle>
        <CardDescription>
          {formatter.format(new Date(leave.created_at))}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-wrap items-center gap-2">
        <Button type="button" variant="outline" render={<Link to="/leaves/$leaveId" params={{ leaveId: String(leave.id) }} />}>
          {t("myLeaves.open")}
        </Button>
      </CardContent>
    </Card>
  )
}
