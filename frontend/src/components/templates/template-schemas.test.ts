import { describe, expect, it } from "vitest"

import {
  createTemplateSchema,
  createTemplateSlotPositionSchema,
  createTemplateSlotSchema,
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

describe("createTemplateSlotSchema", () => {
  it("deduplicates and sorts selected weekdays", () => {
    const schema = createTemplateSlotSchema(t)

    const result = schema.parse({
      weekdays: [5, 1, 5],
      start_time: "09:00",
      end_time: "10:00",
    })

    expect(result.weekdays).toEqual([1, 5])
  })

  it("rejects an empty weekday set", () => {
    const schema = createTemplateSlotSchema(t)

    const result = schema.safeParse({
      weekdays: [],
      start_time: "09:00",
      end_time: "10:00",
    })

    expect(result.success).toBe(false)
  })
})

describe("createTemplateSlotPositionSchema", () => {
  it("allows a single-headcount attendance responsible position", () => {
    const schema = createTemplateSlotPositionSchema(t)

    const result = schema.parse({
      position_id: 7,
      required_headcount: 1,
      attendance_responsible: true,
    })

    expect(result).toEqual({
      position_id: 7,
      required_headcount: 1,
      attendance_responsible: true,
    })
  })

  it("rejects attendance responsible positions with headcount above one", () => {
    const schema = createTemplateSlotPositionSchema(t)

    const result = schema.safeParse({
      position_id: 7,
      required_headcount: 2,
      attendance_responsible: true,
    })

    expect(result.success).toBe(false)
  })
})
