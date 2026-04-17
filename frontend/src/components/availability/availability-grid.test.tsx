import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import type { TemplateShift } from "@/lib/types"
import { renderWithProviders } from "@/test-utils/render"

import { AvailabilityGrid } from "./availability-grid"

const shifts: TemplateShift[] = [
  {
    id: 1,
    template_id: 3,
    weekday: 1,
    start_time: "09:00",
    end_time: "11:00",
    position_id: 101,
    required_headcount: 2,
    created_at: "2026-04-01T00:00:00Z",
    updated_at: "2026-04-01T00:00:00Z",
  },
  {
    id: 2,
    template_id: 3,
    weekday: 2,
    start_time: "12:00",
    end_time: "14:00",
    position_id: 102,
    required_headcount: 1,
    created_at: "2026-04-01T00:00:00Z",
    updated_at: "2026-04-01T00:00:00Z",
  },
]

describe("AvailabilityGrid", () => {
  it("renders shifts grouped by weekday and toggles selection", async () => {
    const user = userEvent.setup()
    const onToggle = vi.fn()

    const { getByText, getAllByRole } = renderWithProviders(
      <AvailabilityGrid
        isPending={false}
        onToggle={onToggle}
        selectedShiftIDs={[2]}
        shifts={shifts}
      />,
    )

    expect(getByText("templates.weekday.mon")).toBeInTheDocument()
    expect(getByText("templates.weekday.tue")).toBeInTheDocument()
    expect(getAllByRole("checkbox")[0]).not.toBeChecked()
    expect(getAllByRole("checkbox")[1]).toBeChecked()

    await user.click(getAllByRole("checkbox")[0])
    await user.click(getAllByRole("checkbox")[1])

    expect(onToggle).toHaveBeenNthCalledWith(1, 1, true)
    expect(onToggle).toHaveBeenNthCalledWith(2, 2, false)
  })

  it("disables checkboxes while pending", () => {
    const { container } = renderWithProviders(
      <AvailabilityGrid
        isPending
        onToggle={vi.fn()}
        selectedShiftIDs={[]}
        shifts={shifts}
      />,
    )

    expect(
      Array.from(container.querySelectorAll('input[type="checkbox"]')).every(
        (checkbox) => checkbox.hasAttribute("disabled"),
      ),
    ).toBe(true)
  })
})
