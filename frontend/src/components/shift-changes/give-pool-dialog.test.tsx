import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { ToastProvider } from "@/components/ui/toast"

import { GivePoolDialog } from "./give-pool-dialog"

function renderDialog(ui: React.ReactElement) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <ToastProvider>{ui}</ToastProvider>
    </QueryClientProvider>,
  )
}

describe("GivePoolDialog", () => {
  it("disables submit when no assignment is attached", () => {
    const { getByRole } = renderDialog(
      <GivePoolDialog
        open
        publicationID={1}
        myAssignmentID={null}
        occurrenceDate="2026-04-20"
        onOpenChange={vi.fn()}
      />,
    )

    const submit = getByRole("button", {
      name: "requests.givePoolDialog.submit",
    }) as HTMLButtonElement
    expect(submit.disabled).toBe(true)
  })

  it("enables submit when an assignment is provided", () => {
    const { getByRole } = renderDialog(
      <GivePoolDialog
        open
        publicationID={1}
        myAssignmentID={42}
        occurrenceDate="2026-04-20"
        onOpenChange={vi.fn()}
      />,
    )

    const submit = getByRole("button", {
      name: "requests.givePoolDialog.submit",
    }) as HTMLButtonElement
    expect(submit.disabled).toBe(false)
  })

  it("closes on cancel", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    const { getByRole } = renderDialog(
      <GivePoolDialog
        open
        publicationID={1}
        myAssignmentID={42}
        occurrenceDate="2026-04-20"
        onOpenChange={onOpenChange}
      />,
    )

    await user.click(getByRole("button", { name: "common.cancel" }))

    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
