import { DndContext } from "@dnd-kit/core"
import { screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it } from "vitest"

import type { Employee } from "@/components/assignments/assignment-board-directory"
import type { AssignmentBoardSlot } from "@/lib/types"
import { renderWithProviders } from "@/test-utils/render"

import { AssignmentBoardSidePanel } from "./assignment-board-side-panel"
import { emptyDraftState } from "./draft-state"

const slots: AssignmentBoardSlot[] = [
  {
    slot: {
      id: 1,
      weekday: 1,
      start_time: "09:00",
      end_time: "11:00",
    },
    positions: [
      {
        position: { id: 101, name: "Front Desk" },
        required_headcount: 1,
        assignments: [
          {
            assignment_id: 20,
            user_id: 11,
            name: "Bob",
            email: "bob@example.com",
          },
        ],
      },
    ],
  },
]

const directory = new Map<number, Employee>([
  [
    10,
    {
      user_id: 10,
      name: "Alice",
      email: "alice@example.com",
      position_ids: new Set([101]),
      submittedSlots: new Set(["1:1"]),
    },
  ],
  [
    11,
    {
      user_id: 11,
      name: "Bob",
      email: "bob@example.com",
      position_ids: new Set([101]),
      submittedSlots: new Set(["1:1"]),
    },
  ],
  [
    12,
    {
      user_id: 12,
      name: "Dana",
      email: "dana@example.com",
      position_ids: new Set([101]),
      submittedSlots: new Set(),
    },
  ],
])

describe("AssignmentBoardSidePanel", () => {
  it("keeps the directory sticky on desktop while the employee list scrolls internally", () => {
    renderPanel()

    const panel = screen
      .getByText("assignments.directory.title")
      .closest("aside")
    expect(panel).toBeInTheDocument()
    expect(panel).toHaveClass(
      "xl:sticky",
      "xl:top-4",
      "xl:max-h-[calc(100svh-2rem)]",
      "xl:self-start",
    )

    const employeeList = panel?.querySelector(".overflow-y-auto")
    expect(employeeList).toBeInTheDocument()
  })

  it("splits employees by submitted availability and computes stats over submitters", () => {
    renderPanel()

    expect(screen.getByText("assignments.directory.submitted")).toBeInTheDocument()
    expect(screen.getByText("assignments.directory.notSubmitted")).toBeInTheDocument()
    expect(screen.getByText("assignments.directory.unassignedCount"))
      .toBeInTheDocument()

    const rows = screen.getAllByTestId("assignment-directory-row")
    expect(rows.map((row) => row.getAttribute("aria-label"))).toEqual([
      "Alice",
      "Bob",
      "Dana",
    ])
    expect(within(rows[2]).getByText("assignments.directory.notSubmittedTag"))
      .toBeInTheDocument()
    expect(within(rows[2]).queryByText("assignments.directory.hours"))
      .not.toBeInTheDocument()
  })

  it("sorts only the submitter section and searches both sections", async () => {
    const user = userEvent.setup()
    renderPanel()

    await user.click(
      screen.getByRole("button", { name: "assignments.directory.sortByName" }),
    )

    expect(
      screen
        .getAllByTestId("assignment-directory-row")
        .map((row) => row.getAttribute("aria-label")),
    ).toEqual(["Alice", "Bob", "Dana"])

    await user.type(
      screen.getByLabelText("assignments.directory.search"),
      "da",
    )

    expect(
      screen
        .getAllByTestId("assignment-directory-row")
        .map((row) => row.getAttribute("aria-label")),
    ).toEqual(["Dana"])
  })
})

function renderPanel() {
  return renderWithProviders(
    <DndContext>
      <AssignmentBoardSidePanel
        slots={slots}
        projectedSlots={slots}
        renderDraftState={emptyDraftState}
        directory={directory}
        disabled={false}
      />
    </DndContext>,
  )
}
