import { describe, expect, it } from "vitest"

import {
  createTemplateSchema,
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
