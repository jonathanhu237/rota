import { QueryClient } from "@tanstack/react-query"
import { screen } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"

import { renderWithProviders } from "@/test-utils/render"
import type { User } from "@/lib/types"

import { SettingsPage } from "./settings"

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    )

  return {
    ...actual,
    createFileRoute: () => (config: unknown) => config,
  }
})

vi.mock("@/components/settings/branding-form", () => ({
  BrandingForm: () => <div>branding-form</div>,
}))

vi.mock("@/components/settings/email-form", () => ({
  EmailForm: () => <div>email-form</div>,
}))

vi.mock("@/components/settings/password-form", () => ({
  PasswordForm: () => <div>password-form</div>,
}))

vi.mock("@/components/settings/preferences-form", () => ({
  PreferencesForm: () => <div>preferences-form</div>,
}))

vi.mock("@/components/settings/profile-form", () => ({
  ProfileForm: () => <div>profile-form</div>,
}))

function renderSettingsPage(user: User) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  })
  queryClient.setQueryData(["auth", "me"], user)

  return renderWithProviders(<SettingsPage />, { queryClient })
}

function makeUser(overrides: Partial<User> = {}): User {
  return {
    id: 1,
    email: "worker@example.com",
    name: "Worker",
    is_admin: false,
    status: "active",
    version: 1,
    language_preference: null,
    theme_preference: "system",
    ...overrides,
  }
}

describe("SettingsPage", () => {
  it("shows branding settings to admins", () => {
    renderSettingsPage(makeUser({ is_admin: true }))

    expect(screen.getByText("settings.branding.title")).toBeInTheDocument()
    expect(screen.getByText("branding-form")).toBeInTheDocument()
  })

  it("hides branding settings from non-admin users", () => {
    renderSettingsPage(makeUser({ is_admin: false }))

    expect(screen.queryByText("settings.branding.title")).not.toBeInTheDocument()
    expect(screen.queryByText("branding-form")).not.toBeInTheDocument()
  })
})
