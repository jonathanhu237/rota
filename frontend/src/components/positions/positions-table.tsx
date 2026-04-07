import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
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
              <th className="px-4 py-3 font-medium">
                {t("positions.table.name")}
              </th>
              <th className="px-4 py-3 font-medium">
                {t("positions.table.description")}
              </th>
              <th className="px-4 py-3 font-medium">
                {t("positions.table.actions")}
              </th>
            </tr>
          </thead>
          <tbody>
            {positions.length === 0 && (
              <tr>
                <td
                  className="px-4 py-6 text-center text-muted-foreground"
                  colSpan={3}
                >
                  {t("positions.empty")}
                </td>
              </tr>
            )}
            {positions.map((position) => (
              <tr key={position.id} className="border-t align-top">
                <td className="px-4 py-3 font-medium">{position.name}</td>
                <td className="px-4 py-3 text-muted-foreground">
                  <div className="max-w-2xl whitespace-pre-wrap break-words">
                    {position.description || t("positions.noDescription")}
                  </div>
                </td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => onEdit(position)}
                    >
                      {t("positions.actions.edit")}
                    </Button>
                    <Button
                      size="sm"
                      variant="destructive"
                      onClick={() => onDelete(position)}
                    >
                      {t("positions.actions.delete")}
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
          {t("positions.pageSummary", {
            start,
            end,
            total,
          })}
          {isFetching && positions.length > 0 && (
            <span className="ml-2">{t("common.refreshing")}</span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            onClick={() => onPageChange(page - 1)}
            disabled={page <= 1}
          >
            {t("positions.pagination.previous")}
          </Button>
          <span className="text-sm text-muted-foreground">
            {t("positions.pagination.page", {
              page,
              totalPages: Math.max(totalPages, 1),
            })}
          </span>
          <Button
            variant="outline"
            onClick={() => onPageChange(page + 1)}
            disabled={page >= totalPages || totalPages === 0}
          >
            {t("positions.pagination.next")}
          </Button>
        </div>
      </div>
    </div>
  )
}
