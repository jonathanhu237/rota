import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
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

  if (isLoading) {
    return (
      <div className="grid gap-3">
        {Array.from({ length: 5 }).map((_, index) => (
          <Skeleton key={index} className="h-12 w-full" />
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

  return (
    <div className="grid gap-4">
      <div className="overflow-x-auto rounded-xl border">
        <table className="min-w-full text-sm">
          <thead className="bg-muted/40 text-left">
            <tr>
              <th className="px-4 py-3 font-medium">{t("users.table.name")}</th>
              <th className="px-4 py-3 font-medium">
                {t("users.table.email")}
              </th>
              <th className="px-4 py-3 font-medium">{t("users.table.role")}</th>
              <th className="px-4 py-3 font-medium">
                {t("users.table.status")}
              </th>
              <th className="px-4 py-3 font-medium">
                {t("users.table.actions")}
              </th>
            </tr>
          </thead>
          <tbody>
            {users.length === 0 && (
              <tr>
                <td
                  className="px-4 py-6 text-center text-muted-foreground"
                  colSpan={5}
                >
                  {t("users.empty")}
                </td>
              </tr>
            )}
            {users.map((user) => (
              <tr key={user.id} className="border-t align-top">
                <td className="px-4 py-3 font-medium">{user.name}</td>
                <td className="px-4 py-3 text-muted-foreground">
                  {user.email}
                </td>
                <td className="px-4 py-3">
                  <Badge variant={user.is_admin ? "default" : "secondary"}>
                    {user.is_admin ? t("common.admin") : t("common.employee")}
                  </Badge>
                </td>
                <td className="px-4 py-3">
                  <Badge
                    variant={
                      user.status === "active"
                        ? "default"
                        : user.status === "pending"
                          ? "secondary"
                          : "destructive"
                    }
                  >
                    {user.status === "active"
                      ? t("common.active")
                      : user.status === "pending"
                        ? t("users.status.pending")
                        : t("common.disabled")}
                  </Badge>
                </td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => onEdit(user)}
                    >
                      {t("users.actions.edit")}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => onResendInvitation(user)}
                      disabled={user.status !== "pending"}
                    >
                      {t("users.actions.resendInvitation")}
                    </Button>
                    <Button
                      size="sm"
                      variant={
                        user.status === "disabled"
                          ? "secondary"
                          : "destructive"
                      }
                      onClick={() => onToggleStatus(user)}
                    >
                      {user.status === "disabled"
                        ? t("users.actions.enable")
                        : t("users.actions.disable")}
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="text-sm text-muted-foreground">
          {t("users.pageSummary", {
            start,
            end,
            total,
          })}
          {isFetching && users.length > 0 && (
            <span className="ml-2">{t("common.refreshing")}</span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            onClick={() => onPageChange(page - 1)}
            disabled={page <= 1}
          >
            {t("users.pagination.previous")}
          </Button>
          <span className="text-sm text-muted-foreground">
            {t("users.pagination.page", {
              page,
              totalPages: Math.max(totalPages, 1),
            })}
          </span>
          <Button
            variant="outline"
            onClick={() => onPageChange(page + 1)}
            disabled={page >= totalPages || totalPages === 0}
          >
            {t("users.pagination.next")}
          </Button>
        </div>
      </div>
    </div>
  )
}
