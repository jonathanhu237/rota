import { describe, expect, it } from "vitest"

import type {
  AssignmentBoardPosition,
  AssignmentBoardSlot,
} from "@/lib/types"

import {
  getVisibleNonCandidateQualified,
  isAssignmentBoardMutable,
  isAssignmentBoardPositionUnderstaffed,
} from "./assignment-board-state"

function makeSlot(): AssignmentBoardSlot {
  return {
    slot: {
      id: 11,
      weekday: 1,
      start_time: "09:00",
      end_time: "12:00",
    },
    positions: [],
  }
}

function makePosition(
  overrides: Partial<AssignmentBoardPosition> = {},
): AssignmentBoardPosition {
  return {
    position: {
      id: 101,
      name: "Front Desk",
    },
    required_headcount: 2,
    candidates: [],
    non_candidate_qualified: [],
    assignments: [],
    ...overrides,
  }
}

describe("isAssignmentBoardPositionUnderstaffed", () => {
  it("returns true when assignments are below required headcount", () => {
    expect(
      isAssignmentBoardPositionUnderstaffed(
        makePosition({
          assignments: [{ assignment_id: 1, user_id: 7, name: "Alice", email: "alice@example.com" }],
        }),
      ),
    ).toBe(true)
  })

  it("returns false when assignments meet or exceed required headcount", () => {
    expect(
      isAssignmentBoardPositionUnderstaffed(
        makePosition({
          assignments: [
            { assignment_id: 1, user_id: 7, name: "Alice", email: "alice@example.com" },
            { assignment_id: 2, user_id: 8, name: "Bob", email: "bob@example.com" },
          ],
        }),
      ),
    ).toBe(false)
  })

  it("filters out users who already appear as candidates or assignments", () => {
    expect(
      getVisibleNonCandidateQualified(
        makeSlot(),
        makePosition({
          candidates: [
            { user_id: 7, name: "Alice", email: "alice@example.com" },
          ],
          non_candidate_qualified: [
            { user_id: 7, name: "Alice", email: "alice@example.com" },
            { user_id: 8, name: "Bob", email: "bob@example.com" },
            { user_id: 9, name: "Cara", email: "cara@example.com" },
          ],
          assignments: [
            {
              assignment_id: 1,
              user_id: 9,
              name: "Cara",
              email: "cara@example.com",
            },
          ],
        }),
        true,
      ),
    ).toEqual([{ user_id: 8, name: "Bob", email: "bob@example.com" }])
  })

  it("returns an empty list when the qualified toggle is off", () => {
    expect(
      getVisibleNonCandidateQualified(
        makeSlot(),
        makePosition({
          non_candidate_qualified: [
            { user_id: 8, name: "Bob", email: "bob@example.com" },
          ],
        }),
        false,
      ),
    ).toEqual([])
  })
})

describe("isAssignmentBoardMutable", () => {
  it("returns true for assigning, published, and active publications", () => {
    expect(isAssignmentBoardMutable("ASSIGNING")).toBe(true)
    expect(isAssignmentBoardMutable("PUBLISHED")).toBe(true)
    expect(isAssignmentBoardMutable("ACTIVE")).toBe(true)
  })

  it("returns false outside the mutable window", () => {
    expect(isAssignmentBoardMutable("DRAFT")).toBe(false)
    expect(isAssignmentBoardMutable("COLLECTING")).toBe(false)
    expect(isAssignmentBoardMutable("ENDED")).toBe(false)
  })
})
