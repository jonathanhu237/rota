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
import type { Pagination, Position } from "@/lib/types"

type PositionsTableProps = {
  positions: Position[]
  pagination?: Pagination
  isLoading: boolean
  isFetching: boolean
  onDelete: (position: Position) => void
  onEdit: (position: Position) => void
  onPageChange: (page: number) => void
}

export function PositionsTable({
  positions,
  pagination,
  isLoading,
  isFetching,
  onDelete,
  onEdit,
  onPageChange,
}: PositionsTableProps) {
  const { t } = useTranslation()
  const page = pagination?.page ?? 1
  const totalPages = pagination?.total_pages ?? 0
  const total = pagination?.total ?? 0
  const pageSize = pagination?.page_size ?? 0
  const start = total === 0 ? 0 : (page - 1) * pageSize + 1
  const end = total === 0 ? 0 : Math.min(page * pageSize, total)
  const columns = React.useMemo<ColumnDef<Position>[]>(
    () => [
      {
        accessorKey: "name",
        header: t("positions.table.name"),
        cell: ({ row }) => (
          <span className="font-medium">{row.original.name}</span>
        ),
      },
      {
        accessorKey: "description",
        header: t("positions.table.description"),
        cell: ({ row }) => (
          <div className="max-w-2xl whitespace-pre-wrap break-words text-muted-foreground">
            {row.original.description || t("positions.noDescription")}
          </div>
        ),
      },
      {
        id: "actions",
        header: t("positions.table.actions"),
        cell: ({ row }) => (
          <div className="flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={() => onEdit(row.original)}
            >
              {t("positions.actions.edit")}
            </Button>
            <Button
              size="sm"
              variant="destructive"
              onClick={() => onDelete(row.original)}
            >
              {t("positions.actions.delete")}
            </Button>
          </div>
        ),
      },
    ],
    [onDelete, onEdit, t],
  )
  const table = useReactTable({
    data: positions,
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    pageCount: totalPages,
    state: {
      pagination: {
        pageIndex: Math.max(page - 1, 0),
        pageSize: pageSize || Math.max(positions.length, 1),
      },
    },
  })

  return (
    <DataTable
      table={table}
      emptyMessage={t("positions.empty")}
      isLoading={isLoading}
      pagination={{
        page,
        totalPages,
        onPageChange,
        previousLabel: t("positions.pagination.previous"),
        nextLabel: t("positions.pagination.next"),
        pageLabel: t("positions.pagination.page", {
          page,
          totalPages: Math.max(totalPages, 1),
        }),
        summary: (
          <>
            {t("positions.pageSummary", {
              start,
              end,
              total,
            })}
            {isFetching && positions.length > 0 && (
              <span className="ml-2">{t("common.refreshing")}</span>
            )}
          </>
        ),
      }}
    />
  )
}
