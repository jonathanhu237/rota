import { describe, expect, it } from "vitest"

import type { AssignmentBoardSlot } from "@/lib/types"

import {
  applyDraftToBoard,
  computeUserHours,
  discardDrafts,
  emptyDraftState,
  enqueueAdd,
  enqueueMove,
  enqueueReplace,
  enqueueSwap,
  getBoardCellKey,
} from "./draft-state"

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
        position: {
          id: 101,
          name: "Front Desk",
        },
        required_headcount: 1,
        candidates: [
          { user_id: 10, name: "Alice", email: "alice@example.com" },
        ],
        non_candidate_qualified: [],
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
  {
    slot: {
      id: 2,
      weekday: 1,
      start_time: "12:00",
      end_time: "15:30",
    },
    positions: [
      {
        position: {
          id: 101,
          name: "Front Desk",
        },
        required_headcount: 2,
        candidates: [
          { user_id: 10, name: "Alice", email: "alice@example.com" },
        ],
        non_candidate_qualified: [],
        assignments: [
          {
            assignment_id: 21,
            user_id: 12,
            name: "Cara",
            email: "cara@example.com",
          },
        ],
      },
      {
        position: {
          id: 102,
          name: "Kitchen",
        },
        required_headcount: 1,
        candidates: [],
        non_candidate_qualified: [],
        assignments: [
          {
            assignment_id: 22,
            user_id: 13,
            name: "Dana",
            email: "dana@example.com",
          },
        ],
      },
    ],
  },
]

describe("draft state reducers", () => {
  it("enqueues a MOVE as unassign then assign", () => {
    const state = enqueueMove(
      emptyDraftState,
      {
        assignmentID: 20,
        userID: 11,
        name: "Bob",
        email: "bob@example.com",
        slotID: 1,
        positionID: 101,
      },
      {
        slotID: 2,
        positionID: 101,
      },
    )

    expect(state.ops.map((op) => op.kind)).toEqual(["unassign", "assign"])
    expect(state.ops[1]).toMatchObject({
      kind: "assign",
      userID: 11,
      slotID: 2,
      positionID: 101,
      isUnqualified: false,
    })
  })

  it("enqueues a SWAP as two unassigns followed by two assigns", () => {
    const state = enqueueSwap(
      emptyDraftState,
      {
        assignmentID: 20,
        userID: 11,
        name: "Bob",
        email: "bob@example.com",
        slotID: 1,
        positionID: 101,
      },
      {
        assignmentID: 22,
        userID: 13,
        name: "Dana",
        email: "dana@example.com",
        slotID: 2,
        positionID: 102,
      },
      {
        slotID: 2,
        positionID: 102,
        isUnqualified: true,
      },
      {
        slotID: 1,
        positionID: 101,
      },
    )

    expect(state.ops.map((op) => op.kind)).toEqual([
      "unassign",
      "unassign",
      "assign",
      "assign",
    ])
    expect(state.ops[2]).toMatchObject({
      kind: "assign",
      userID: 11,
      slotID: 2,
      positionID: 102,
      isUnqualified: true,
    })
  })

  it("enqueues a REPLACE as unassign outgoing then assign incoming", () => {
    const state = enqueueReplace(
      emptyDraftState,
      {
        assignmentID: 20,
        userID: 11,
        name: "Bob",
        email: "bob@example.com",
        slotID: 1,
        positionID: 101,
      },
      {
        userID: 10,
        name: "Alice",
        email: "alice@example.com",
      },
      {
        slotID: 1,
        positionID: 101,
      },
    )

    expect(state.ops.map((op) => op.kind)).toEqual(["unassign", "assign"])
    expect(state.ops[1]).toMatchObject({
      kind: "assign",
      userID: 10,
      slotID: 1,
      positionID: 101,
    })
  })

  it("enqueues an ADD as one assign", () => {
    const state = enqueueAdd(
      emptyDraftState,
      {
        userID: 10,
        name: "Alice",
        email: "alice@example.com",
      },
      {
        slotID: 2,
        positionID: 101,
      },
    )

    expect(state.ops).toHaveLength(1)
    expect(state.ops[0]).toMatchObject({
      kind: "assign",
      userID: 10,
      slotID: 2,
      positionID: 101,
    })
  })
})

describe("draft projection", () => {
  it("applies queued move, swap, replace, and add operations to a snapshot", () => {
    let state = enqueueMove(
      emptyDraftState,
      {
        assignmentID: 20,
        userID: 11,
        name: "Bob",
        email: "bob@example.com",
        slotID: 1,
        positionID: 101,
      },
      {
        slotID: 2,
        positionID: 101,
      },
    )
    state = enqueueReplace(
      state,
      {
        assignmentID: 22,
        userID: 13,
        name: "Dana",
        email: "dana@example.com",
        slotID: 2,
        positionID: 102,
      },
      {
        userID: 10,
        name: "Alice",
        email: "alice@example.com",
      },
      {
        slotID: 2,
        positionID: 102,
      },
    )

    const projected = applyDraftToBoard(slots, state)

    expect(projected.get(getBoardCellKey(1, 101))).toEqual([])
    expect(projected.get(getBoardCellKey(2, 101))?.map((item) => item.name)).toEqual([
      "Cara",
      "Bob",
    ])
    expect(projected.get(getBoardCellKey(2, 102))?.map((item) => item.name)).toEqual([
      "Alice",
    ])
  })

  it("computes hours with and without drafts", () => {
    expect(computeUserHours(slots, emptyDraftState, 11)).toBe(2)

    const state = enqueueMove(
      emptyDraftState,
      {
        assignmentID: 20,
        userID: 11,
        name: "Bob",
        email: "bob@example.com",
        slotID: 1,
        positionID: 101,
      },
      {
        slotID: 2,
        positionID: 101,
      },
    )

    expect(computeUserHours(slots, state, 11)).toBe(3.5)
  })

  it("discards drafts", () => {
    const state = enqueueAdd(
      emptyDraftState,
      {
        userID: 10,
        name: "Alice",
        email: "alice@example.com",
      },
      {
        slotID: 2,
        positionID: 101,
      },
    )

    expect(state.ops).toHaveLength(1)
    expect(discardDrafts()).toEqual({ ops: [] })
  })
})
