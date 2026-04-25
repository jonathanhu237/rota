import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import type { Pagination, Publication } from "@/lib/types"
import { renderWithProviders } from "@/test-utils/render"

import { PublicationsTable } from "./publications-table"

const publications: Publication[] = [
  {
    id: 1,
    template_id: 3,
    template_name: "Spring template",
    name: "Week 17",
    description: "",
    state: "ASSIGNING",
    submission_start_at: "2026-04-17T09:00:00Z",
    submission_end_at: "2026-04-17T10:00:00Z",
    planned_active_from: "2026-04-17T11:00:00Z",
    planned_active_until: "2026-05-01T11:00:00Z",
    activated_at: null,
    created_at: "2026-04-17T08:00:00Z",
    updated_at: "2026-04-17T08:00:00Z",
  },
  {
    id: 2,
    template_id: 4,
    template_name: "Summer template",
    name: "Week 18",
    description: "",
    state: "ACTIVE",
    submission_start_at: "2026-04-24T09:00:00Z",
    submission_end_at: "2026-04-24T10:00:00Z",
    planned_active_from: "2026-04-24T11:00:00Z",
    planned_active_until: "2026-05-08T11:00:00Z",
    activated_at: "2026-04-24T11:30:00Z",
    created_at: "2026-04-24T08:00:00Z",
    updated_at: "2026-04-24T08:00:00Z",
  },
]

const pagination: Pagination = {
  page: 2,
  page_size: 10,
  total: 25,
  total_pages: 3,
}

describe("PublicationsTable", () => {
  it("renders state badges and opens rows", async () => {
    const user = userEvent.setup()
    const onOpen = vi.fn()
    const onLifecycleAction = vi.fn()

    const { getAllByRole, getByText } = renderWithProviders(
      <PublicationsTable
        isFetching={false}
        isLoading={false}
        onLifecycleAction={onLifecycleAction}
        onOpen={onOpen}
        onPageChange={vi.fn()}
        pagination={pagination}
        publications={publications}
      />,
    )

    expect(getByText("publications.state.assigning")).toBeInTheDocument()
    expect(getByText("publications.state.active")).toBeInTheDocument()

    await user.click(getByText("Week 17"))
    await user.click(getAllByRole("button", { name: "publications.actions.publish" })[0])

    expect(onOpen).toHaveBeenCalledWith(publications[0])
    expect(onLifecycleAction).toHaveBeenCalledWith(publications[0], "publish")
  })

  it("enables pagination buttons when there are more pages", () => {
    const onPageChange = vi.fn()

    const { getAllByRole } = renderWithProviders(
      <PublicationsTable
        isFetching={false}
        isLoading={false}
        onLifecycleAction={vi.fn()}
        onOpen={vi.fn()}
        onPageChange={onPageChange}
        pagination={pagination}
        publications={publications}
      />,
    )

    expect(
      getAllByRole("button", { name: "publications.pagination.previous" })[0],
    ).toBeEnabled()
    expect(
      getAllByRole("button", { name: "publications.pagination.next" })[0],
    ).toBeEnabled()
  })

  it("disables pagination buttons on the first and only page", () => {
    const { container } = renderWithProviders(
      <PublicationsTable
        isFetching={false}
        isLoading={false}
        onLifecycleAction={vi.fn()}
        onOpen={vi.fn()}
        onPageChange={vi.fn()}
        pagination={{
          page: 1,
          page_size: 10,
          total: 0,
          total_pages: 0,
        }}
        publications={[]}
      />,
    )

    const previousButton = Array.from(container.querySelectorAll("button")).find(
      (button) => button.textContent === "publications.pagination.previous",
    )
    const nextButton = Array.from(container.querySelectorAll("button")).find(
      (button) => button.textContent === "publications.pagination.next",
    )

    expect(previousButton).toBeTruthy()
    expect(nextButton).toBeTruthy()
    expect(previousButton).toBeDisabled()
    expect(nextButton).toBeDisabled()
  })
})
