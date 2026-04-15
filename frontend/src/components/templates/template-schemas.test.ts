import { describe, expect, it } from "vitest"

import {
  createTemplateSchema,
  createTemplateShiftSchema,
} from "./template-schemas"

const t = (key: string) => key

describe("createTemplateSchema", () => {
  it("trims the template name", () => {
    const schema = createTemplateSchema(t)

    const result = schema.parse({
      name: " Weekday Template ",
      description: " Core shifts ",
    })

    expect(result).toEqual({
      name: "Weekday Template",
      description: "Core shifts",
    })
  })
})

describe("createTemplateShiftSchema", () => {
  it("accepts valid shift input", () => {
    const schema = createTemplateShiftSchema(t)

    const result = schema.parse({
      weekday: 2,
      start_time: "09:00",
      end_time: "12:00",
      position_id: 4,
      required_headcount: 3,
    })

    expect(result).toEqual({
      weekday: 2,
      start_time: "09:00",
      end_time: "12:00",
      position_id: 4,
      required_headcount: 3,
    })
  })

  it("rejects weekday outside Monday-Sunday range", () => {
    const schema = createTemplateShiftSchema(t)

    const result = schema.safeParse({
      weekday: 8,
      start_time: "09:00",
      end_time: "12:00",
      position_id: 4,
      required_headcount: 3,
    })

    expect(result.success).toBe(false)
  })

  it("rejects invalid time ranges", () => {
    const schema = createTemplateShiftSchema(t)

    const result = schema.safeParse({
      weekday: 2,
      start_time: "12:00",
      end_time: "12:00",
      position_id: 4,
      required_headcount: 3,
    })

    expect(result.success).toBe(false)
  })

  it("rejects missing position selections", () => {
    const schema = createTemplateShiftSchema(t)

    const result = schema.safeParse({
      weekday: 2,
      start_time: "09:00",
      end_time: "12:00",
      position_id: 0,
      required_headcount: 3,
    })

    expect(result.success).toBe(false)
  })

  it("rejects non-positive headcounts", () => {
    const schema = createTemplateShiftSchema(t)

    const result = schema.safeParse({
      weekday: 2,
      start_time: "09:00",
      end_time: "12:00",
      position_id: 4,
      required_headcount: 0,
    })

    expect(result.success).toBe(false)
  })
})
