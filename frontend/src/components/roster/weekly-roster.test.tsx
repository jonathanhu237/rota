import { afterEach, describe, expect, it, vi } from "vitest"

import type { Publication, RosterWeekday } from "@/lib/types"
import { renderWithProviders } from "@/test-utils/render"

import { WeeklyRoster } from "./weekly-roster"

const frontDesk = { id: 101, name: "Front Desk" }
const cashier = { id: 102, name: "Cashier" }

function makeWeekdays(): RosterWeekday[] {
  // Mon (1)..Fri (5) — daytime slot 09:00-12:00 with 2 front-desk + 1 cashier.
  // Sat / Sun: no slots (off-schedule for the only time block in the test).
  const monday: RosterWeekday = {
    weekday: 1,
    slots: [
      {
        occurrence_date: "2026-04-13",
        slot: { id: 1, weekday: 1, start_time: "09:00", end_time: "12:00" },
        positions: [
          {
            position: frontDesk,
            required_headcount: 2,
            assignments: [
              { assignment_id: 11, user_id: 7, name: "Alice" },
              { assignment_id: 12, user_id: 8, name: "Bob" },
            ],
          },
          {
            position: cashier,
            required_headcount: 1,
            assignments: [
              { assignment_id: 13, user_id: 9, name: "Carol" },
            ],
          },
        ],
      },
    ],
  }

  // Tuesday: partial — 2/3 filled.
  const tuesday: RosterWeekday = {
    weekday: 2,
    slots: [
      {
        occurrence_date: "2026-04-14",
        slot: { id: 2, weekday: 2, start_time: "09:00", end_time: "12:00" },
        positions: [
          {
            position: frontDesk,
            required_headcount: 2,
            assignments: [
              { assignment_id: 21, user_id: 7, name: "Alice" },
            ],
          },
          {
            position: cashier,
            required_headcount: 1,
            assignments: [
              { assignment_id: 22, user_id: 9, name: "Carol" },
            ],
          },
        ],
      },
    ],
  }

  // Wednesday: empty — 0/3.
  const wednesday: RosterWeekday = {
    weekday: 3,
    slots: [
      {
        occurrence_date: "2026-04-15",
        slot: { id: 3, weekday: 3, start_time: "09:00", end_time: "12:00" },
        positions: [
          {
            position: frontDesk,
            required_headcount: 2,
            assignments: [],
          },
          {
            position: cashier,
            required_headcount: 1,
            assignments: [],
          },
        ],
      },
    ],
  }

  return [
    monday,
    tuesday,
    wednesday,
    { weekday: 4, slots: [] },
    { weekday: 5, slots: [] },
    { weekday: 6, slots: [] },
    { weekday: 7, slots: [] },
  ]
}

const publishedPublication: Publication = {
  id: 99,
  template_id: 1,
  template_name: "Test Template",
  name: "Test Pub",
  description: "",
  state: "PUBLISHED",
  submission_start_at: "2026-04-01T00:00:00Z",
  submission_end_at: "2026-04-08T00:00:00Z",
  planned_active_from: "2026-04-13T00:00:00Z",
  planned_active_until: "2026-04-20T00:00:00Z",
  activated_at: null,
  created_at: "2026-04-01T00:00:00Z",
  updated_at: "2026-04-01T00:00:00Z",
}

const activePublication: Publication = {
  ...publishedPublication,
  state: "ACTIVE",
}

