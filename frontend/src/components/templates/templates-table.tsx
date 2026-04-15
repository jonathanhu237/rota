import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
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

  if (isLoading) {
    return (
      <div className="grid gap-3">
        {Array.from({ length: 5 }).map((_, index) => (
          <Skeleton key={index} className="h-14 w-full" />
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
              <th className="px-4 py-3 font-medium">{t("templates.table.name")}</th>
              <th className="px-4 py-3 font-medium">
                {t("templates.table.shiftCount")}
              </th>
              <th className="px-4 py-3 font-medium">
                {t("templates.table.locked")}
              </th>
              <th className="px-4 py-3 font-medium">
                {t("templates.table.updatedAt")}
              </th>
            </tr>
          </thead>
          <tbody>
            {templates.length === 0 && (
              <tr>
                <td
                  className="px-4 py-6 text-center text-muted-foreground"
                  colSpan={4}
                >
                  {t("templates.empty")}
                </td>
              </tr>
            )}
            {templates.map((template) => (
              <tr
                key={template.id}
                className="cursor-pointer border-t align-top transition-colors hover:bg-muted/30"
                onClick={() => onOpen(template)}
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault()
                    onOpen(template)
                  }
                }}
                tabIndex={0}
              >
                <td className="px-4 py-3 font-medium">{template.name}</td>
                <td className="px-4 py-3 text-muted-foreground">
                  {template.shift_count}
                </td>
                <td className="px-4 py-3">
                  <Badge
                    variant={template.is_locked ? "destructive" : "secondary"}
                  >
                    {template.is_locked
                      ? t("templates.locked")
                      : t("templates.unlocked")}
                  </Badge>
                </td>
                <td className="px-4 py-3 text-muted-foreground">
                  {formatter.format(new Date(template.updated_at))}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="text-sm text-muted-foreground">
          {t("templates.pageSummary", {
            start,
            end,
            total,
          })}
          {isFetching && templates.length > 0 && (
            <span className="ml-2">{t("common.refreshing")}</span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            disabled={page <= 1}
            onClick={() => onPageChange(page - 1)}
            type="button"
          >
            {t("templates.pagination.previous")}
          </Button>
          <span className="text-sm text-muted-foreground">
            {t("templates.pagination.page", {
              page,
              totalPages: Math.max(totalPages, 1),
            })}
          </span>
          <Button
            variant="outline"
            disabled={page >= totalPages || totalPages === 0}
            onClick={() => onPageChange(page + 1)}
            type="button"
          >
            {t("templates.pagination.next")}
          </Button>
        </div>
      </div>
    </div>
  )
}
