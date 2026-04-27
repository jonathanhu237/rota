import { waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"
import type { User } from "@/lib/types"

import { EmailForm } from "./email-form"

const requestEmailChangeMock = vi.hoisted(() => vi.fn())

vi.mock("@/components/settings/settings-api", () => ({
  requestEmailChangeMutation: {
    mutationFn: requestEmailChangeMock,
  },
}))

const currentUser: User = {
  id: 1,
  email: "alice@example.com",
  name: "Alice",
  is_admin: false,
  status: "active",
  version: 1,
  language_preference: "en",
  theme_preference: "system",
}

function renderEmailForm() {
  return renderWithProviders(<EmailForm user={currentUser} />)
}

describe("EmailForm", () => {
  beforeEach(() => {
    requestEmailChangeMock.mockReset()
  })

  it("rejects an invalid new email before submitting", async () => {
    const user = userEvent.setup()
    const { findByText, getByLabelText, getByRole } = renderEmailForm()

    await user.click(getByRole("button", { name: "settings.email.changeButton" }))
    await user.type(getByLabelText("settings.email.dialog.newEmail"), "not-email")
    await user.type(getByLabelText("settings.email.dialog.currentPassword"), "pa55word")
    await user.click(getByRole("button", { name: "settings.email.dialog.submit" }))

    expect(await findByText("settings.validation.emailInvalid")).toBeInTheDocument()
    expect(requestEmailChangeMock).not.toHaveBeenCalled()
  })

  it("submits the email change request when inputs are valid", async () => {
    const user = userEvent.setup()
    requestEmailChangeMock.mockResolvedValue(undefined)
    const { findByText, getByLabelText, getByRole } = renderEmailForm()

    await user.click(getByRole("button", { name: "settings.email.changeButton" }))
    await user.type(getByLabelText("settings.email.dialog.newEmail"), "alice2@example.com")
    await user.type(getByLabelText("settings.email.dialog.currentPassword"), "pa55word")
    await user.click(getByRole("button", { name: "settings.email.dialog.submit" }))

    await waitFor(() => {
      expect(requestEmailChangeMock.mock.calls[0][0]).toEqual({
        new_email: "alice2@example.com",
        current_password: "pa55word",
      })
    })
    expect(await findByText("settings.email.dialog.sent")).toBeInTheDocument()
  })

  it("maps server errors to the matching form field", async () => {
    const user = userEvent.setup()
    requestEmailChangeMock.mockRejectedValue({
      isAxiosError: true,
      response: {
        data: {
          error: {
            code: "EMAIL_ALREADY_EXISTS",
            message: "Email already exists",
          },
        },
      },
    })
    const { getByLabelText, getByRole } = renderEmailForm()

    await user.click(getByRole("button", { name: "settings.email.changeButton" }))
    const dialog = document.querySelector('[role="dialog"]') as HTMLElement
    await user.type(
      within(dialog).getByLabelText("settings.email.dialog.newEmail"),
      "alice2@example.com",
    )
    await user.type(getByLabelText("settings.email.dialog.currentPassword"), "pa55word")
    await user.click(getByRole("button", { name: "settings.email.dialog.submit" }))

    expect(
      await within(dialog).findByText("settings.email.errors.EMAIL_ALREADY_EXISTS"),
    ).toBeInTheDocument()
  })
})
