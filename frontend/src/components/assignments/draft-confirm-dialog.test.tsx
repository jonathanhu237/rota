import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { DraftConfirmDialog } from "./draft-confirm-dialog"

describe("DraftConfirmDialog", () => {
  it("lists qualification warnings and confirms submission", async () => {
    const user = userEvent.setup()
    const onConfirm = vi.fn()
    const onCancel = vi.fn()
    const onOpenChange = vi.fn()

    const { getByRole, getByText } = renderWithProviders(
      <DraftConfirmDialog
        open
        warnings={[
          {
            id: "assign-1",
            userName: "Alice",
            slotLabel: "Monday 09:00-11:00",
            positionName: "Kitchen",
          },
        ]}
        isPending={false}
        onCancel={onCancel}
        onConfirm={onConfirm}
        onOpenChange={onOpenChange}
      />,
    )

    expect(
      getByText("assignments.drafts.confirmDialog.title"),
    ).toBeInTheDocument()
    expect(
      getByText("assignments.drafts.confirmDialog.warningTitle"),
    ).toBeInTheDocument()

    await user.click(
      getByRole("button", { name: "assignments.drafts.confirmAndSubmit" }),
    )

    expect(onConfirm).toHaveBeenCalled()
  })

  it("returns null when there are no warnings", () => {
    const { queryByText } = renderWithProviders(
      <DraftConfirmDialog
        open
        warnings={[]}
        isPending={false}
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
      />,
    )

    expect(
      queryByText("assignments.drafts.confirmDialog.title"),
    ).not.toBeInTheDocument()
  })
})
