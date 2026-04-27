import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { ChevronRight } from "lucide-react"
import { useTranslation } from "react-i18next"

import { PublicationStateBadge } from "@/components/publications/publication-state-badge"
import { buttonVariants } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { currentPublicationQueryOptions } from "@/lib/queries"
import type { Publication, User } from "@/lib/types"

type CurrentPublicationCardProps = {
  user: User | undefined
}

export function CurrentPublicationCard({ user }: CurrentPublicationCardProps) {
  const { t, i18n } = useTranslation()
  const publicationQuery = useQuery(currentPublicationQueryOptions)
  const isAdmin = user?.is_admin === true
  const formatter = new Intl.DateTimeFormat(i18n.resolvedLanguage, {
    dateStyle: "medium",
  })

  if (publicationQuery.isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("dashboard.currentPublication.title")}</CardTitle>
          <CardDescription>
            {t("dashboard.currentPublication.description")}
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3">
          <Skeleton className="h-6 w-48" />
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-8 w-40" />
        </CardContent>
      </Card>
    )
  }

  const publication = publicationQuery.data
  if (publication == null || publication.state === "ENDED") {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("dashboard.currentPublication.title")}</CardTitle>
          <CardDescription>
            {t("dashboard.currentPublication.description")}
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4">
          <p className="text-sm text-muted-foreground">
            {t("dashboard.currentPublication.empty")}
          </p>
          {isAdmin && (
            <div>
              <Link to="/publications" className={buttonVariants()}>
                {t("dashboard.currentPublication.cta.noneAdmin")}
                <ChevronRight data-icon="inline-end" />
              </Link>
            </div>
          )}
        </CardContent>
      </Card>
    )
  }

  const from = formatter.format(new Date(publication.planned_active_from))
  const until = formatter.format(new Date(publication.planned_active_until))
  const helperKey = getHelperKey(publication, isAdmin)

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("dashboard.currentPublication.title")}</CardTitle>
        <CardDescription>
          {t("dashboard.currentPublication.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4">
        <div className="flex flex-wrap items-center gap-3">
          <span className="text-base font-medium">{publication.name}</span>
          <PublicationStateBadge state={publication.state} />
        </div>
        <div className="text-sm text-muted-foreground">
          {t("dashboard.currentPublication.window", { from, until })}
        </div>
        {helperKey && (
          <p className="text-sm text-muted-foreground">{t(helperKey)}</p>
        )}
        <PublicationCTA publication={publication} isAdmin={isAdmin} />
      </CardContent>
    </Card>
  )
}

function getHelperKey(publication: Publication, isAdmin: boolean) {
  switch (publication.state) {
    case "DRAFT":
      return isAdmin ? null : "dashboard.currentPublication.copy.preparing"
    case "ASSIGNING":
      return isAdmin ? null : "dashboard.currentPublication.copy.awaiting"
    case "COLLECTING":
    case "PUBLISHED":
    case "ACTIVE":
    case "ENDED":
      return null
  }
}

function PublicationCTA({
  publication,
  isAdmin,
}: {
  publication: Publication
  isAdmin: boolean
}) {
  const { t } = useTranslation()
  const publicationID = String(publication.id)

  switch (publication.state) {
    case "DRAFT":
      if (!isAdmin) {
        return null
      }
      return (
        <div>
          <Link
            to="/publications/$publicationId"
            params={{ publicationId: publicationID }}
            className={buttonVariants()}
          >
            {t("dashboard.currentPublication.cta.draftAdmin")}
            <ChevronRight data-icon="inline-end" />
          </Link>
        </div>
      )
    case "COLLECTING":
      return (
        <div>
          {isAdmin ? (
            <Link
              to="/publications/$publicationId"
              params={{ publicationId: publicationID }}
              className={buttonVariants()}
            >
              {t("dashboard.currentPublication.cta.collectingAdmin")}
              <ChevronRight data-icon="inline-end" />
            </Link>
          ) : (
            <Link to="/availability" className={buttonVariants()}>
              {t("dashboard.currentPublication.cta.collectingEmployee")}
              <ChevronRight data-icon="inline-end" />
            </Link>
          )}
        </div>
      )
    case "ASSIGNING":
      if (!isAdmin) {
        return null
      }
      return (
        <div>
          <Link
            to="/publications/$publicationId/assignments"
            params={{ publicationId: publicationID }}
            className={buttonVariants()}
          >
            {t("dashboard.currentPublication.cta.assigningAdmin")}
            <ChevronRight data-icon="inline-end" />
          </Link>
        </div>
      )
    case "PUBLISHED":
    case "ACTIVE":
      return (
        <div>
          <Link to="/roster" className={buttonVariants()}>
            {t("dashboard.currentPublication.cta.published")}
            <ChevronRight data-icon="inline-end" />
          </Link>
        </div>
      )
    case "ENDED":
      return null
  }
}
