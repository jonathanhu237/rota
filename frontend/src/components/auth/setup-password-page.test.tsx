import { QueryClient } from "@tanstack/react-query"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { SetupPasswordPage } from "./setup-password-page"

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

describe("SetupPasswordPage", () => {
  it("renders the configured product name", async () => {
    const previewSetupToken = vi.fn().mockResolvedValue({
      email: "worker@example.com",
      name: "Worker",
      purpose: "invitation",
    })

    const { findByText, queryByText } = renderWithProviders(
      <SetupPasswordPage token="token-123" previewSetupToken={previewSetupToken} />,
      { queryClient: queryClientWithBranding("排班系统") },
    )

    expect(
      await findByText("setupPassword.invitationDescription 排班系统"),
    ).toBeInTheDocument()
    expect(queryByText("Rota")).not.toBeInTheDocument()
  })

  it("renders a loading state on mount and then the form after preview succeeds", async () => {
    const previewSetupToken = vi.fn().mockResolvedValue({
      email: "worker@example.com",
      name: "Worker",
      purpose: "invitation",
    })

    const { getByText, findByDisplayValue } = renderWithProviders(
      <SetupPasswordPage token="token-123" previewSetupToken={previewSetupToken} />,
    )

    expect(getByText("setupPassword.loading")).toBeInTheDocument()
    expect(await findByDisplayValue("worker@example.com")).toBeInTheDocument()
  })

  it("shows an error state when preview fails", async () => {
    const previewSetupToken = vi.fn().mockRejectedValue({
      response: {
        data: {
          error: {
            code: "TOKEN_EXPIRED",
            message: "Token expired",
          },
        },
      },
      isAxiosError: true,
    })

    const { findByText } = renderWithProviders(
      <SetupPasswordPage token="token-123" previewSetupToken={previewSetupToken} />,
    )

    expect(await findByText("setupPassword.errors.TOKEN_EXPIRED")).toBeInTheDocument()
  })

  it("submits the password setup form when preview succeeds", async () => {
    const user = userEvent.setup()
    const previewSetupToken = vi.fn().mockResolvedValue({
      email: "worker@example.com",
      name: "Worker",
      purpose: "password_reset",
    })
    const submitSetupPassword = vi.fn().mockResolvedValue(undefined)
    const onSuccess = vi.fn()

    const { findByLabelText, getByRole } = renderWithProviders(
      <SetupPasswordPage
        token="token-123"
        previewSetupToken={previewSetupToken}
        submitSetupPassword={submitSetupPassword}
        onSuccess={onSuccess}
      />,
    )

    await user.type(
      await findByLabelText("setupPassword.password"),
      "pa55word",
    )
    await user.type(
      await findByLabelText("setupPassword.confirmPassword"),
      "pa55word",
    )
    await user.click(getByRole("button", { name: "setupPassword.submit" }))

    expect(submitSetupPassword).toHaveBeenCalledWith({
      token: "token-123",
      password: "pa55word",
    })
    expect(onSuccess).toHaveBeenCalledTimes(1)
  })
})
