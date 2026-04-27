import { describe, expect, it, vi } from "vitest"
import userEvent from "@testing-library/user-event"

import { renderWithProviders } from "@/test-utils/render"

import { UsersTable } from "./users-table"

describe("UsersTable", () => {
  it("shows resend invitation for pending users and disables it for active users", async () => {
    const user = userEvent.setup()
    const onResendInvitation = vi.fn()
    const onToggleStatus = vi.fn()
    const onEdit = vi.fn()

    const { getAllByRole } = renderWithProviders(
      <UsersTable
        users={[
          {
            id: 1,
            email: "pending@example.com",
            name: "Pending",
            is_admin: false,
            status: "pending",
            version: 1,
            language_preference: null,
            theme_preference: null,
          },
          {
            id: 2,
            email: "active@example.com",
            name: "Active",
            is_admin: false,
            status: "active",
            version: 1,
            language_preference: null,
            theme_preference: null,
          },
        ]}
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
})
