import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { CloneTemplateDialog } from "./clone-template-dialog"

describe("CloneTemplateDialog", () => {
  it("closes when cancel is clicked", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()

    const { getByRole } = renderWithProviders(
      <CloneTemplateDialog
        isPending={false}
        open
        onConfirm={vi.fn()}
        onOpenChange={onOpenChange}
        template={{ name: "Blueprint" } as never}
      />,
    )

    await user.click(getByRole("button", { name: "common.cancel" }))

    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it("confirms cloning", async () => {
    const user = userEvent.setup()
    const onConfirm = vi.fn()

    const { getByRole } = renderWithProviders(
      <CloneTemplateDialog
        isPending={false}
        open
        onConfirm={onConfirm}
        onOpenChange={vi.fn()}
        template={{ name: "Blueprint" } as never}
      />,
    )

    await user.click(getByRole("button", { name: "templates.cloneDialog.confirm" }))

    expect(onConfirm).toHaveBeenCalledTimes(1)
  })
})