describe("WeeklyRoster", () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it("renders the 2D grid with one column per weekday and a row per distinct time block", () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-04-15T12:00:00+08:00")) // Wednesday

    const { getByText, container } = renderWithProviders(
      <WeeklyRoster weekdays={makeWeekdays()} currentUserID={7} />,
    )

    // 7 weekday header cells + corner cell. Time-row label shown once.
    expect(getByText("templates.weekday.mon")).toBeInTheDocument()
    expect(getByText("templates.weekday.sun")).toBeInTheDocument()
    expect(getByText("09:00–12:00")).toBeInTheDocument()

    // 7 cells in the time row (timeBlockIndex=0).
    const cells = container.querySelectorAll('[data-testid^="roster-cell-0-"]')
    expect(cells).toHaveLength(3) // only Mon/Tue/Wed are scheduled; Thu-Sun off-schedule.
  })

  it("highlights the current weekday header with the today badge", () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-04-15T12:00:00+08:00")) // Wednesday

    const { getByText } = renderWithProviders(
      <WeeklyRoster weekdays={makeWeekdays()} />,
    )

    expect(getByText("roster.today")).toBeInTheDocument()
  })

  it("applies the correct status background per cell coverage", () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-04-15T12:00:00+08:00"))

    const { container } = renderWithProviders(
      <WeeklyRoster weekdays={makeWeekdays()} currentUserID={7} />,
    )

    const monCell = container.querySelector('[data-testid="roster-cell-0-1"]')
    const tueCell = container.querySelector('[data-testid="roster-cell-0-2"]')
    const wedCell = container.querySelector('[data-testid="roster-cell-0-3"]')

    expect(monCell?.className).toContain("bg-emerald-50")
    expect(tueCell?.className).toContain("bg-amber-50")
    expect(wedCell?.className).toContain("bg-red-50")
  })

  it("renders dashed empty-seat placeholders for unfilled headcount", () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-04-15T12:00:00+08:00"))

    const { getAllByText } = renderWithProviders(
      <WeeklyRoster weekdays={makeWeekdays()} currentUserID={7} />,
    )

    // Tuesday: 1 empty front-desk seat, 0 empty cashier
    // Wednesday: 2 empty front-desk + 1 empty cashier
    // Total: 4 empty placeholders across the visible grid.
    expect(getAllByText("roster.cell.empty")).toHaveLength(4)
  })

  it("renders off-schedule cells with the muted dash glyph", () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-04-15T12:00:00+08:00"))

    const { getAllByLabelText } = renderWithProviders(
      <WeeklyRoster weekdays={makeWeekdays()} currentUserID={7} />,
    )

    // 4 off-schedule cells: Thu, Fri, Sat, Sun.
    expect(getAllByLabelText("roster.offSchedule")).toHaveLength(4)
  })

  it("highlights the current user's chip with the primary token", () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-04-15T12:00:00+08:00"))

    const { getAllByText, getByText } = renderWithProviders(
      <WeeklyRoster weekdays={makeWeekdays()} currentUserID={7} />,
    )

    for (const aliceLabel of getAllByText("Alice")) {
      const aliceChip = aliceLabel.closest("div")
      expect(aliceChip).toHaveClass(
        "border-primary/40",
        "bg-primary/10",
        "text-primary",
      )
    }
    const bobChip = getByText("Bob").closest("div")
    expect(bobChip).not.toHaveClass("text-primary")
  })

  it("renders the swap / give-direct / give-pool menu only for self chips when state is PUBLISHED", () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-04-15T12:00:00+08:00"))

    const onShiftAction = vi.fn()

    const { getAllByLabelText } = renderWithProviders(
      <WeeklyRoster
        weekdays={makeWeekdays()}
        currentUserID={7}
        publication={publishedPublication}
        onShiftAction={onShiftAction}
      />,
    )

    // Alice is the only self chip (user 7) and appears in Mon + Tue cells = 2 menus.
    const triggers = getAllByLabelText("requests.actions.openMenu")
    expect(triggers).toHaveLength(2)
  })

  it("hides the self-chip menu when state is not PUBLISHED", () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-04-15T12:00:00+08:00"))

    const { queryAllByLabelText } = renderWithProviders(
      <WeeklyRoster
        weekdays={makeWeekdays()}
        currentUserID={7}
        publication={activePublication}
        onShiftAction={vi.fn()}
      />,
    )

    expect(queryAllByLabelText("requests.actions.openMenu")).toHaveLength(0)
  })

  it("shows an empty-week message when no slots exist anywhere", () => {
    const { getByText } = renderWithProviders(
      <WeeklyRoster
        weekdays={[
          { weekday: 1, slots: [] },
          { weekday: 2, slots: [] },
          { weekday: 3, slots: [] },
          { weekday: 4, slots: [] },
          { weekday: 5, slots: [] },
          { weekday: 6, slots: [] },
          { weekday: 7, slots: [] },
        ]}
      />,
    )

    expect(getByText("roster.emptyWeek")).toBeInTheDocument()
  })
})
