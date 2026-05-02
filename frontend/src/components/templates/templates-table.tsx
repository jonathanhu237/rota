/* eslint-disable react-hooks/incompatible-library */

import * as React from "react"
import { useTranslation } from "react-i18next"
import {
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table"

import { DataTable } from "@/components/data-table"
import { Badge } from "@/components/ui/badge"
import type { Pagination, Template } from "@/lib/types"

type TemplatesTableProps = {
  templates: Template[]
  pagination?: Pagination
  isLoading: boolean
  isFetching: boolean
  onOpen: (template: Template) => void
  onPageChange: (page: number) => void
}

export function TemplatesTable({
  templates,
  pagination,
  isLoading,
  isFetching,
  onOpen,
  onPageChange,
}: TemplatesTableProps) {
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
  const columns = React.useMemo<ColumnDef<Template>[]>(
    () => [
      {
        accessorKey: "name",
        header: t("templates.table.name"),
        cell: ({ row }) => (
          <span className="font-medium">{row.original.name}</span>
        ),
      },
      {
        accessorKey: "shift_count",
        header: t("templates.table.shiftCount"),
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {row.original.shift_count}
          </span>
        ),
      },
      {
        accessorKey: "is_locked",
        header: t("templates.table.locked"),
        cell: ({ row }) => (
          <Badge variant={row.original.is_locked ? "destructive" : "secondary"}>
            {row.original.is_locked
              ? t("templates.locked")
              : t("templates.unlocked")}
          </Badge>
        ),
      },
      {
        accessorKey: "updated_at",
        header: t("templates.table.updatedAt"),
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {formatter.format(new Date(row.original.updated_at))}
          </span>
        ),
      },
    ],
    [formatter, t],
  )
  const table = useReactTable({
    data: templates,
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    pageCount: totalPages,
    state: {
      pagination: {
        pageIndex: Math.max(page - 1, 0),
        pageSize: pageSize || Math.max(templates.length, 1),
      },
    },
  })

  return (
    <DataTable
      table={table}
      emptyMessage={t("templates.empty")}
      isLoading={isLoading}
      loadingRows={5}
      getRowProps={(row) => ({
        className: "cursor-pointer",
        onClick: () => onOpen(row.original),
        onKeyDown: (event) => {
          if (event.key === "Enter" || event.key === " ") {
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
        previousLabel: t("templates.pagination.previous"),
        nextLabel: t("templates.pagination.next"),
        pageLabel: t("templates.pagination.page", {
          page,
          totalPages: Math.max(totalPages, 1),
        }),
        summary: (
          <>
            {t("templates.pageSummary", {
              start,
              end,
              total,
            })}
            {isFetching && templates.length > 0 && (
              <span className="ml-2">{t("common.refreshing")}</span>
            )}
          </>
        ),
      }}
    />
  )
}
