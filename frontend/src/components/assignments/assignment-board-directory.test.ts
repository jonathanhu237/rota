import { describe, expect, it } from "vitest"

import type { AssignmentBoardEmployee } from "@/lib/types"

import { deriveEmployeeDirectory } from "./assignment-board-directory"

const employees: AssignmentBoardEmployee[] = [
  {
    user_id: 10,
    name: "Alice",
    email: "alice@example.com",
    position_ids: [101, 102],
  },
  {
    user_id: 11,
    name: "Bob",
    email: "bob@example.com",
    position_ids: [101],
  },
]

describe("deriveEmployeeDirectory", () => {
  it("indexes API employees by user id", () => {
    const directory = deriveEmployeeDirectory(employees)

    expect([...directory.keys()]).toEqual([10, 11])
    expect(directory.get(10)).toMatchObject({
      user_id: 10,
      name: "Alice",
      email: "alice@example.com",
    })
  })

  it("converts position_ids to sets", () => {
    const directory = deriveEmployeeDirectory(employees)

    expect(directory.get(10)?.position_ids).toEqual(new Set([101, 102]))
    expect(directory.get(11)?.position_ids).toEqual(new Set([101]))
  })
})
