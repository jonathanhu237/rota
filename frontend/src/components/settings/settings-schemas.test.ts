import { describe, expect, it } from "vitest"

import { createBrandingSchema } from "./settings-schemas"

const t = (key: string) => key

describe("createBrandingSchema", () => {
  it("counts Unicode code points for branding length limits", () => {
    const schema = createBrandingSchema(t)
    const productName = "😀".repeat(60)
    const organizationName = "🧭".repeat(100)

    const result = schema.parse({
      product_name: ` ${productName} `,
      organization_name: ` ${organizationName} `,
      version: 1,
    })

    expect(result).toEqual({
      product_name: productName,
      organization_name: organizationName,
      version: 1,
    })
  })

  it("rejects product and organization names over the code point limits", () => {
    const schema = createBrandingSchema(t)

    expect(
      schema.safeParse({
        product_name: "😀".repeat(61),
        organization_name: "",
        version: 1,
      }).success,
    ).toBe(false)

    expect(
      schema.safeParse({
        product_name: "Rota",
        organization_name: "🧭".repeat(101),
        version: 1,
      }).success,
    ).toBe(false)
  })
})
