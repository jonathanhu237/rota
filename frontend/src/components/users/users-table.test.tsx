import { describe, expect, it, vi } from "vitest"
import userEvent from "@testing-library/user-event"

import { renderWithProviders } from "@/test-utils/render"

import { UsersTable } from "./users-table"

const users = [
  {
    id: 1,
    email: "pending@example.com",
    name: "Pending",
    is_admin: false,
    status: "pending" as const,
    version: 1,
    language_preference: null,
    theme_preference: null,
  },
  {
    id: 2,
    email: "active@example.com",
    name: "Active",
    is_admin: true,
    status: "active" as const,
    version: 1,
    language_preference: null,
    theme_preference: null,
  },
]

describe("UsersTable", () => {
  it("shows resend invitation for pending users and disables it for active users", async () => {
    const user = userEvent.setup()
    const onResendInvitation = vi.fn()
    const onToggleStatus = vi.fn()
    const onEdit = vi.fn()

    const { getAllByRole } = renderWithProviders(
      <UsersTable
        users={users}
        isLoading={false}
        isFetching={false}
        onEdit={onEdit}
        onPageChange={vi.fn()}
        onResendInvitation={onResendInvitation}
        onToggleStatus={onToggleStatus}
      />,
    )

    const resendButtons = getAllByRole("button", {
      name: "users.actions.resendInvitation",
    })

    await user.click(resendButtons[0])
    expect(onResendInvitation).toHaveBeenCalledWith(
      expect.objectContaining({ id: 1, status: "pending" }),
    )
    expect(resendButtons[0]).toBeEnabled()
    expect(resendButtons[1]).toBeDisabled()
    expect(onToggleStatus).not.toHaveBeenCalled()
  })

  it("renders columns and empty/loading states through DataTable", () => {
    const { getByRole, getByText, rerender, container } = renderWithProviders(
      <UsersTable
        users={users}
        isLoading={false}
        isFetching={false}
        onEdit={vi.fn()}
        onPageChange={vi.fn()}
        onResendInvitation={vi.fn()}
        onToggleStatus={vi.fn()}
      />,
    )

    expect(getByRole("columnheader", { name: "users.table.name" })).toBeInTheDocument()
    expect(getByText("Pending")).toBeInTheDocument()
    expect(getByText("common.admin")).toBeInTheDocument()

    rerender(
      <UsersTable
        users={[]}
        isLoading={false}
        isFetching={false}
        onEdit={vi.fn()}
        onPageChange={vi.fn()}
        onResendInvitation={vi.fn()}
        onToggleStatus={vi.fn()}
      />,
    )
    expect(getByText("users.empty")).toBeInTheDocument()

    rerender(
      <UsersTable
        users={[]}
        isLoading
        isFetching={false}
        onEdit={vi.fn()}
        onPageChange={vi.fn()}
        onResendInvitation={vi.fn()}
        onToggleStatus={vi.fn()}
      />,
    )
    expect(container.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(5)
  })

  it("requests server pages from pagination controls", async () => {
    const user = userEvent.setup()
    const onPageChange = vi.fn()

    const { getByRole } = renderWithProviders(
      <UsersTable
        users={users}
        pagination={{
          page: 2,
          page_size: 10,
          total: 25,
          total_pages: 3,
        }}
        isLoading={false}
        isFetching={false}
        onEdit={vi.fn()}
        onPageChange={onPageChange}
        onResendInvitation={vi.fn()}
        onToggleStatus={vi.fn()}
      />,
    )

    await user.click(getByRole("button", { name: "users.pagination.previous" }))
    await user.click(getByRole("button", { name: "users.pagination.next" }))

    expect(onPageChange).toHaveBeenNthCalledWith(1, 1)
    expect(onPageChange).toHaveBeenNthCalledWith(2, 3)
  })
})
