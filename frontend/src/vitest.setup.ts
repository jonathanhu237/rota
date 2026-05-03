import { cleanup } from "@testing-library/react"
import { vi } from "vitest"
import { afterEach } from "vitest"

import "@testing-library/jest-dom/vitest"

afterEach(() => {
  cleanup()
})

vi.mock("react-i18next", async () => {
  const actual = await vi.importActual<typeof import("react-i18next")>(
    "react-i18next",
  )

  return {
    ...actual,
    useTranslation: () => ({
      t: (key: string, options?: Record<string, unknown>) => {
        if (typeof options?.productName === "string") {
          return `${key} ${options.productName}`
        }
        return key
      },
      i18n: {
        language: "en",
        resolvedLanguage: "en",
        changeLanguage: vi.fn(),
      },
    }),
  }
})
