import { describe, expect, it } from "vitest"

import type { AssignmentBoardShift } from "@/lib/types"

import { isAssignmentBoardShiftUnderstaffed } from "./assignment-board-state"

function makeShift(
  overrides: Partial<AssignmentBoardShift> = {},
): AssignmentBoardShift {
  return {
    shift: {
      id: 11,
      weekday: 1,
      start_time: "09:00",
      end_time: "12:00",
      position_id: 101,
      position_name: "Front Desk",
      required_headcount: 2,
    },
    candidates: [],
    assignments: [],
    ...overrides,
  }
}

describe("isAssignmentBoardShiftUnderstaffed", () => {
  it("returns true when assignments are below required headcount", () => {
    expect(
      isAssignmentBoardShiftUnderstaffed(
        makeShift({
          assignments: [{ assignment_id: 1, user_id: 7, name: "Alice", email: "alice@example.com" }],
        }),
      ),
    ).toBe(true)
  })

  it("returns false when assignments meet or exceed required headcount", () => {
    expect(
      isAssignmentBoardShiftUnderstaffed(
        makeShift({
          assignments: [
            { assignment_id: 1, user_id: 7, name: "Alice", email: "alice@example.com" },
            { assignment_id: 2, user_id: 8, name: "Bob", email: "bob@example.com" },
          ],
        }),
      ),
    ).toBe(false)
  })
})
