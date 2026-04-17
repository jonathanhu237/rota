import { fireEvent, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { TemplateFormDialog } from "./template-form-dialog"

describe("TemplateFormDialog", () => {
  it("renders empty fields when opened", () => {
    const { getByLabelText } = renderWithProviders(
      <TemplateFormDialog
        isPending={false}
        open
        onOpenChange={vi.fn()}
        onSubmit={vi.fn()}
      />,
    )

    expect(getByLabelText("templates.name")).toHaveValue("")
    expect(getByLabelText("templates.descriptionLabel")).toHaveValue("")
  })

  it("submits valid values", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()

    renderWithProviders(
      <TemplateFormDialog
        isPending={false}
        open
        onOpenChange={vi.fn()}
        onSubmit={onSubmit}
      />,
    )

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement
    await user.type(within(dialog).getByLabelText("templates.name"), "Morning crew")
    await user.type(
      within(dialog).getByLabelText("templates.descriptionLabel"),
      "Weekday morning coverage",
    )
    await user.click(dialog.querySelector('button[type="submit"]')!)

    expect(onSubmit).toHaveBeenCalledWith({
      name: "Morning crew",
      description: "Weekday morning coverage",
    })
  })

  it("shows a required name error", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()

    renderWithProviders(
      <TemplateFormDialog
        isPending={false}
        open
        onOpenChange={vi.fn()}
        onSubmit={onSubmit}
      />,
    )

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement
    await user.click(dialog.querySelector('button[type="submit"]')!)

    expect(
      within(dialog).getByText("templates.validation.nameRequired"),
    ).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("shows a name length error", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()

    renderWithProviders(
      <TemplateFormDialog
        isPending={false}
        open
        onOpenChange={vi.fn()}
        onSubmit={onSubmit}
      />,
    )

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement
    const nameInput = within(dialog).getByLabelText("templates.name")

    fireEvent.change(nameInput, { target: { value: "a".repeat(101) } })
    await user.click(dialog.querySelector('button[type="submit"]')!)

    expect(
      within(dialog).getByText("templates.validation.nameTooLong"),
    ).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })
})
