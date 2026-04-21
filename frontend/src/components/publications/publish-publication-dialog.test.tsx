import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { PublishPublicationDialog } from "./publish-publication-dialog"

describe("PublishPublicationDialog", () => {
  it("closes when cancel is clicked", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()

    const { getByRole } = renderWithProviders(
      <PublishPublicationDialog
        isPending={false}
        open
        onConfirm={vi.fn()}
        onOpenChange={onOpenChange}
        publication={{ name: "Week 17" } as never}
      />,
    )

    await user.click(
      getByRole("button", { name: "publications.publishDialog.cancel" }),
    )

    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it("confirms publish", async () => {
    const user = userEvent.setup()
    const onConfirm = vi.fn()

    const { getByRole } = renderWithProviders(
      <PublishPublicationDialog
        isPending={false}
        open
        onConfirm={onConfirm}
        onOpenChange={vi.fn()}
        publication={{ name: "Week 17" } as never}
      />,
    )

    await user.click(
      getByRole("button", { name: "publications.publishDialog.confirm" }),
    )

    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it("disables the confirm button while pending", () => {
    const { getByRole } = renderWithProviders(
      <PublishPublicationDialog
        isPending
        open
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
        publication={{ name: "Week 17" } as never}
      />,
    )

    expect(
      getByRole("button", { name: "publications.publishDialog.submitting" }),
    ).toBeDisabled()
  })
})
