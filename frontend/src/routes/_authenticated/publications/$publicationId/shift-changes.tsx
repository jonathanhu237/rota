import { useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, createFileRoute, redirect } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import { PublicationStateBadge } from "@/components/publications/publication-state-badge"
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
import {
  currentUserQueryOptions,
  publicationMembersQueryOptions,
  publicationQueryOptions,
  shiftChangeRequestsQueryOptions,
} from "@/lib/queries"
import type {
  ShiftChangeRequest,
  ShiftChangeState,
  ShiftChangeType,
} from "@/lib/types"

export const Route = createFileRoute(
  "/_authenticated/publications/$publicationId/shift-changes",
)({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(currentUserQueryOptions)
    if (!user.is_admin) {
      throw redirect({ to: "/" })
    }
  },
  component: PublicationShiftChangesPage,
})

type StateFilter = "all" | "pending" | "decided"

const decidedStates: ShiftChangeState[] = [
  "approved",
  "rejected",
  "cancelled",
  "expired",
  "invalidated",
]

function PublicationShiftChangesPage() {
  const { publicationId } = Route.useParams()
  const numericPublicationID = Number(publicationId)

  const { t, i18n } = useTranslation()
  const [stateFilter, setStateFilter] = useState<StateFilter>("all")

  const publicationQuery = useQuery(
    publicationQueryOptions(numericPublicationID),
  )
  const requestsQuery = useQuery(
    shiftChangeRequestsQueryOptions(numericPublicationID),
  )
  const membersQuery = useQuery(
    publicationMembersQueryOptions(numericPublicationID),
  )

  const memberLookup = useMemo(() => {
    const map = new Map<number, string>()
    for (const member of membersQuery.data ?? []) {
      map.set(member.user_id, member.name)
    }
    return map
  }, [membersQuery.data])

  const formatter = useMemo(
    () =>
      new Intl.DateTimeFormat(i18n.resolvedLanguage, {
        dateStyle: "medium",
        timeStyle: "short",
      }),
    [i18n.resolvedLanguage],
  )

  const formatTimestamp = (value: string | null) =>
    value ? formatter.format(new Date(value)) : t("common.notAvailable")

  const resolveName = (userID: number | null) => {
    if (userID == null) {
      return t("common.notAvailable")
    }
    return memberLookup.get(userID) ?? `#${userID}`
  }

  const filteredRequests = useMemo(() => {
    const requests = requestsQuery.data ?? []

    if (stateFilter === "pending") {
      return requests.filter((request) => request.state === "pending")
    }

    if (stateFilter === "decided") {
      return requests.filter((request) => decidedStates.includes(request.state))
    }

    return requests
  }, [requestsQuery.data, stateFilter])

  if (publicationQuery.isLoading || requestsQuery.isLoading) {
    return (
      <div className="grid gap-4">
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  const publication = publicationQuery.data

  if (publicationQuery.isError || !publication) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("publications.shiftChanges.title")}</CardTitle>
          <CardDescription>
            {t("publications.shiftChanges.loadError")}
          </CardDescription>
        </CardHeader>
      </Card>
    )
  }

  return (
    <div className="grid gap-6">
      <Card>
        <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <CardTitle>{t("publications.shiftChanges.title")}</CardTitle>
            <CardDescription>
              {t("publications.shiftChanges.description", {
                name: publication.name,
              })}
            </CardDescription>
            <div className="pt-2">
              <PublicationStateBadge state={publication.state} />
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Link
              className="text-sm font-medium text-foreground underline underline-offset-4"
              params={{ publicationId: String(publication.id) }}
              to="/publications/$publicationId"
            >
              {t("publications.shiftChanges.backToPublication")}
            </Link>
          </div>
        </CardHeader>
        <CardContent className="grid gap-4">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm text-muted-foreground">
              {t("publications.shiftChanges.filter.label")}
            </span>
            {(["all", "pending", "decided"] as const).map((value) => (
              <Button
                key={value}
                size="sm"
                type="button"
                variant={stateFilter === value ? "default" : "outline"}
                onClick={() => setStateFilter(value)}
              >
                {t(`publications.shiftChanges.filter.${value}`)}
              </Button>
            ))}
          </div>
          {requestsQuery.isError ? (
            <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
              {t("publications.shiftChanges.loadError")}
            </div>
          ) : (
            <ShiftChangeRequestsTable
              requests={filteredRequests}
              resolveName={resolveName}
              formatTimestamp={formatTimestamp}
            />
          )}
        </CardContent>
      </Card>
    </div>
  )
}

