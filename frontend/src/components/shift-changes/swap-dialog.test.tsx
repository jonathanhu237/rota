import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { ToastProvider } from "@/components/ui/toast"
import type { PublicationMember, RosterWeekday } from "@/lib/types"

import { SwapDialog, type SwapDialogMyShift } from "./swap-dialog"

const members: PublicationMember[] = [
  { user_id: 8, name: "Bob" },
  { user_id: 9, name: "Carol" },
]

const rosterWeekdays: RosterWeekday[] = [
  {
    weekday: 2,
    slots: [
      {
        occurrence_date: "2026-04-21",
        slot: {
          id: 200,
          weekday: 2,
          start_time: "09:00",
          end_time: "12:00",
        },
        positions: [
          {
            position: {
              id: 1,
              name: "Front Desk",
            },
            required_headcount: 1,
            assignments: [{ assignment_id: 51, user_id: 8, name: "Bob" }],
          },
        ],
      },
    ],
  },
]

const myShift: SwapDialogMyShift = {
  assignmentID: 1,
  weekday: 1,
  occurrenceDate: "2026-04-20",
  slot: {
    id: 100,
    weekday: 1,
    start_time: "09:00",
    end_time: "12:00",
  },
  position: {
    id: 1,
    name: "Front Desk",
  },
}

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

describe("SwapDialog", () => {
  it("renders member options and prompts the user to pick a shift", () => {
    const { getByLabelText } = renderDialog(
      <SwapDialog
        open
        publicationID={1}
        myShift={myShift}
        members={members}
        rosterWeekdays={rosterWeekdays}
        onOpenChange={vi.fn()}
      />,
    )

    const counterpartSelect = getByLabelText(
      "requests.swapDialog.counterpartLabel",
    ) as HTMLSelectElement
    const options = Array.from(counterpartSelect.options).map((o) => o.text)
    expect(options).toContain("Bob")
    expect(options).toContain("Carol")

    const shiftSelect = getByLabelText(
      "requests.swapDialog.counterpartShiftLabel",
    ) as HTMLSelectElement
    expect(shiftSelect.disabled).toBe(true)
  })

  it("closes when cancel is clicked", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    const { getByRole } = renderDialog(
      <SwapDialog
        open
        publicationID={1}
        myShift={myShift}
        members={members}
        rosterWeekdays={rosterWeekdays}
        onOpenChange={onOpenChange}
      />,
    )

    await user.click(getByRole("button", { name: "common.cancel" }))

    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
