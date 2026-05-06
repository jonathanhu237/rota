import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { createFileRoute, useNavigate } from "@tanstack/react-router"
import { Search } from "lucide-react"
import { useTranslation } from "react-i18next"

import { AdminAvailabilityTable } from "@/components/availability/admin-availability-table"
import { PublicationStateBadge } from "@/components/publications/publication-state-badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { adminAvailabilityBoardQueryOptions } from "@/lib/queries"

const pageSize = 10

export const Route = createFileRoute(
  "/_authenticated/publications/$publicationId/availability/",
)({
  component: PublicationAvailabilityPage,
})

export function PublicationAvailabilityPage() {
  const { publicationId } = Route.useParams()
  const numericPublicationID = Number(publicationId)

  const { t } = useTranslation()
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState("")

  const boardQuery = useQuery(
    adminAvailabilityBoardQueryOptions(
      numericPublicationID,
      page,
      pageSize,
      search,
    ),
  )
  const board = boardQuery.data

  if (boardQuery.isLoading) {
    return (
      <div className="grid gap-4">
        <Skeleton className="h-36 w-full" />
        <Skeleton className="h-80 w-full" />
      </div>
    )
  }

  if (boardQuery.isError || !board) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("adminAvailability.title")}</CardTitle>
          <CardDescription>{t("adminAvailability.loadError")}</CardDescription>
        </CardHeader>
      </Card>
    )
  }

  return (
    <div className="grid gap-6">
      <Card>
        <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <CardTitle>{t("adminAvailability.title")}</CardTitle>
            <CardDescription>
              {t("adminAvailability.description", {
                name: board.publication.name,
              })}
            </CardDescription>
            <div className="pt-2">
              <PublicationStateBadge state={board.publication.state} />
            </div>
          </div>
          <Button
            type="button"
            variant="outline"
            onClick={() =>
              navigate({
                to: "/publications/$publicationId/assignments",
                params: { publicationId },
              })
            }
          >
            {t("adminAvailability.backToAssignments")}
          </Button>
        </CardHeader>
        <CardContent className="grid gap-4">
          <label className="grid gap-2 sm:max-w-sm">
            <span className="text-sm font-medium">
              {t("adminAvailability.search.label")}
            </span>
            <div className="relative">
              <Search
                aria-hidden="true"
                className="pointer-events-none absolute left-2 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
              />
              <Input
                className="pl-8"
                placeholder={t("adminAvailability.search.placeholder")}
                value={search}
                onChange={(event) => {
                  setSearch(event.currentTarget.value)
                  setPage(1)
                }}
              />
            </div>
          </label>
          <AdminAvailabilityTable
            employees={board.employees}
            pagination={board.pagination}
            isLoading={boardQuery.isLoading}
            isFetching={boardQuery.isFetching}
            onPageChange={setPage}
            onEdit={(employee) =>
              navigate({
                to: "/publications/$publicationId/availability/$userId",
                params: {
                  publicationId,
                  userId: String(employee.user_id),
                },
              })
            }
          />
        </CardContent>
      </Card>
    </div>
  )
}
