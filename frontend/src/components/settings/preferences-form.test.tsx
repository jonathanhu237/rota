import userEvent from "@testing-library/user-event"
import { waitFor } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"

import type { User } from "@/lib/types"
import { renderWithProviders } from "@/test-utils/render"

import { PreferencesForm } from "./preferences-form"

const updateOwnProfileMock = vi.hoisted(() => vi.fn())
const setThemePreferenceMock = vi.hoisted(() => vi.fn())
const applyLanguagePreferenceMock = vi.hoisted(() => vi.fn())

vi.mock("@/components/settings/settings-api", () => ({
  updateOwnProfileMutation: {
    mutationFn: updateOwnProfileMock,
  },
}))

vi.mock("@/components/theme-context", () => ({
  useTheme: () => ({
    setThemePreference: setThemePreferenceMock,
  }),
}))

vi.mock("@/i18n", () => ({
  normalizeLanguage: (language?: string | null) =>
    language?.toLowerCase().startsWith("zh") ? "zh" : "en",
  applyLanguagePreference: applyLanguagePreferenceMock,
}))

function makeUser(overrides: Partial<User> = {}): User {
  return {
    id: 1,
    email: "alice@example.com",
    name: "Alice Example",
    is_admin: false,
    status: "active",
    version: 1,
    language_preference: "en",
    theme_preference: "system",
    ...overrides,
  }
}

describe("PreferencesForm", () => {
  beforeEach(() => {
    updateOwnProfileMock.mockReset()
    setThemePreferenceMock.mockReset()
    applyLanguagePreferenceMock.mockReset()
  })

  it("renders language and theme enum values", () => {
    const { getByRole } = renderWithProviders(
      <PreferencesForm user={makeUser()} />,
    )

    expect(
      getByRole("radio", { name: "settings.preferences.languageZh" }),
    ).toBeInTheDocument()
    expect(
      getByRole("radio", { name: "settings.preferences.languageEn" }),
    ).toBeInTheDocument()
    expect(
      getByRole("radio", { name: "settings.preferences.themeSystem" }),
    ).toBeInTheDocument()
    expect(
      getByRole("radio", { name: "settings.preferences.themeLight" }),
    ).toBeInTheDocument()
    expect(
      getByRole("radio", { name: "settings.preferences.themeDark" }),
    ).toBeInTheDocument()
  })

  it("saves preferences and applies local language and theme state", async () => {
    const user = userEvent.setup()
    updateOwnProfileMock.mockResolvedValue(
      makeUser({
        language_preference: "zh",
        theme_preference: "dark",
      }),
    )

    const { getByRole } = renderWithProviders(
      <PreferencesForm user={makeUser()} />,
    )

    await user.click(
      getByRole("radio", { name: "settings.preferences.languageZh" }),
    )
    await user.click(
      getByRole("radio", { name: "settings.preferences.themeDark" }),
    )
    await user.click(getByRole("button", { name: "settings.common.save" }))

    await waitFor(() => {
      expect(updateOwnProfileMock).toHaveBeenCalled()
    })
    expect(updateOwnProfileMock.mock.calls[0][0]).toEqual({
      language_preference: "zh",
      theme_preference: "dark",
    })
    await waitFor(() => {
      expect(applyLanguagePreferenceMock).toHaveBeenCalledWith("zh")
      expect(setThemePreferenceMock).toHaveBeenCalledWith("dark")
    })
  })
})
