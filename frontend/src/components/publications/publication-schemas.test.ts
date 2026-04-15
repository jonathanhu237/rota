import { describe, expect, it } from "vitest"

import { createPublicationSchema } from "./publication-schemas"

const t = (key: string) => key

describe("createPublicationSchema", () => {
  it("accepts valid publication input and trims the name", () => {
    const schema = createPublicationSchema(t)

    const result = schema.parse({
      template_id: 3,
      name: " Weekday Coverage ",
      submission_start_at: "2026-04-20T09:00",
      submission_end_at: "2026-04-22T18:00",
      planned_active_from: "2026-04-22T18:00",
    })

    expect(result).toEqual({
      template_id: 3,
      name: "Weekday Coverage",
      submission_start_at: "2026-04-20T09:00",
      submission_end_at: "2026-04-22T18:00",
      planned_active_from: "2026-04-22T18:00",
    })
  })

  it("rejects missing template selections", () => {
    const schema = createPublicationSchema(t)

    const result = schema.safeParse({
      template_id: 0,
      name: "Weekday Coverage",
      submission_start_at: "2026-04-20T09:00",
      submission_end_at: "2026-04-22T18:00",
      planned_active_from: "2026-04-22T18:00",
    })

    expect(result.success).toBe(false)
  })

  it("rejects invalid publication windows", () => {
    const schema = createPublicationSchema(t)

    const result = schema.safeParse({
      template_id: 3,
      name: "Weekday Coverage",
      submission_start_at: "2026-04-22T18:00",
      submission_end_at: "2026-04-22T17:00",
      planned_active_from: "2026-04-22T19:00",
    })

    expect(result.success).toBe(false)
  })
})
