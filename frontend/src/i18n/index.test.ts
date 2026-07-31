import { afterEach, describe, expect, it } from "vitest"

import i18n, {
  applyLanguagePreference,
  normalizeLanguage,
} from "./index"

const originalLanguage = document.documentElement.lang

afterEach(async () => {
  await applyLanguagePreference("en")
  document.documentElement.lang = originalLanguage
})

describe("application language metadata", () => {
  it("sets the initial document language from i18n", () => {
    expect(document.documentElement.lang).toBe(
      normalizeLanguage(i18n.resolvedLanguage),
    )
  })

  it("updates the document language after a language change", async () => {
    await applyLanguagePreference("zh")
    expect(document.documentElement.lang).toBe("zh")

    await applyLanguagePreference("en")
    expect(document.documentElement.lang).toBe("en")
  })
})
