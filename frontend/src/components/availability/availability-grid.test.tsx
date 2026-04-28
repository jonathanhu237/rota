import { screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"

import type { QualifiedShift } from "@/lib/types"
import { renderWithProviders } from "@/test-utils/render"

import { AvailabilityGrid } from "./availability-grid"

vi.mock("react-i18next", async () => {
  const actual =
    await vi.importActual<typeof import("react-i18next")>("react-i18next")

  function t(key: string, options?: Record<string, unknown>) {
    if (key === "availability.shift.timeRange") {
      return `${String(options?.startTime)}-${String(options?.endTime)}`
    }

    if (key === "availability.shift.compositionEntry") {
      return `${String(options?.position)} × ${String(options?.count)}`
    }

    if (key === "availability.shift.composition") {
      return String(options?.summary)
    }

    return key
  }

  return {
    ...actual,
    useTranslation: () => ({
      t,
      i18n: {
        language: "en",
        resolvedLanguage: "en",
        changeLanguage: vi.fn(),
      },
    }),
  }
})

const shifts: QualifiedShift[] = [
  {
    slot_id: 21,
    weekday: 1,
    start_time: "09:00",
    end_time: "11:00",
    composition: [
      {
        position_id: 101,
        position_name: "Front Desk",
        required_headcount: 2,
      },
      {
        position_id: 102,
        position_name: "Cashier",
        required_headcount: 1,
      },
    ],
  },
  {
    slot_id: 22,
    weekday: 2,
    start_time: "09:00",
    end_time: "11:00",
    composition: [
      {
        position_id: 103,
        position_name: "Stock",
        required_headcount: 1,
      },
    ],
  },
  {
    slot_id: 23,
    weekday: 3,
    start_time: "12:00",
    end_time: "14:00",
    composition: [
      {
        position_id: 104,
        position_name: "Floor",
        required_headcount: 3,
      },
    ],
  },
]

describe("AvailabilityGrid", () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it("renders the 2D grid header with one body row per distinct time block", () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-04-15T12:00:00+08:00")) // Wednesday

    renderWithProviders(
      <AvailabilityGrid
        isPending={false}
        onToggle={vi.fn()}
        selectedSlots={[]}
        shifts={shifts}
      />,
    )

    expect(
      screen.getByRole("grid", { name: "availability.gridTitle" }),
    ).toBeInTheDocument()
    expect(screen.getAllByRole("columnheader")).toHaveLength(7)
    expect(screen.getByText("templates.weekday.mon")).toBeInTheDocument()
    expect(screen.getByText("templates.weekday.sun")).toBeInTheDocument()
    expect(screen.getAllByRole("rowheader")).toHaveLength(2)
    expect(screen.getByText("09:00-11:00")).toBeInTheDocument()
    expect(screen.getByText("12:00-14:00")).toBeInTheDocument()
    expect(screen.getAllByRole("gridcell")).toHaveLength(14)
  })

  it("highlights today's weekday header with the today badge", () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-04-15T12:00:00+08:00")) // Wednesday

    renderWithProviders(
      <AvailabilityGrid
        isPending={false}
        onToggle={vi.fn()}
        selectedSlots={[]}
        shifts={shifts}
      />,
    )

    const wednesday = screen.getByTestId("availability-weekday-header-3")
    expect(wednesday).toHaveClass("bg-primary/10", "text-primary")
    expect(screen.getByText("availability.today")).toBeInTheDocument()

    for (const weekday of [1, 2, 4, 5, 6, 7]) {
      expect(
        screen.getByTestId(`availability-weekday-header-${weekday}`),
      ).not.toHaveClass("bg-primary/10")
    }
  })

  it("renders qualified checkboxes from selected slots and toggles with the correct payload", async () => {
    const user = userEvent.setup()
    const onToggle = vi.fn()

    renderWithProviders(
      <AvailabilityGrid
        isPending={false}
        onToggle={onToggle}
        selectedSlots={[{ slot_id: 22, weekday: 2 }]}
        shifts={shifts}
      />,
    )

    const checkboxes = screen.getAllByRole("checkbox")
    expect(checkboxes).toHaveLength(3)
    expect(checkboxes[0]).not.toBeChecked()
    expect(checkboxes[1]).toBeChecked()

    await user.click(checkboxes[0])
    await user.click(checkboxes[1])

    expect(onToggle).toHaveBeenNthCalledWith(1, 21, 1, true)
    expect(onToggle).toHaveBeenNthCalledWith(2, 22, 2, false)
  })

  it("renders off-schedule cells with a muted dash and no checkbox", () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-04-15T12:00:00+08:00"))

    renderWithProviders(
      <AvailabilityGrid
        isPending={false}
        onToggle={vi.fn()}
        selectedSlots={[]}
        shifts={shifts}
      />,
    )

    const offScheduleCells = screen.getAllByLabelText(
      "availability.offSchedule",
    )
    expect(offScheduleCells).toHaveLength(11)
    expect(screen.getAllByText("—")).toHaveLength(11)
    expect(
      offScheduleCells.every(
        (cell) => within(cell).queryByRole("checkbox") === null,
      ),
    ).toBe(true)
  })

  it("includes the composition summary in the qualified checkbox accessible name", () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-04-15T12:00:00+08:00"))

    renderWithProviders(
      <AvailabilityGrid
        isPending={false}
        onToggle={vi.fn()}
        selectedSlots={[]}
        shifts={shifts}
      />,
    )

    expect(
      screen.getByRole("checkbox", {
        name: /templates\.weekday\.mon 09:00-11:00 Front Desk × 2 \/ Cashier × 1/,
      }),
    ).toBeInTheDocument()
  })

  it("disables checkboxes while pending", () => {
    renderWithProviders(
      <AvailabilityGrid
        isPending
        onToggle={vi.fn()}
        selectedSlots={[]}
        shifts={shifts}
      />,
    )

    expect(
      screen
        .getAllByRole("checkbox")
        .every((checkbox) => checkbox.hasAttribute("disabled")),
    ).toBe(true)
  })

  it("renders weekday headers with no body rows for empty input", () => {
    renderWithProviders(
      <AvailabilityGrid
        isPending={false}
        onToggle={vi.fn()}
        selectedSlots={[]}
        shifts={[]}
      />,
    )

    expect(screen.getAllByRole("columnheader")).toHaveLength(7)
    expect(screen.queryAllByRole("rowheader")).toHaveLength(0)
    expect(screen.queryAllByRole("gridcell")).toHaveLength(0)
    expect(screen.queryAllByRole("checkbox")).toHaveLength(0)
  })
})
