import type { ReactElement } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useRouterState } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import {
  leaveQueryOptions,
  publicationQueryOptions,
  templateQueryOptions,
} from "@/lib/queries"
import type { Leave, Publication, TemplateDetail } from "@/lib/types"

type Crumb = {
  label: string
  link?: ReactElement
}

type BreadcrumbBuildInput = {
  pathname: string
  t: (key: string, options?: Record<string, unknown>) => string
  publicationID: number | null
  publication: Publication | null | undefined
  templateID: number | null
  template: TemplateDetail | undefined
  leaveID: number | null
  leave: Leave | undefined
}

export function AppBreadcrumbs() {
  const { t } = useTranslation()
  const routerState = useRouterState()
  const pathname = normalizePath(routerState.location.pathname)
  const publicationID = readNumericSegment(pathname, "publications")
  const templateID = readNumericSegment(pathname, "templates")
  const leaveID = readNumericSegment(pathname, "leaves")

  const publicationQuery = useQuery({
    ...publicationQueryOptions(publicationID ?? 0),
    enabled: publicationID !== null,
  })
  const templateQuery = useQuery({
    ...templateQueryOptions(templateID ?? 0),
    enabled: templateID !== null,
  })
  const leaveQuery = useQuery({
    ...leaveQueryOptions(leaveID ?? 0),
    enabled: leaveID !== null,
  })

  const crumbs = buildBreadcrumbs({
    pathname,
    t,
    publicationID,
    publication: publicationQuery.data,
    templateID,
    template: templateQuery.data,
    leaveID,
    leave: leaveQuery.data,
  })

  return (
    <Breadcrumb>
      <BreadcrumbList>
        {crumbs.map((crumb, index) => {
          const isLast = index === crumbs.length - 1
          return (
            <FragmentedCrumb
              key={`${crumb.label}-${index}`}
              crumb={crumb}
              isLast={isLast}
              showSeparator={index > 0}
            />
          )
        })}
      </BreadcrumbList>
    </Breadcrumb>
  )
}

function FragmentedCrumb({
  crumb,
  isLast,
  showSeparator,
}: {
  crumb: Crumb
  isLast: boolean
  showSeparator: boolean
}) {
  return (
    <>
      {showSeparator && <BreadcrumbSeparator />}
      <BreadcrumbItem>
        {isLast || !crumb.link ? (
          <BreadcrumbPage>{crumb.label}</BreadcrumbPage>
        ) : (
          <BreadcrumbLink render={crumb.link}>{crumb.label}</BreadcrumbLink>
        )}
      </BreadcrumbItem>
    </>
  )
}

function buildBreadcrumbs(input: BreadcrumbBuildInput): Crumb[] {
  const { pathname, t } = input

  switch (true) {
    case pathname === "/":
      return [{ label: t("breadcrumbs.dashboard") }]
    case pathname === "/roster":
      return [{ label: t("breadcrumbs.roster") }]
    case pathname === "/availability":
      return [{ label: t("breadcrumbs.availability") }]
    case pathname === "/requests":
      return [{ label: t("breadcrumbs.requests") }]
    case pathname === "/settings":
      return [{ label: t("breadcrumbs.settings") }]
    case pathname === "/users":
      return [{ label: t("breadcrumbs.users") }]
    case pathname === "/positions":
      return [{ label: t("breadcrumbs.positions") }]
    case pathname === "/leaves":
      return [{ label: t("breadcrumbs.leaves") }]
    case pathname === "/leaves/new":
      return [
        { label: t("breadcrumbs.leaves"), link: <Link to="/leaves" /> },
        { label: t("breadcrumbs.requestLeave") },
      ]
    case pathname.startsWith("/leaves/"):
      return [
        { label: t("breadcrumbs.leaves"), link: <Link to="/leaves" /> },
        { label: leaveLabel(input.leave, input.leaveID, t) },
      ]
    case pathname === "/templates":
      return [{ label: t("breadcrumbs.templates") }]
    case pathname.startsWith("/templates/"):
      return [
        { label: t("breadcrumbs.templates"), link: <Link to="/templates" /> },
        { label: input.template?.name ?? t("breadcrumbs.templateFallback") },
      ]
    case pathname === "/publications":
      return [{ label: t("breadcrumbs.publications") }]
    case pathname.startsWith("/publications/"):
      return publicationBreadcrumbs(input)
    default:
      return [{ label: t("breadcrumbs.current") }]
  }
}

function publicationBreadcrumbs(input: BreadcrumbBuildInput): Crumb[] {
  const { pathname, publicationID, publication, t } = input
  const publicationLabel = publication?.name ?? t("breadcrumbs.publicationFallback")
  const base: Crumb[] = [
    {
      label: t("breadcrumbs.publications"),
      link: <Link to="/publications" />,
    },
  ]

  if (publicationID === null) {
    return [...base, { label: publicationLabel }]
  }

  const detailCrumb: Crumb = {
    label: publicationLabel,
    link: (
      <Link
        to="/publications/$publicationId"
        params={{ publicationId: String(publicationID) }}
      />
    ),
  }

  if (pathname.endsWith("/assignments")) {
    return [
      ...base,
      detailCrumb,
      { label: t("breadcrumbs.assignments") },
    ]
  }

  if (pathname.endsWith("/shift-changes")) {
    return [
      ...base,
      detailCrumb,
      { label: t("breadcrumbs.shiftChanges") },
    ]
  }

  return [...base, { label: publicationLabel }]
}

function leaveLabel(
  leave: Leave | undefined,
  leaveID: number | null,
  t: (key: string, options?: Record<string, unknown>) => string,
) {
  if (!leave) {
    return t("breadcrumbs.leaveFallback", { id: leaveID ?? "" })
  }

  return `${leave.request.occurrence_date} · ${t(
    `leave.category.${leave.category}`,
  )}`
}

function normalizePath(pathname: string) {
  if (pathname === "/") {
    return pathname
  }
  return pathname.replace(/\/+$/, "")
}

function readNumericSegment(pathname: string, segment: string) {
  const parts = normalizePath(pathname).split("/").filter(Boolean)
  const index = parts.indexOf(segment)
  if (index === -1 || index === parts.length - 1) {
    return null
  }

  const value = Number(parts[index + 1])
  return Number.isFinite(value) ? value : null
}
