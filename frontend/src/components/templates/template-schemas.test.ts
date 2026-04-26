import { describe, expect, it } from "vitest"

import { createTemplateSchema } from "./template-schemas"

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
