import { useQuery } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import { RequestsList } from "@/components/requests/requests-list"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { getTranslatedApiError } from "@/lib/api-error"
import {
  currentPublicationQueryOptions,
  currentUserQueryOptions,
  publicationMembersQueryOptions,
  publicationRosterQueryOptions,
  shiftChangeRequestsQueryOptions,
} from "@/lib/queries"

export const Route = createFileRoute("/_authenticated/requests")({
  component: RequestsPage,
})

function RequestsPage() {
  const { t } = useTranslation()

  const currentUserQuery = useQuery(currentUserQueryOptions)
  const currentPublicationQuery = useQuery(currentPublicationQueryOptions)
  const currentPublication = currentPublicationQuery.data
  const publicationID = currentPublication?.id ?? 0
  const isPublished = currentPublication?.state === "PUBLISHED"

  const requestsQuery = useQuery({
    ...shiftChangeRequestsQueryOptions(publicationID),
    enabled: isPublished,
  })
  const membersQuery = useQuery({
    ...publicationMembersQueryOptions(publicationID),
    enabled: isPublished,
  })
  const rosterQuery = useQuery({
    ...publicationRosterQueryOptions(publicationID),
    enabled: isPublished,
  })

  if (currentPublicationQuery.isLoading || currentUserQuery.isLoading) {
    return (
      <div className="grid gap-4">
        <Skeleton className="h-28 w-full" />
        <Skeleton className="h-72 w-full" />
      </div>
    )
  }

  if (currentPublicationQuery.isError) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("requests.title")}</CardTitle>
          <CardDescription>
            {getTranslatedApiError(
              t,
              currentPublicationQuery.error,
              "requests.errors",
              "requests.errors.INTERNAL_ERROR",
            )}
          </CardDescription>
        </CardHeader>
      </Card>
    )
  }

  if (!currentPublication || !isPublished) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("requests.title")}</CardTitle>
          <CardDescription>{t("requests.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">
            {t("requests.empty")}
          </div>
        </CardContent>
      </Card>
    )
  }

  const currentUser = currentUserQuery.data

  return (
    <div className="grid gap-6">
      <Card>
        <CardHeader>
          <CardTitle>{t("requests.title")}</CardTitle>
          <CardDescription>
            {t("requests.descriptionWithPublication", {
              name: currentPublication.name,
            })}
          </CardDescription>
        </CardHeader>
      </Card>

      {requestsQuery.isLoading || membersQuery.isLoading || rosterQuery.isLoading ? (
        <div className="grid gap-3">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-24 w-full" />
        </div>
      ) : requestsQuery.isError || membersQuery.isError || rosterQuery.isError ? (
        <Card>
          <CardContent className="pt-4">
            <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
              {getTranslatedApiError(
                t,
                requestsQuery.error ?? membersQuery.error ?? rosterQuery.error,
                "requests.errors",
                "requests.errors.INTERNAL_ERROR",
              )}
            </div>
          </CardContent>
        </Card>
      ) : currentUser ? (
        <RequestsList
          publicationID={currentPublication.id}
          requests={requestsQuery.data ?? []}
          members={membersQuery.data ?? []}
          currentUserID={currentUser.id}
          rosterWeekdays={rosterQuery.data?.weekdays ?? []}
        />
      ) : null}
    </div>
  )
}
