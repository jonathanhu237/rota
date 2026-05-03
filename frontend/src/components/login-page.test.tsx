import type { AnchorHTMLAttributes, ForwardedRef, ReactNode } from "react"
import { QueryClient } from "@tanstack/react-query"
import { screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { LoginPage } from "./login-page"

type LinkMockProps = {
  to: string
  children?: ReactNode
} & AnchorHTMLAttributes<HTMLAnchorElement>

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    )
  const React = await vi.importActual<typeof import("react")>("react")
  const Link = React.forwardRef(function LinkMock(
    { to, children, ...props }: LinkMockProps,
    ref: ForwardedRef<HTMLAnchorElement>,
  ) {
    return React.createElement("a", { href: to, ref, ...props }, children)
  })

  return {
    ...actual,
    Link,
    useNavigate: () => vi.fn(),
  }
})

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

describe("LoginPage", () => {
  it("renders the configured product name", () => {
    renderWithProviders(<LoginPage />, {
      queryClient: queryClientWithBranding("排班系统"),
    })

    expect(screen.getByText("login.description 排班系统")).toBeInTheDocument()
    expect(screen.queryByText("Rota")).not.toBeInTheDocument()
  })
})
