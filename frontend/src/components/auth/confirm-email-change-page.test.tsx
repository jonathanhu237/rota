import { describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { ConfirmEmailChangePage } from "./confirm-email-change-page"

function apiError(code: string) {
  return {
    isAxiosError: true,
    response: {
      data: {
        error: {
          code,
          message: code,
        },
      },
    },
  }
}

describe("ConfirmEmailChangePage", () => {
  it("renders the success branch with a re-login CTA", async () => {
    const confirmEmailChange = vi.fn().mockResolvedValue(undefined)

    const { findByText, getByRole } = renderWithProviders(
      <ConfirmEmailChangePage
        token="token-123"
        confirmEmailChange={confirmEmailChange}
      />,
    )

    expect(await findByText("confirmEmailChange.successTitle")).toBeInTheDocument()
    expect(
      getByRole("link", { name: "confirmEmailChange.backToLogin" }),
    ).toHaveAttribute("href", "/login")
    expect(confirmEmailChange).toHaveBeenCalledWith({ token: "token-123" })
  })

  it("renders the missing-token branch without calling the API", () => {
    const confirmEmailChange = vi.fn()

    const { getByText } = renderWithProviders(
      <ConfirmEmailChangePage confirmEmailChange={confirmEmailChange} />,
    )

    expect(getByText("confirmEmailChange.errors.INVALID_LINK")).toBeInTheDocument()
    expect(confirmEmailChange).not.toHaveBeenCalled()
  })

  it.each([
    "INVALID_TOKEN",
    "TOKEN_NOT_FOUND",
    "TOKEN_USED",
    "TOKEN_EXPIRED",
    "EMAIL_ALREADY_EXISTS",
  ])("renders the %s error branch", async (code) => {
    const confirmEmailChange = vi.fn().mockRejectedValue(apiError(code))

    const { findByText } = renderWithProviders(
      <ConfirmEmailChangePage
        token="token-123"
        confirmEmailChange={confirmEmailChange}
      />,
    )

    expect(await findByText(`confirmEmailChange.errors.${code}`)).toBeInTheDocument()
  })
})
