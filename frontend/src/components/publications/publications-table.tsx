/* eslint-disable react-hooks/incompatible-library */

import * as React from "react"
import { useTranslation } from "react-i18next"
import {
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table"

import { DataTable } from "@/components/data-table"
import { Button } from "@/components/ui/button"
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
  const page = pagination?.page ?? 1
  const totalPages = pagination?.total_pages ?? 0
  const total = pagination?.total ?? 0
  const pageSize = pagination?.page_size ?? 0
  const start = total === 0 ? 0 : (page - 1) * pageSize + 1
  const end = total === 0 ? 0 : Math.min(page * pageSize, total)
  const formatter = React.useMemo(
    () =>
      new Intl.DateTimeFormat(i18n.resolvedLanguage, {
        dateStyle: "medium",
        timeStyle: "short",
      }),
    [i18n.resolvedLanguage],
  )
  const columns = React.useMemo<ColumnDef<Publication>[]>(
    () => [
      {
        accessorKey: "name",
        header: t("publications.table.name"),
        cell: ({ row }) => (
          <span className="font-medium">{row.original.name}</span>
        ),
      },
      {
        accessorKey: "template_name",
        header: t("publications.table.template"),
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {row.original.template_name}
          </span>
        ),
      },
      {
        accessorKey: "state",
        header: t("publications.table.state"),
        cell: ({ row }) => <PublicationStateBadge state={row.original.state} />,
      },
      {
        id: "submissionWindow",
        header: t("publications.table.submissionWindow"),
        cell: ({ row }) => (
          <div className="text-muted-foreground">
            <div>
              {formatter.format(new Date(row.original.submission_start_at))}
            </div>
            <div>
              {formatter.format(new Date(row.original.submission_end_at))}
            </div>
          </div>
        ),
      },
      {
        id: "activeWindow",
        header: t("publications.table.activeWindow"),
        cell: ({ row }) => (
          <div className="text-muted-foreground">
            <div>
              {formatter.format(new Date(row.original.planned_active_from))}
            </div>
            <div>
              {formatter.format(new Date(row.original.planned_active_until))}
            </div>
          </div>
        ),
      },
      {
        accessorKey: "created_at",
        header: t("publications.table.createdAt"),
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {formatter.format(new Date(row.original.created_at))}
          </span>
        ),
      },
      {
        id: "actions",
        header: t("publications.table.actions"),
        cell: ({ row }) => {
          const action = getPublicationLifecycleAction(row.original.state)

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
                onLifecycleAction(row.original, action)
              }}
            >
              {t(`publications.actions.${action}`)}
            </Button>
          )
        },
      },
    ],
    [formatter, onLifecycleAction, t],
  )
  const table = useReactTable({
    data: publications,
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    pageCount: totalPages,
    state: {
      pagination: {
        pageIndex: Math.max(page - 1, 0),
        pageSize: pageSize || Math.max(publications.length, 1),
      },
    },
  })

  return (
    <DataTable
      table={table}
      emptyMessage={t("publications.empty")}
      isLoading={isLoading}
      loadingRows={5}
      getRowProps={(row) => ({
        className: "cursor-pointer",
        onClick: () => onOpen(row.original),
        onKeyDown: (event) => {
          if (
            (event.key === "Enter" || event.key === " ") &&
            !isInteractiveEventTarget(event.target)
          ) {
            event.preventDefault()
            onOpen(row.original)
          }
        },
        tabIndex: 0,
      })}
      pagination={{
        page,
        totalPages,
        onPageChange,
        previousLabel: t("publications.pagination.previous"),
        nextLabel: t("publications.pagination.next"),
        pageLabel: t("publications.pagination.page", {
          page,
          totalPages: Math.max(totalPages, 1),
        }),
        summary: (
          <>
            {t("publications.pageSummary", {
              start,
              end,
              total,
            })}
            {isFetching && publications.length > 0 && (
              <span className="ml-2">{t("common.refreshing")}</span>
            )}
          </>
        ),
      }}
    />
  )
}

function isInteractiveEventTarget(target: EventTarget | null) {
  return (
    target instanceof HTMLElement &&
    Boolean(
      target.closest(
        'a, button, input, select, textarea, [role="button"], [role="link"], [contenteditable="true"]',
      ),
    )
  )
}
