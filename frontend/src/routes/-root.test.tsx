import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { RootErrorComponent } from "./__root"

describe("RootErrorComponent", () => {
  it("renders localized fallback copy and a login link", () => {
    renderRootError()

    expect(screen.getByText("rootError.title")).toBeInTheDocument()
    expect(screen.getByText("rootError.description")).toBeInTheDocument()
    expect(screen.queryByText("Something went wrong!")).not.toBeInTheDocument()
    expect(screen.getByRole("link", { name: "rootError.login" })).toHaveAttribute(
      "href",
      "/login",
    )
  })

  it("runs the retry action", async () => {
    const user = userEvent.setup()
    const onRetry = vi.fn()

    renderRootError({ onRetry })

    await user.click(screen.getByRole("button", { name: "rootError.retry" }))

    expect(onRetry).toHaveBeenCalledOnce()
  })
})

function renderRootError(overrides: { onRetry?: () => void } = {}) {
  return render(
    <RootErrorComponent
      error={new Error("failed")}
      reset={vi.fn()}
      {...overrides}
    />,
  )
}
