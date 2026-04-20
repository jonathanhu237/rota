import { describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { UserFormDialog } from "./user-form-dialog"

describe("UserFormDialog", () => {
  it("does not render password fields", () => {
    const { queryByLabelText } = renderWithProviders(
      <UserFormDialog
        mode="create"
        open
        isPending={false}
        onOpenChange={vi.fn()}
        onSubmit={vi.fn()}
      />,
    )

    expect(queryByLabelText("users.password")).not.toBeInTheDocument()
    expect(queryByLabelText("users.confirmPassword")).not.toBeInTheDocument()
  })
})
