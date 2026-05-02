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
import { Button } from "@/components/ui/button"
import type { Pagination, User } from "@/lib/types"

type UsersTableProps = {
  users: User[]
  pagination?: Pagination
  isLoading: boolean
  isFetching: boolean
  onEdit: (user: User) => void
  onPageChange: (page: number) => void
  onResendInvitation: (user: User) => void
  onToggleStatus: (user: User) => void
}

export function UsersTable({
  users,
  pagination,
  isLoading,
  isFetching,
  onEdit,
  onPageChange,
  onResendInvitation,
  onToggleStatus,
}: UsersTableProps) {
  const { t } = useTranslation()
  const page = pagination?.page ?? 1
  const totalPages = pagination?.total_pages ?? 0
  const total = pagination?.total ?? 0
  const pageSize = pagination?.page_size ?? 0
  const start = total === 0 ? 0 : (page - 1) * pageSize + 1
  const end = total === 0 ? 0 : Math.min(page * pageSize, total)
  const columns = React.useMemo<ColumnDef<User>[]>(
    () => [
      {
        accessorKey: "name",
        header: t("users.table.name"),
        cell: ({ row }) => (
          <span className="font-medium">{row.original.name}</span>
        ),
      },
      {
        accessorKey: "email",
        header: t("users.table.email"),
        cell: ({ row }) => (
          <span className="text-muted-foreground">{row.original.email}</span>
        ),
      },
      {
        id: "role",
        header: t("users.table.role"),
        cell: ({ row }) => (
          <Badge variant={row.original.is_admin ? "default" : "secondary"}>
            {row.original.is_admin ? t("common.admin") : t("common.employee")}
          </Badge>
        ),
      },
      {
        accessorKey: "status",
        header: t("users.table.status"),
        cell: ({ row }) => (
          <Badge
            variant={
              row.original.status === "active"
                ? "default"
                : row.original.status === "pending"
                  ? "secondary"
                  : "destructive"
            }
          >
            {row.original.status === "active"
              ? t("common.active")
              : row.original.status === "pending"
                ? t("users.status.pending")
                : t("common.disabled")}
          </Badge>
        ),
      },
      {
        id: "actions",
        header: t("users.table.actions"),
        cell: ({ row }) => (
          <div className="flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={() => onEdit(row.original)}
            >
              {t("users.actions.edit")}
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => onResendInvitation(row.original)}
              disabled={row.original.status !== "pending"}
            >
              {t("users.actions.resendInvitation")}
            </Button>
            <Button
              size="sm"
              variant={
                row.original.status === "disabled" ? "secondary" : "destructive"
              }
              onClick={() => onToggleStatus(row.original)}
            >
              {row.original.status === "disabled"
                ? t("users.actions.enable")
                : t("users.actions.disable")}
            </Button>
          </div>
        ),
      },
    ],
    [onEdit, onResendInvitation, onToggleStatus, t],
  )
  const table = useReactTable({
    data: users,
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    pageCount: totalPages,
    state: {
      pagination: {
        pageIndex: Math.max(page - 1, 0),
        pageSize: pageSize || Math.max(users.length, 1),
      },
    },
  })

  return (
    <DataTable
      table={table}
      emptyMessage={t("users.empty")}
      isLoading={isLoading}
      pagination={{
        page,
        totalPages,
        onPageChange,
        previousLabel: t("users.pagination.previous"),
        nextLabel: t("users.pagination.next"),
        pageLabel: t("users.pagination.page", {
          page,
          totalPages: Math.max(totalPages, 1),
        }),
        summary: (
          <>
            {t("users.pageSummary", {
              start,
              end,
              total,
            })}
            {isFetching && users.length > 0 && (
              <span className="ml-2">{t("common.refreshing")}</span>
            )}
          </>
        ),
      }}
    />
  )
}
