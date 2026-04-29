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
        unqualifiedDrafts={[
          {
            id: "assign-1",
            userName: "Alice",
            slotLabel: "Monday 09:00-11:00",
            positionName: "Kitchen",
          },
        ]}
        unsubmittedDrafts={[]}
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
      getByText("assignments.drafts.confirmDialog.unqualifiedSection"),
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
        unqualifiedDrafts={[]}
        unsubmittedDrafts={[]}
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

  it("renders amber-only and both-severity dialog states", () => {
    const warning = {
      id: "assign-1",
      userName: "Alice",
      slotLabel: "Monday 09:00-11:00",
      positionName: "Kitchen",
    }

    const { getByText, rerender, queryByText } = renderWithProviders(
      <DraftConfirmDialog
        open
        unqualifiedDrafts={[]}
        unsubmittedDrafts={[warning]}
        isPending={false}
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
      />,
    )

    expect(
      getByText("assignments.drafts.confirmDialog.titleUnsubmitted"),
    ).toBeInTheDocument()
    expect(
      getByText("assignments.drafts.confirmDialog.unsubmittedSection"),
    ).toBeInTheDocument()
    expect(
      queryByText("assignments.drafts.confirmDialog.unqualifiedSection"),
    ).not.toBeInTheDocument()

    rerender(
      <DraftConfirmDialog
        open
        unqualifiedDrafts={[warning]}
        unsubmittedDrafts={[{ ...warning, id: "assign-2" }]}
        isPending={false}
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
      />,
    )

    expect(
      getByText("assignments.drafts.confirmDialog.titleBoth"),
    ).toBeInTheDocument()
    expect(
      getByText("assignments.drafts.confirmDialog.unqualifiedSection"),
    ).toBeInTheDocument()
    expect(
      getByText("assignments.drafts.confirmDialog.unsubmittedSection"),
    ).toBeInTheDocument()
  })
})
