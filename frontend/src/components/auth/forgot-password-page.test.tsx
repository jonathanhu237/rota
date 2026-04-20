import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { ForgotPasswordPage } from "./forgot-password-page"

describe("ForgotPasswordPage", () => {
  it("shows a generic success message after submit", async () => {
    const user = userEvent.setup()
    const requestPasswordReset = vi.fn().mockRejectedValue(new Error("boom"))

    const { getByLabelText, getByRole, findByText } = renderWithProviders(
      <ForgotPasswordPage requestPasswordReset={requestPasswordReset} />,
    )

    await user.type(getByLabelText("forgotPassword.email"), "worker@example.com")
    await user.click(getByRole("button", { name: "forgotPassword.submit" }))

    expect(requestPasswordReset).toHaveBeenCalledWith("worker@example.com")
    expect(
      await findByText("forgotPassword.success"),
    ).toBeInTheDocument()
  })
})
