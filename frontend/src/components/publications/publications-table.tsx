import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import {
  getPublicationLifecycleAction,
  type PublicationLifecycleAction,
} from "@/lib/publications"
import type { Pagination, Publication } from "@/lib/types"

import { PublicationStateBadge } from "./publication-state-badge"

type PublicationsTableProps = {
  publications: Publication[]
  pagination?: Pagination
  isLoading: boolean
  isFetching: boolean
  onOpen: (publication: Publication) => void
  onLifecycleAction: (
    publication: Publication,
    action: PublicationLifecycleAction,
  ) => void
  onPageChange: (page: number) => void
}

export function PublicationsTable({
  publications,
  pagination,
  isLoading,
  isFetching,
  onOpen,
  onLifecycleAction,
  onPageChange,
}: PublicationsTableProps) {
  const { t, i18n } = useTranslation()

  if (isLoading) {
    return (
      <div className="grid gap-3">
        {Array.from({ length: 5 }).map((_, index) => (
          <Skeleton key={index} className="h-16 w-full" />
        ))}
      </div>
    )
  }

  const page = pagination?.page ?? 1
  const totalPages = pagination?.total_pages ?? 0
  const total = pagination?.total ?? 0
  const pageSize = pagination?.page_size ?? 0
  const start = total === 0 ? 0 : (page - 1) * pageSize + 1
  const end = total === 0 ? 0 : Math.min(page * pageSize, total)
  const formatter = new Intl.DateTimeFormat(i18n.resolvedLanguage, {
    dateStyle: "medium",
    timeStyle: "short",
  })

  return (
    <div className="grid gap-4">
      <div className="overflow-x-auto rounded-xl border">
        <table className="min-w-full text-sm">
          <thead className="bg-muted/40 text-left">
            <tr>
              <th className="px-4 py-3 font-medium">
                {t("publications.table.name")}
              </th>
              <th className="px-4 py-3 font-medium">
                {t("publications.table.template")}
              </th>
              <th className="px-4 py-3 font-medium">
                {t("publications.table.state")}
              </th>
              <th className="px-4 py-3 font-medium">
                {t("publications.table.submissionWindow")}
              </th>
              <th className="px-4 py-3 font-medium">
                {t("publications.table.activeWindow")}
              </th>
              <th className="px-4 py-3 font-medium">
                {t("publications.table.createdAt")}
              </th>
              <th className="px-4 py-3 font-medium">
                {t("publications.table.actions")}
              </th>
            </tr>
          </thead>
          <tbody>
            {publications.length === 0 && (
              <tr>
                <td
                  className="px-4 py-6 text-center text-muted-foreground"
                  colSpan={7}
                >
                  {t("publications.empty")}
                </td>
              </tr>
            )}
            {publications.map((publication) => (
              <tr
                key={publication.id}
                className="cursor-pointer border-t align-top transition-colors hover:bg-muted/30"
                onClick={() => onOpen(publication)}
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault()
                    onOpen(publication)
                  }
                }}
                tabIndex={0}
              >
                <td className="px-4 py-3 font-medium">{publication.name}</td>
                <td className="px-4 py-3 text-muted-foreground">
                  {publication.template_name}
                </td>
                <td className="px-4 py-3">
                  <PublicationStateBadge state={publication.state} />
                </td>
                <td className="px-4 py-3 text-muted-foreground">
                  <div>{formatter.format(new Date(publication.submission_start_at))}</div>
                  <div>{formatter.format(new Date(publication.submission_end_at))}</div>
                </td>
                <td className="px-4 py-3 text-muted-foreground">
                  <div>{formatter.format(new Date(publication.planned_active_from))}</div>
                  <div>{formatter.format(new Date(publication.planned_active_until))}</div>
                </td>
                <td className="px-4 py-3 text-muted-foreground">
                  {formatter.format(new Date(publication.created_at))}
                </td>
                <td className="px-4 py-3">
                  {(() => {
                    const action = getPublicationLifecycleAction(
                      publication.state,
                    )

                    if (!action) {
                      return <span className="text-muted-foreground">-</span>
                    }

                    return (
                      <Button
                        size="sm"
                        variant={
                          action === "end"
                            ? "destructive"
                            : action === "publish"
                              ? "default"
                              : "secondary"
                        }
                        onClick={(event) => {
                          event.stopPropagation()
                          onLifecycleAction(publication, action)
                        }}
                      >
                        {t(`publications.actions.${action}`)}
                      </Button>
                    )
                  })()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="text-sm text-muted-foreground">
          {t("publications.pageSummary", {
            start,
            end,
            total,
          })}
          {isFetching && publications.length > 0 && (
            <span className="ml-2">{t("common.refreshing")}</span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <button
            className="inline-flex h-9 items-center justify-center rounded-md border border-input px-4 text-sm font-medium transition-colors hover:bg-muted disabled:pointer-events-none disabled:opacity-50"
            disabled={page <= 1}
            onClick={() => onPageChange(page - 1)}
            type="button"
          >
            {t("publications.pagination.previous")}
          </button>
          <span className="text-sm text-muted-foreground">
            {t("publications.pagination.page", {
              page,
              totalPages: Math.max(totalPages, 1),
            })}
          </span>
          <button
            className="inline-flex h-9 items-center justify-center rounded-md border border-input px-4 text-sm font-medium transition-colors hover:bg-muted disabled:pointer-events-none disabled:opacity-50"
            disabled={page >= totalPages || totalPages === 0}
            onClick={() => onPageChange(page + 1)}
            type="button"
          >
            {t("publications.pagination.next")}
          </button>
        </div>
      </div>
    </div>
  )
}
