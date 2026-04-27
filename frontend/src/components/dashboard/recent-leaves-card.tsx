import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { ChevronRight } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { buttonVariants } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import {
  currentPublicationQueryOptions,
  myLeavesQueryOptions,
} from "@/lib/queries"
import type { Leave, LeaveState } from "@/lib/types"

const stateVariant: Record<
  LeaveState,
  "default" | "secondary" | "outline" | "destructive"
> = {
  pending: "outline",
  completed: "default",
  failed: "destructive",
  cancelled: "secondary",
}

export function RecentLeavesCard() {
  const { t, i18n } = useTranslation()
  const leavesQuery = useQuery(myLeavesQueryOptions(1, 3))
  const currentPublicationQuery = useQuery(currentPublicationQueryOptions)
  const formatter = new Intl.DateTimeFormat(i18n.resolvedLanguage, {
    dateStyle: "medium",
  })

  if (leavesQuery.isLoading || currentPublicationQuery.isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("dashboard.recentLeaves.title")}</CardTitle>
          <CardDescription>
            {t("dashboard.recentLeaves.description")}
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </CardContent>
      </Card>
    )
  }

  const leaves = leavesQuery.data ?? []
  const hasActivePublication = currentPublicationQuery.data?.state === "ACTIVE"
  if (leaves.length === 0 && !hasActivePublication) {
    return null
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="grid gap-1">
            <CardTitle>{t("dashboard.recentLeaves.title")}</CardTitle>
            <CardDescription>
              {t("dashboard.recentLeaves.description")}
            </CardDescription>
          </div>
          <Link to="/leaves" className={buttonVariants({ variant: "outline" })}>
            {t("dashboard.recentLeaves.viewAll")}
            <ChevronRight data-icon="inline-end" />
          </Link>
        </div>
      </CardHeader>
      <CardContent>
        {leaves.length > 0 ? (
          <div className="grid gap-3">
            {leaves.map((leave) => (
              <RecentLeaveRow
                key={leave.id}
                leave={leave}
                formatter={formatter}
              />
            ))}
          </div>
        ) : (
          <div className="grid gap-3 rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
            <p>{t("dashboard.recentLeaves.empty")}</p>
            <div>
              <Link to="/leaves/new" className={buttonVariants()}>
                {t("leaves.requestCta")}
                <ChevronRight data-icon="inline-end" />
              </Link>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function RecentLeaveRow({
  leave,
  formatter,
}: {
  leave: Leave
  formatter: Intl.DateTimeFormat
}) {
  const { t } = useTranslation()

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border p-3">
      <div className="grid min-w-0 gap-1">
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant={stateVariant[leave.state]}>
            {t(`leave.state.${leave.state}`)}
          </Badge>
          <Badge variant="outline">{t(`leave.category.${leave.category}`)}</Badge>
        </div>
        <div className="truncate text-sm font-medium">
          {leave.request.occurrence_date} ·{" "}
          {t(`leave.type.${leave.request.type}`)}
        </div>
        <div className="text-xs text-muted-foreground">
          {formatter.format(new Date(leave.created_at))}
        </div>
      </div>
      <Link
        to="/leaves/$leaveId"
        params={{ leaveId: String(leave.id) }}
        className={buttonVariants({ variant: "outline" })}
      >
        {t("leaves.history.open")}
      </Link>
    </div>
  )
}
