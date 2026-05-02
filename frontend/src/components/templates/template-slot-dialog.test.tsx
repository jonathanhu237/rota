import { fireEvent, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { TemplateSlotDialog } from "./template-slot-dialog"

describe("TemplateSlotDialog", () => {
  it("submits HH:MM time values", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()

    renderWithProviders(
      <TemplateSlotDialog
        mode="create"
        isPending={false}
        open
        onOpenChange={vi.fn()}
        onSubmit={onSubmit}
      />,
    )

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement
    fireEvent.change(within(dialog).getByLabelText("templates.slot.startTime"), {
      target: { value: "08:30" },
    })
    fireEvent.change(within(dialog).getByLabelText("templates.slot.endTime"), {
      target: { value: "12:15" },
    })
    await user.click(dialog.querySelector('button[type="submit"]')!)

    expect(onSubmit).toHaveBeenCalledWith({
      weekdays: [1, 2, 3, 4, 5],
      start_time: "08:30",
      end_time: "12:15",
    })
  })

  it("rejects invalid time ranges", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()

    renderWithProviders(
      <TemplateSlotDialog
        mode="create"
        isPending={false}
        open
        onOpenChange={vi.fn()}
        onSubmit={onSubmit}
      />,
    )

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement
    fireEvent.change(within(dialog).getByLabelText("templates.slot.startTime"), {
      target: { value: "12:15" },
    })
    fireEvent.change(within(dialog).getByLabelText("templates.slot.endTime"), {
      target: { value: "08:30" },
    })
    await user.click(dialog.querySelector('button[type="submit"]')!)

    expect(
      within(dialog).getByText("templates.validation.invalidShiftTime"),
    ).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })
})
