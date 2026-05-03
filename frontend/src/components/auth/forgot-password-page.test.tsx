import { QueryClient } from "@tanstack/react-query"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { ForgotPasswordPage } from "./forgot-password-page"

function queryClientWithBranding(productName: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  })
  queryClient.setQueryData(["branding"], {
    product_name: productName,
    organization_name: "Acme",
    version: 2,
    created_at: "",
    updated_at: "",
  })
  return queryClient
}

describe("ForgotPasswordPage", () => {
  it("renders the configured product name", () => {
    const { getByText, queryByText } = renderWithProviders(
      <ForgotPasswordPage />,
      { queryClient: queryClientWithBranding("排班系统") },
    )

    expect(getByText("forgotPassword.description 排班系统")).toBeInTheDocument()
    expect(queryByText("Rota")).not.toBeInTheDocument()
  })

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

  it("shows a rate limit error when the API returns TOO_MANY_REQUESTS", async () => {
    const user = userEvent.setup()
    const requestPasswordReset = vi.fn().mockRejectedValue({
      isAxiosError: true,
      response: {
        data: {
          error: {
            code: "TOO_MANY_REQUESTS",
            message: "Too many requests",
          },
        },
      },
    })

    const { getByLabelText, getByRole, findByText, queryByText } =
      renderWithProviders(
        <ForgotPasswordPage requestPasswordReset={requestPasswordReset} />,
      )

    await user.type(getByLabelText("forgotPassword.email"), "worker@example.com")
    await user.click(getByRole("button", { name: "forgotPassword.submit" }))

    expect(
      await findByText("forgotPassword.errors.TOO_MANY_REQUESTS"),
    ).toBeInTheDocument()
    expect(queryByText("forgotPassword.success")).not.toBeInTheDocument()
  })
})
