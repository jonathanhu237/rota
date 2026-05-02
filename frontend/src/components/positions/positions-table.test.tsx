import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import type { Pagination, Position } from "@/lib/types"
import { renderWithProviders } from "@/test-utils/render"

import { PositionsTable } from "./positions-table"

const positions: Position[] = [
  {
    id: 1,
    name: "Front Desk",
    description: "Handles guests",
    created_at: "2026-04-17T08:00:00Z",
    updated_at: "2026-04-17T08:00:00Z",
  },
  {
    id: 2,
    name: "Cashier",
    description: "",
    created_at: "2026-04-17T08:00:00Z",
    updated_at: "2026-04-17T08:00:00Z",
  },
]

const pagination: Pagination = {
  page: 2,
  page_size: 10,
  total: 25,
  total_pages: 3,
}

describe("PositionsTable", () => {
  it("renders columns, rows, and row actions", async () => {
    const user = userEvent.setup()
    const onDelete = vi.fn()
    const onEdit = vi.fn()

    const { getAllByRole, getByRole, getByText } = renderWithProviders(
      <PositionsTable
        positions={positions}
        pagination={pagination}
        isLoading={false}
        isFetching={false}
        onDelete={onDelete}
        onEdit={onEdit}
        onPageChange={vi.fn()}
      />,
    )

    expect(
      getByRole("columnheader", { name: "positions.table.name" }),
    ).toBeInTheDocument()
    expect(getByText("Front Desk")).toBeInTheDocument()
    expect(getByText("positions.noDescription")).toBeInTheDocument()

    await user.click(getAllByRole("button", { name: "positions.actions.edit" })[0])
    await user.click(
      getAllByRole("button", { name: "positions.actions.delete" })[0],
    )

    expect(onEdit).toHaveBeenCalledWith(positions[0])
    expect(onDelete).toHaveBeenCalledWith(positions[0])
  })

  it("renders empty and loading states", () => {
    const { getByText, rerender, container } = renderWithProviders(
      <PositionsTable
        positions={[]}
        isLoading={false}
        isFetching={false}
        onDelete={vi.fn()}
        onEdit={vi.fn()}
        onPageChange={vi.fn()}
      />,
    )

    expect(getByText("positions.empty")).toBeInTheDocument()

    rerender(
      <PositionsTable
        positions={[]}
        isLoading
        isFetching={false}
        onDelete={vi.fn()}
        onEdit={vi.fn()}
        onPageChange={vi.fn()}
      />,
    )
    expect(container.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(5)
  })

  it("requests server pages from pagination controls", async () => {
    const user = userEvent.setup()
    const onPageChange = vi.fn()

    const { getByRole } = renderWithProviders(
      <PositionsTable
        positions={positions}
        pagination={pagination}
        isLoading={false}
        isFetching={false}
        onDelete={vi.fn()}
        onEdit={vi.fn()}
        onPageChange={onPageChange}
      />,
    )

    await user.click(getByRole("button", { name: "positions.pagination.previous" }))
    await user.click(getByRole("button", { name: "positions.pagination.next" }))

    expect(onPageChange).toHaveBeenNthCalledWith(1, 1)
    expect(onPageChange).toHaveBeenNthCalledWith(2, 3)
  })
})
