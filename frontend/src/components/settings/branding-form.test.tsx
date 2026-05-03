import { QueryClient } from "@tanstack/react-query"
import { waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { BrandingForm } from "./branding-form"

const updateBrandingMock = vi.hoisted(() => vi.fn())

vi.mock("@/components/settings/settings-api", () => ({
  updateBrandingMutation: {
    mutationFn: updateBrandingMock,
  },
}))

function renderBrandingForm() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  })
  queryClient.setQueryData(["branding"], {
    product_name: "Rota",
    organization_name: "",
    version: 2,
    created_at: "2026-05-04T00:00:00Z",
    updated_at: "2026-05-04T00:00:00Z",
  })

  return renderWithProviders(<BrandingForm />, { queryClient })
}

describe("BrandingForm", () => {
  beforeEach(() => {
    updateBrandingMock.mockReset()
  })

  it("rejects blank and too-long names", async () => {
    const user = userEvent.setup()
    const { findByText, getByLabelText, getByRole } = renderBrandingForm()

    await user.clear(getByLabelText("settings.branding.productName"))
    await user.click(getByRole("button", { name: "settings.common.save" }))

    expect(
      await findByText("settings.validation.productNameRequired"),
    ).toBeInTheDocument()
    expect(updateBrandingMock).not.toHaveBeenCalled()

    await user.type(
      getByLabelText("settings.branding.productName"),
      "a".repeat(61),
    )
    await user.click(getByRole("button", { name: "settings.common.save" }))

    expect(
      await findByText("settings.validation.productNameMax"),
    ).toBeInTheDocument()
    expect(updateBrandingMock).not.toHaveBeenCalled()
  })

  it("submits trimmed product and organization names with the version", async () => {
    const user = userEvent.setup()
    updateBrandingMock.mockResolvedValue({
      product_name: "排班系统",
      organization_name: "Acme",
      version: 3,
      created_at: "2026-05-04T00:00:00Z",
      updated_at: "2026-05-04T00:01:00Z",
    })
    const { getByLabelText, getByRole } = renderBrandingForm()

    await user.clear(getByLabelText("settings.branding.productName"))
    await user.type(getByLabelText("settings.branding.productName"), " 排班系统 ")
    await user.type(getByLabelText("settings.branding.organizationName"), " Acme ")
    await user.click(getByRole("button", { name: "settings.common.save" }))

    await waitFor(() => {
      expect(updateBrandingMock).toHaveBeenCalled()
    })
    expect(updateBrandingMock.mock.calls[0][0]).toEqual({
      product_name: "排班系统",
      organization_name: "Acme",
      version: 2,
    })
  })

  it("shows a localized version conflict error", async () => {
    const user = userEvent.setup()
    updateBrandingMock.mockRejectedValue({
      isAxiosError: true,
      response: {
        data: {
          error: {
            code: "VERSION_CONFLICT",
            message: "conflict",
          },
        },
      },
    })
    const { findByText, getByRole } = renderBrandingForm()

    await user.click(getByRole("button", { name: "settings.common.save" }))

    expect(
      await findByText("settings.branding.errors.VERSION_CONFLICT"),
    ).toBeInTheDocument()
  })
})
