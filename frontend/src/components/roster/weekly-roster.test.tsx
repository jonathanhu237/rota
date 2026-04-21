import { afterEach, describe, expect, it, vi } from "vitest"

import type { RosterWeekday } from "@/lib/types"
import { renderWithProviders } from "@/test-utils/render"

import { WeeklyRoster } from "./weekly-roster"

const weekdays: RosterWeekday[] = Array.from({ length: 7 }, (_, index) => ({
  weekday: index + 1,
  shifts: [],
}))

describe("WeeklyRoster", () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it("renders seven weekday columns, the current day badge, empty states, and highlights the current user", () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-04-15T12:00:00+08:00"))

    const roster: RosterWeekday[] = weekdays.map((weekday) => ({
      ...weekday,
      shifts: [],
    }))
    roster[2] = {
      weekday: 3,
      shifts: [
        {
          shift: {
            id: 3,
            weekday: 3,
            start_time: "09:00",
            end_time: "12:00",
            position_id: 101,
            position_name: "Front Desk",
            required_headcount: 2,
          },
          assignments: [
            { assignment_id: 71, user_id: 7, name: "Alice" },
            { assignment_id: 81, user_id: 8, name: "Bob" },
          ],
        },
      ],
    }

    const { container, getAllByText, getByText } = renderWithProviders(
      <WeeklyRoster weekdays={roster} currentUserID={7} />,
    )

    expect(container.querySelectorAll("section")).toHaveLength(7)
    expect(getByText("roster.today")).toBeInTheDocument()
    expect(getAllByText("roster.emptyWeekday")).toHaveLength(6)
    expect(getByText("Alice").closest("div")).toHaveClass(
      "border-primary/40",
      "bg-primary/10",
      "text-primary",
    )
  })
})
