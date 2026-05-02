import * as React from "react"
import {
  flexRender,
  type Row,
  type RowData,
  type Table as TanStackTable,
} from "@tanstack/react-table"

import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { cn } from "@/lib/utils"

declare module "@tanstack/react-table" {
  // TanStack exposes these generic parameters for module augmentation.
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    className?: string
    headerClassName?: string
  }
}

type DataTablePagination = {
  page: number
  pageLabel: React.ReactNode
  previousLabel: React.ReactNode
  nextLabel: React.ReactNode
  summary: React.ReactNode
  totalPages: number
  onPageChange: (page: number) => void
}

type DataTableProps<TData> = {
  table: TanStackTable<TData>
  emptyMessage: React.ReactNode
  isLoading?: boolean
  loadingRows?: number
  pagination?: DataTablePagination
  getRowProps?: (row: Row<TData>) => React.HTMLAttributes<HTMLTableRowElement>
}

export function DataTable<TData>({
  table,
  emptyMessage,
  isLoading = false,
  loadingRows = 5,
  pagination,
  getRowProps,
}: DataTableProps<TData>) {
  const visibleColumnCount = Math.max(table.getVisibleLeafColumns().length, 1)

  return (
    <div className="flex flex-col gap-4">
      <div className="rounded-lg border" aria-busy={isLoading}>
        <Table>
          <TableHeader className="bg-muted/40">
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead
                    key={header.id}
                    className={cn("px-4 py-3", header.column.columnDef.meta?.headerClassName)}
                  >
                    {header.isPlaceholder
                      ? null
                      : flexRender(
                          header.column.columnDef.header,
                          header.getContext(),
                        )}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {isLoading ? (
              Array.from({ length: loadingRows }).map((_, index) => (
                <TableRow key={index}>
                  <TableCell className="px-4 py-3" colSpan={visibleColumnCount}>
                    <Skeleton className="h-6 w-full" />
                  </TableCell>
                </TableRow>
              ))
            ) : table.getRowModel().rows.length === 0 ? (
              <TableRow>
                <TableCell
                  className="px-4 py-6 text-center text-muted-foreground"
                  colSpan={visibleColumnCount}
                >
                  {emptyMessage}
                </TableCell>
              </TableRow>
            ) : (
              table.getRowModel().rows.map((row) => {
                const rowProps = getRowProps?.(row) ?? {}

                return (
                  <TableRow
                    key={row.id}
                    data-state={row.getIsSelected() ? "selected" : undefined}
                    {...rowProps}
                    className={cn("align-top", rowProps.className)}
                  >
                    {row.getVisibleCells().map((cell) => (
                      <TableCell
                        key={cell.id}
                        className={cn(
                          "px-4 py-3",
                          cell.column.columnDef.meta?.className,
                        )}
                      >
                        {flexRender(
                          cell.column.columnDef.cell,
                          cell.getContext(),
                        )}
                      </TableCell>
                    ))}
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </div>
      {pagination && (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="text-sm text-muted-foreground">
            {pagination.summary}
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              onClick={() => pagination.onPageChange(pagination.page - 1)}
              disabled={pagination.page <= 1}
              type="button"
            >
              {pagination.previousLabel}
            </Button>
            <span className="text-sm text-muted-foreground">
              {pagination.pageLabel}
            </span>
            <Button
              variant="outline"
              onClick={() => pagination.onPageChange(pagination.page + 1)}
              disabled={
                pagination.page >= pagination.totalPages ||
                pagination.totalPages === 0
              }
              type="button"
            >
              {pagination.nextLabel}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
