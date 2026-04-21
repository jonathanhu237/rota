import { describe, expect, it } from "vitest"

import {
  createGiveDirectSchema,
  createSwapSchema,
} from "./shift-change-schemas"

const t = (key: string) => key

describe("createSwapSchema", () => {
  it("accepts valid counterpart selections", () => {
    const schema = createSwapSchema(t)

    const result = schema.parse({
      counterpart_user_id: 7,
      counterpart_assignment_id: 42,
    })

    expect(result).toEqual({
      counterpart_user_id: 7,
      counterpart_assignment_id: 42,
    })
  })

  it("rejects a missing counterpart user", () => {
    const schema = createSwapSchema(t)

    const result = schema.safeParse({
      counterpart_user_id: 0,
      counterpart_assignment_id: 42,
    })

    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0]?.path).toEqual(["counterpart_user_id"])
    }
  })

  it("rejects a missing counterpart shift", () => {
    const schema = createSwapSchema(t)

    const result = schema.safeParse({
      counterpart_user_id: 7,
      counterpart_assignment_id: 0,
    })

    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0]?.path).toEqual(["counterpart_assignment_id"])
    }
  })
})

describe("createGiveDirectSchema", () => {
  it("accepts a positive counterpart user id", () => {
    const schema = createGiveDirectSchema(t)

    const result = schema.parse({ counterpart_user_id: 9 })

    expect(result).toEqual({ counterpart_user_id: 9 })
  })

  it("rejects an unset counterpart user id", () => {
    const schema = createGiveDirectSchema(t)

    const result = schema.safeParse({ counterpart_user_id: 0 })

    expect(result.success).toBe(false)
  })
})
