import { fireEvent, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { CreatePublicationDialog } from "./create-publication-dialog"

const templates = [
  {
    id: 1,
    name: "April roster",
    description: "",
    is_locked: false,
    shift_count: 1,
    created_at: "2026-04-01T00:00:00Z",
    updated_at: "2026-04-01T00:00:00Z",
  },
]

describe("CreatePublicationDialog", () => {
  it("renders", () => {
    const { getByLabelText } = renderWithProviders(
      <CreatePublicationDialog
        isPending={false}
        isTemplatesLoading={false}
        open
        onOpenChange={vi.fn()}
        onSubmit={vi.fn()}
        templates={templates}
      />,
    )

    expect(getByLabelText("publications.form.template")).toBeInTheDocument()
    expect(getByLabelText("publications.name")).toBeInTheDocument()
    expect(getByLabelText("publications.submissionStartAt")).toBeInTheDocument()
    expect(getByLabelText("publications.submissionEndAt")).toBeInTheDocument()
    expect(getByLabelText("publications.plannedActiveFrom")).toBeInTheDocument()
  })

  it("submits valid values", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()

    renderWithProviders(
      <CreatePublicationDialog
        isPending={false}
        isTemplatesLoading={false}
        open
        onOpenChange={vi.fn()}
        onSubmit={onSubmit}
        templates={templates}
      />,
    )

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement
    await user.selectOptions(
      within(dialog).getByLabelText("publications.form.template"),
      "1",
    )
    await user.type(within(dialog).getByLabelText("publications.name"), "Week 17")
    fireEvent.change(
      within(dialog).getByLabelText("publications.submissionStartAt"),
      { target: { value: "2026-04-17T09:00" } },
    )
    fireEvent.change(
      within(dialog).getByLabelText("publications.submissionEndAt"),
      { target: { value: "2026-04-17T12:00" } },
    )
    fireEvent.change(
      within(dialog).getByLabelText("publications.plannedActiveFrom"),
      { target: { value: "2026-04-17T13:00" } },
    )
    await user.click(dialog.querySelector('button[type="submit"]')!)

    expect(onSubmit).toHaveBeenCalledWith({
      template_id: 1,
      name: "Week 17",
      submission_start_at: "2026-04-17T09:00",
      submission_end_at: "2026-04-17T12:00",
      planned_active_from: "2026-04-17T13:00",
    })
  })

  it("shows a validation error when the publication window is invalid", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()

    renderWithProviders(
      <CreatePublicationDialog
        isPending={false}
        isTemplatesLoading={false}
        open
        onOpenChange={vi.fn()}
        onSubmit={onSubmit}
        templates={templates}
      />,
    )

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement
    await user.selectOptions(
      within(dialog).getByLabelText("publications.form.template"),
      "1",
    )
    await user.type(within(dialog).getByLabelText("publications.name"), "Week 17")
    fireEvent.change(
      within(dialog).getByLabelText("publications.submissionStartAt"),
      { target: { value: "2026-04-17T12:00" } },
    )
    fireEvent.change(
      within(dialog).getByLabelText("publications.submissionEndAt"),
      { target: { value: "2026-04-17T11:00" } },
    )
    fireEvent.change(
      within(dialog).getByLabelText("publications.plannedActiveFrom"),
      { target: { value: "2026-04-17T13:00" } },
    )
    await user.click(dialog.querySelector('button[type="submit"]')!)

    expect(
      within(dialog).getByText("publications.validation.invalidWindow"),
    ).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })
})
