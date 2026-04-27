import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { ChevronRight } from "lucide-react"
import { useTranslation } from "react-i18next"

import { buttonVariants } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { unreadNotificationsQueryOptions } from "@/lib/queries"

export function TodoCard() {
  const { t } = useTranslation()
  const unreadQuery = useQuery(unreadNotificationsQueryOptions)

  if (unreadQuery.isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("dashboard.todo.title")}</CardTitle>
          <CardDescription>{t("dashboard.todo.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <Skeleton className="h-8 w-52" />
        </CardContent>
      </Card>
    )
  }

  const count = unreadQuery.data ?? 0
  if (count <= 0) {
    return null
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("dashboard.todo.title")}</CardTitle>
        <CardDescription>{t("dashboard.todo.description")}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm font-medium">
          {t("dashboard.todo.unreadRequests", { count })}
        </p>
        <Link to="/requests" className={buttonVariants({ variant: "outline" })}>
          {t("dashboard.todo.cta")}
          <ChevronRight data-icon="inline-end" />
        </Link>
      </CardContent>
    </Card>
  )
}
