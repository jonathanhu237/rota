/* eslint-disable react-hooks/incompatible-library */

import * as React from "react"
import userEvent from "@testing-library/user-event"
import {
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table"
import { describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { DataTable } from "./data-table"

type Person = {
  id: number
  name: string
  email: string
}

const people: Person[] = [
  { id: 1, name: "Ada", email: "ada@example.com" },
  { id: 2, name: "Grace", email: "grace@example.com" },
]

function TestDataTable({
  data = people,
  isLoading = false,
  onPageChange = vi.fn(),
}: {
  data?: Person[]
  isLoading?: boolean
  onPageChange?: (page: number) => void
}) {
  const columns = React.useMemo<ColumnDef<Person>[]>(
    () => [
      {
        accessorKey: "name",
        header: "Name",
      },
      {
        accessorKey: "email",
        header: "Email",
      },
    ],
    [],
  )
  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    pageCount: 3,
    state: {
      pagination: {
        pageIndex: 1,
        pageSize: 10,
      },
    },
  })

  return (
    <DataTable
      table={table}
      emptyMessage="No people"
      isLoading={isLoading}
      pagination={{
        page: 2,
        totalPages: 3,
        previousLabel: "Previous",
        nextLabel: "Next",
        pageLabel: "Page 2 of 3",
        summary: "Showing 11-20 of 25 people",
        onPageChange,
      }}
    />
  )
}

describe("DataTable", () => {
  it("renders headers and rows from a TanStack table instance", () => {
    const { getByRole, getByText } = renderWithProviders(<TestDataTable />)

    expect(getByRole("columnheader", { name: "Name" })).toBeInTheDocument()
    expect(getByRole("columnheader", { name: "Email" })).toBeInTheDocument()
    expect(getByText("Ada")).toBeInTheDocument()
    expect(getByText("grace@example.com")).toBeInTheDocument()
  })

  it("renders an empty state when the table has no rows", () => {
    const { getByText } = renderWithProviders(<TestDataTable data={[]} />)

    expect(getByText("No people")).toBeInTheDocument()
  })

  it("renders skeleton rows while loading", () => {
    const { container } = renderWithProviders(<TestDataTable isLoading />)

    expect(container.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(5)
  })

  it("calls manual pagination callbacks", async () => {
    const user = userEvent.setup()
    const onPageChange = vi.fn()
    const { getByRole } = renderWithProviders(
      <TestDataTable onPageChange={onPageChange} />,
    )

    await user.click(getByRole("button", { name: "Previous" }))
    await user.click(getByRole("button", { name: "Next" }))

    expect(onPageChange).toHaveBeenNthCalledWith(1, 1)
    expect(onPageChange).toHaveBeenNthCalledWith(2, 3)
  })
})