type ShiftChangeRequestsTableProps = {
  requests: ShiftChangeRequest[]
  resolveName: (userID: number | null) => string
  formatTimestamp: (value: string | null) => string
}

export function ShiftChangeRequestsTable({
  requests,
  resolveName,
  formatTimestamp,
}: ShiftChangeRequestsTableProps) {
  const { t } = useTranslation()

  return (
    <div className="overflow-x-auto rounded-xl border">
      <table className="min-w-full text-sm">
        <thead className="bg-muted/40 text-left">
          <tr>
            <th className="px-4 py-3 font-medium">
              {t("publications.shiftChanges.table.id")}
            </th>
            <th className="px-4 py-3 font-medium">
              {t("publications.shiftChanges.table.type")}
            </th>
            <th className="px-4 py-3 font-medium">
              {t("publications.shiftChanges.table.requester")}
            </th>
            <th className="px-4 py-3 font-medium">
              {t("publications.shiftChanges.table.counterpart")}
            </th>
            <th className="px-4 py-3 font-medium">
              {t("publications.shiftChanges.table.occurrence")}
            </th>
            <th className="px-4 py-3 font-medium">
              {t("publications.shiftChanges.table.state")}
            </th>
            <th className="px-4 py-3 font-medium">
              {t("publications.shiftChanges.table.createdAt")}
            </th>
            <th className="px-4 py-3 font-medium">
              {t("publications.shiftChanges.table.decidedAt")}
            </th>
          </tr>
        </thead>
        <tbody>
          {requests.length === 0 && (
            <tr>
              <td
                className="px-4 py-6 text-center text-muted-foreground"
                colSpan={8}
              >
                {t("publications.shiftChanges.empty")}
              </td>
            </tr>
          )}
          {requests.map((request) => (
            <tr key={request.id} className="border-t align-top">
              <td className="px-4 py-3 font-medium">#{request.id}</td>
              <td className="px-4 py-3">
                <RequestTypeBadge type={request.type} />
              </td>
              <td className="px-4 py-3 text-muted-foreground">
                {resolveName(request.requester_user_id)}
              </td>
              <td className="px-4 py-3 text-muted-foreground">
                {resolveName(request.counterpart_user_id)}
              </td>
              <td className="px-4 py-3 text-muted-foreground">
                {request.occurrence_date}
                {request.counterpart_occurrence_date
                  ? ` / ${request.counterpart_occurrence_date}`
                  : ""}
              </td>
              <td className="px-4 py-3">
                <RequestStateBadge state={request.state} />
              </td>
              <td className="px-4 py-3 text-muted-foreground">
                {formatTimestamp(request.created_at)}
              </td>
              <td className="px-4 py-3 text-muted-foreground">
                {formatTimestamp(request.decided_at)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function RequestTypeBadge({ type }: { type: ShiftChangeType }) {
  const { t } = useTranslation()

  return (
    <Badge variant="outline">
      {t(`publications.shiftChanges.requestType.${type}`)}
    </Badge>
  )
}

const stateVariantByState: Record<
  ShiftChangeState,
  "default" | "secondary" | "outline" | "destructive"
> = {
  pending: "default",
  approved: "default",
  rejected: "destructive",
  cancelled: "secondary",
  expired: "secondary",
  invalidated: "destructive",
}

function RequestStateBadge({ state }: { state: ShiftChangeState }) {
  const { t } = useTranslation()

  return (
    <Badge variant={stateVariantByState[state]}>
      {t(`publications.shiftChanges.state.${state}`)}
    </Badge>
  )
}
