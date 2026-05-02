import { fireEvent, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { DatePicker, DateTimePicker, TimePicker } from "./date-time-picker"

describe("DatePicker", () => {
  it("emits YYYY-MM-DD values from calendar selection", async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()

    renderWithProviders(
      <DatePicker
        id="date"
        value="2026-04-17"
        onChange={onChange}
        placeholder="Pick date"
      />,
    )

    await user.click(screen.getByRole("button", { name: /Apr 17, 2026/ }))
    fireEvent.click(document.querySelector('[data-day="4/18/2026"]')!)

    expect(onChange).toHaveBeenCalledWith("2026-04-18")
  })
})

describe("TimePicker", () => {
  it("emits HH:MM values and empty strings", () => {
    const onChange = vi.fn()

    renderWithProviders(
      <TimePicker aria-label="Time" value="09:00" onChange={onChange} />,
    )

    fireEvent.change(screen.getByLabelText("Time"), {
      target: { value: "10:30" },
    })
    fireEvent.change(screen.getByLabelText("Time"), {
      target: { value: "" },
    })

    expect(onChange).toHaveBeenNthCalledWith(1, "10:30")
    expect(onChange).toHaveBeenNthCalledWith(2, "")
  })
})

describe("DateTimePicker", () => {
  it("preserves YYYY-MM-DDTHH:mm values when time changes", () => {
    const onChange = vi.fn()

    renderWithProviders(
      <DateTimePicker
        id="datetime"
        value="2026-04-17T09:00"
        onChange={onChange}
        placeholder="Pick date"
        timeLabel="Time"
      />,
    )

    fireEvent.change(screen.getByLabelText("Time"), {
      target: { value: "11:45" },
    })

    expect(onChange).toHaveBeenCalledWith("2026-04-17T11:45")
  })

  it("keeps date-only selection as empty form state until time is entered", () => {
    const onChange = vi.fn()

    renderWithProviders(
      <DateTimePicker
        id="datetime"
        value=""
        onChange={onChange}
        placeholder="Pick date"
        timeLabel="Time"
      />,
    )

    fireEvent.click(screen.getByRole("button", { name: "Pick date" }))
    const dateCell = document.querySelector(
      "[data-day]:not([data-outside])",
    ) as HTMLElement
    const selectedDate = formatCalendarDataDay(dateCell.dataset.day ?? "")
    fireEvent.click(dateCell.querySelector("button") ?? dateCell)

    expect(onChange).toHaveBeenCalledWith("")
    expect(screen.getByLabelText("Time")).toBeEnabled()

    fireEvent.change(screen.getByLabelText("Time"), {
      target: { value: "11:45" },
    })

    expect(onChange).toHaveBeenLastCalledWith(`${selectedDate}T11:45`)
  })

  it("represents invalid or cleared values as empty form state", () => {
    const onChange = vi.fn()

    renderWithProviders(
      <DateTimePicker
        id="datetime"
        value=""
        onChange={onChange}
        placeholder="Pick date"
        timeLabel="Time"
      />,
    )

    expect(screen.getByRole("button", { name: "Pick date" })).toBeInTheDocument()
    expect(screen.getByLabelText("Time")).toHaveValue("")
    expect(screen.getByLabelText("Time")).toBeDisabled()
  })
})

function formatCalendarDataDay(value: string) {
  if (/^\d{4}-\d{2}-\d{2}$/.test(value)) {
    return value
  }

  const [month, day, year] = value.split("/")
  return `${year}-${month.padStart(2, "0")}-${day.padStart(2, "0")}`
}
