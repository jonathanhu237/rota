import { describe, expect, it } from "vitest"

import type { AssignmentBoardEmployee } from "@/lib/types"

import {
  computeDirectoryStats,
  deriveEmployeeDirectory,
} from "./assignment-board-directory"

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

describe("computeDirectoryStats", () => {
  it("returns zeros for an empty input", () => {
    expect(computeDirectoryStats([])).toEqual({
      total: 0,
      min: 0,
      avg: 0,
      max: 0,
      stddev: 0,
      zeroCount: 0,
    })
  })

  it("computes min, avg, max, stddev, zeroCount across non-empty hours", () => {
    const stats = computeDirectoryStats([0, 0, 2, 4, 4])

    expect(stats.total).toBe(5)
    expect(stats.min).toBe(0)
    expect(stats.max).toBe(4)
    expect(stats.avg).toBeCloseTo(2)
    // population stddev of [0,0,2,4,4] is sqrt(((4+4+0+4+4)/5)) = sqrt(3.2)
    expect(stats.stddev).toBeCloseTo(Math.sqrt(3.2))
    expect(stats.zeroCount).toBe(2)
  })

  it("reports zero stddev when all hours are equal", () => {
    const stats = computeDirectoryStats([3, 3, 3, 3])

    expect(stats.min).toBe(3)
    expect(stats.max).toBe(3)
    expect(stats.avg).toBe(3)
    expect(stats.stddev).toBe(0)
    expect(stats.zeroCount).toBe(0)
  })
})
