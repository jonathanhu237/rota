import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { ToastProvider } from "@/components/ui/toast"
import type { PublicationMember } from "@/lib/types"

import { GiveDirectDialog } from "./give-direct-dialog"

const members: PublicationMember[] = [
  { user_id: 8, name: "Bob" },
  { user_id: 9, name: "Carol" },
]

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

describe("GiveDirectDialog", () => {
  it("renders a select with every member", () => {
    const { getByLabelText } = renderDialog(
      <GiveDirectDialog
        open
        publicationID={1}
        myAssignmentID={42}
        members={members}
        onOpenChange={vi.fn()}
      />,
    )

    const select = getByLabelText(
      "requests.giveDirectDialog.counterpartLabel",
    ) as HTMLSelectElement

    expect(Array.from(select.options).map((o) => o.text)).toEqual(
      expect.arrayContaining(["Bob", "Carol"]),
    )
  })

  it("disables submit when no assignment context is provided", () => {
    const { getByRole } = renderDialog(
      <GiveDirectDialog
        open
        publicationID={1}
        myAssignmentID={null}
        members={members}
        onOpenChange={vi.fn()}
      />,
    )

    const submit = getByRole("button", {
      name: "requests.giveDirectDialog.submit",
    }) as HTMLButtonElement
    expect(submit.disabled).toBe(true)
  })

  it("closes on cancel", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    const { getByRole } = renderDialog(
      <GiveDirectDialog
        open
        publicationID={1}
        myAssignmentID={42}
        members={members}
        onOpenChange={onOpenChange}
      />,
    )

    await user.click(getByRole("button", { name: "common.cancel" }))

    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
