import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import type { Pagination, Template } from "@/lib/types"
import { renderWithProviders } from "@/test-utils/render"

import { TemplatesTable } from "./templates-table"

const templates: Template[] = [
  {
    id: 1,
    name: "Spring Week",
    description: "",
    is_locked: false,
    shift_count: 8,
    created_at: "2026-04-17T08:00:00Z",
    updated_at: "2026-04-18T08:00:00Z",
  },
  {
    id: 2,
    name: "Locked Week",
    description: "",
    is_locked: true,
    shift_count: 12,
    created_at: "2026-04-17T08:00:00Z",
    updated_at: "2026-04-19T08:00:00Z",
  },
]

const pagination: Pagination = {
  page: 2,
  page_size: 10,
  total: 25,
  total_pages: 3,
}

describe("TemplatesTable", () => {
  it("renders columns and opens a template row", async () => {
    const user = userEvent.setup()
    const onOpen = vi.fn()

    const { getByRole, getByText } = renderWithProviders(
      <TemplatesTable
        templates={templates}
        pagination={pagination}
        isLoading={false}
        isFetching={false}
        onOpen={onOpen}
        onPageChange={vi.fn()}
      />,
    )

    expect(
      getByRole("columnheader", { name: "templates.table.name" }),
    ).toBeInTheDocument()
    expect(getByText("Spring Week")).toBeInTheDocument()
    expect(getByText("templates.unlocked")).toBeInTheDocument()
    expect(getByText("templates.locked")).toBeInTheDocument()

    await user.click(getByText("Spring Week"))

    expect(onOpen).toHaveBeenCalledWith(templates[0])
  })

  it("renders empty and loading states", () => {
    const { getByText, rerender, container } = renderWithProviders(
      <TemplatesTable
        templates={[]}
        isLoading={false}
        isFetching={false}
        onOpen={vi.fn()}
        onPageChange={vi.fn()}
      />,
    )

    expect(getByText("templates.empty")).toBeInTheDocument()

    rerender(
      <TemplatesTable
        templates={[]}
        isLoading
        isFetching={false}
        onOpen={vi.fn()}
        onPageChange={vi.fn()}
      />,
    )
    expect(container.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(5)
  })

  it("requests server pages from pagination controls", async () => {
    const user = userEvent.setup()
    const onPageChange = vi.fn()

    const { getByRole } = renderWithProviders(
      <TemplatesTable
        templates={templates}
        pagination={pagination}
        isLoading={false}
        isFetching={false}
        onOpen={vi.fn()}
        onPageChange={onPageChange}
      />,
    )

    await user.click(getByRole("button", { name: "templates.pagination.previous" }))
    await user.click(getByRole("button", { name: "templates.pagination.next" }))

    expect(onPageChange).toHaveBeenNthCalledWith(1, 1)
    expect(onPageChange).toHaveBeenNthCalledWith(2, 3)
  })
})
