import userEvent from "@testing-library/user-event"
import { waitFor } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { PasswordForm } from "./password-form"

const changeOwnPasswordMock = vi.hoisted(() => vi.fn())

vi.mock("@/components/settings/settings-api", () => ({
  changeOwnPasswordMutation: {
    mutationFn: changeOwnPasswordMock,
  },
}))

describe("PasswordForm", () => {
  beforeEach(() => {
    changeOwnPasswordMock.mockReset()
  })

  it("rejects a new password shorter than 8 characters", async () => {
    const user = userEvent.setup()

    const { findByText, getByLabelText, getByRole } = renderWithProviders(
      <PasswordForm />,
    )

    await user.type(getByLabelText("settings.password.current"), "oldpass123")
    await user.type(getByLabelText("settings.password.new"), "short")
    await user.type(getByLabelText("settings.password.confirm"), "short")
    await user.click(getByRole("button", { name: "settings.common.save" }))

    expect(await findByText("settings.validation.passwordMin")).toBeInTheDocument()
    expect(changeOwnPasswordMock).not.toHaveBeenCalled()
  })

  it("rejects mismatched new password confirmation", async () => {
    const user = userEvent.setup()

    const { findByText, getByLabelText, getByRole } = renderWithProviders(
      <PasswordForm />,
    )

    await user.type(getByLabelText("settings.password.current"), "oldpass123")
    await user.type(getByLabelText("settings.password.new"), "newpass123")
    await user.type(getByLabelText("settings.password.confirm"), "different123")
    await user.click(getByRole("button", { name: "settings.common.save" }))

    expect(
      await findByText("settings.validation.passwordMismatch"),
    ).toBeInTheDocument()
    expect(changeOwnPasswordMock).not.toHaveBeenCalled()
  })

  it("submits the password change when inputs are valid", async () => {
    const user = userEvent.setup()
    changeOwnPasswordMock.mockResolvedValue(undefined)

    const { getByLabelText, getByRole } = renderWithProviders(<PasswordForm />)

    await user.type(getByLabelText("settings.password.current"), "oldpass123")
    await user.type(getByLabelText("settings.password.new"), "newpass123")
    await user.type(getByLabelText("settings.password.confirm"), "newpass123")
    await user.click(getByRole("button", { name: "settings.common.save" }))

    await waitFor(() => {
      expect(changeOwnPasswordMock).toHaveBeenCalled()
    })
    expect(changeOwnPasswordMock.mock.calls[0][0]).toEqual({
      current_password: "oldpass123",
      new_password: "newpass123",
    })
  })
})
