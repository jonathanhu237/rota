import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { DeletePublicationDialog } from "./delete-publication-dialog"

describe("DeletePublicationDialog", () => {
  it("closes when cancel is clicked", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()

    const { getByRole } = renderWithProviders(
      <DeletePublicationDialog
        isPending={false}
        open
        onConfirm={vi.fn()}
        onOpenChange={onOpenChange}
        publication={{ name: "Week 17" } as never}
      />,
    )

    await user.click(getByRole("button", { name: "common.cancel" }))

    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it("confirms deletion", async () => {
    const user = userEvent.setup()
    const onConfirm = vi.fn()

    const { getByRole } = renderWithProviders(
      <DeletePublicationDialog
        isPending={false}
        open
        onConfirm={onConfirm}
        onOpenChange={vi.fn()}
        publication={{ name: "Week 17" } as never}
      />,
    )

    await user.click(
      getByRole("button", { name: "publications.deleteDialog.confirm" }),
    )

    expect(onConfirm).toHaveBeenCalledTimes(1)
  })
})
