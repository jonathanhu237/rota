import { describe, expect, it } from "vitest"

import { cn } from "./utils"

describe("cn", () => {
  it("merges class names correctly", () => {
    expect(cn("px-2 py-1", "px-4")).toBe("py-1 px-4")
  })

  it("handles conditional classes", () => {
    const shouldHide = false

    expect(
      cn(
        "text-sm",
        shouldHide ? "hidden" : undefined,
        ["font-medium", undefined],
        {
          underline: true,
          italic: false,
        },
      ),
    ).toBe("text-sm font-medium underline")
  })
})
