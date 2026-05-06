/* eslint-disable react-hooks/incompatible-library */

import * as React from "react"
import {
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table"
import { useTranslation } from "react-i18next"

import { DataTable } from "@/components/data-table"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import type {
  AdminAvailabilityEmployee,
  Pagination,
} from "@/lib/types"

type AdminAvailabilityTableProps = {
  employees: AdminAvailabilityEmployee[]
  pagination?: Pagination
  isLoading: boolean
  isFetching: boolean
  onEdit: (employee: AdminAvailabilityEmployee) => void
  onPageChange: (page: number) => void
}

export function AdminAvailabilityTable({
  employees,
  pagination,
  isLoading,
  isFetching,
  onEdit,
  onPageChange,
}: AdminAvailabilityTableProps) {
  const { t } = useTranslation()
  const page = pagination?.page ?? 1
  const totalPages = pagination?.total_pages ?? 0
  const total = pagination?.total ?? 0
  const pageSize = pagination?.page_size ?? 0
  const start = total === 0 ? 0 : (page - 1) * pageSize + 1
  const end = total === 0 ? 0 : Math.min(page * pageSize, total)

  const columns = React.useMemo<ColumnDef<AdminAvailabilityEmployee>[]>(
    () => [
      {
        accessorKey: "name",
        header: t("adminAvailability.table.name"),
        cell: ({ row }) => (
          <div className="grid gap-1">
            <span className="font-medium">{row.original.name}</span>
            <span className="text-xs text-muted-foreground">
              {row.original.email}
            </span>
          </div>
        ),
      },
      {
        id: "positions",
        header: t("adminAvailability.table.positions"),
        cell: ({ row }) => (
          <div className="flex flex-wrap gap-1">
            {row.original.positions.map((position) => (
              <Badge key={position.id} variant="outline">
                {position.name}
              </Badge>
            ))}
          </div>
        ),
      },
      {
        accessorKey: "submitted_count",
        header: t("adminAvailability.table.submittedCount"),
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {t("adminAvailability.table.submittedCountValue", {
              count: row.original.submitted_count,
            })}
          </span>
        ),
      },
      {
        id: "actions",
        header: t("adminAvailability.table.actions"),
        meta: {
          className: "text-right",
          headerClassName: "text-right",
        },
        cell: ({ row }) => (
          <Button
            size="sm"
            type="button"
            variant="outline"
            onClick={() => onEdit(row.original)}
          >
            {t("adminAvailability.table.edit")}
          </Button>
        ),
      },
    ],
    [onEdit, t],
  )

  const table = useReactTable({
    data: employees,
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    pageCount: totalPages,
    state: {
      pagination: {
        pageIndex: Math.max(page - 1, 0),
        pageSize: pageSize || Math.max(employees.length, 1),
      },
    },
  })

  return (
    <DataTable
      table={table}
      emptyMessage={t("adminAvailability.table.empty")}
      isLoading={isLoading}
      loadingRows={5}
      pagination={{
        page,
        totalPages,
        onPageChange,
        previousLabel: t("adminAvailability.pagination.previous"),
        nextLabel: t("adminAvailability.pagination.next"),
        pageLabel: t("adminAvailability.pagination.page", {
          page,
          totalPages: Math.max(totalPages, 1),
        }),
        summary: (
          <>
            {t("adminAvailability.pageSummary", {
              start,
              end,
              total,
            })}
            {isFetching && employees.length > 0 && (
              <span className="ml-2">{t("common.refreshing")}</span>
            )}
          </>
        ),
      }}
    />
  )
}
