import { describe, expect, it } from "vitest"

import {
  createForgotPasswordSchema,
  createSetupPasswordSchema,
} from "./auth-schemas"

const t = (key: string) => key

describe("createForgotPasswordSchema", () => {
  it("requires a valid email address", () => {
    const schema = createForgotPasswordSchema(t)

    expect(schema.safeParse({ email: "" }).success).toBe(false)
    expect(schema.safeParse({ email: "worker@example.com" }).success).toBe(true)
  })
})

describe("createSetupPasswordSchema", () => {
  it("accepts matching passwords with sufficient length", () => {
    const schema = createSetupPasswordSchema(t)

    expect(
      schema.safeParse({
        password: "pa55word",
        confirmPassword: "pa55word",
      }).success,
    ).toBe(true)
  })

  it("rejects short passwords", () => {
    const schema = createSetupPasswordSchema(t)

    expect(
      schema.safeParse({
        password: "short",
        confirmPassword: "short",
      }).success,
    ).toBe(false)
  })

  it("rejects mismatched passwords", () => {
    const schema = createSetupPasswordSchema(t)

    expect(
      schema.safeParse({
        password: "pa55word",
        confirmPassword: "different",
      }).success,
    ).toBe(false)
  })
})
