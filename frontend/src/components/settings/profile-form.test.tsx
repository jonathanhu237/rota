import userEvent from "@testing-library/user-event"
import { waitFor } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"

import type { User } from "@/lib/types"
import { renderWithProviders } from "@/test-utils/render"

import { ProfileForm } from "./profile-form"

const updateOwnProfileMock = vi.hoisted(() => vi.fn())

vi.mock("@/components/settings/settings-api", () => ({
  updateOwnProfileMutation: {
    mutationFn: updateOwnProfileMock,
  },
}))

function makeUser(overrides: Partial<User> = {}): User {
  return {
    id: 1,
    email: "alice@example.com",
    name: "Alice Example",
    is_admin: false,
    status: "active",
    version: 1,
    language_preference: null,
    theme_preference: null,
    ...overrides,
  }
}

describe("ProfileForm", () => {
  beforeEach(() => {
    updateOwnProfileMock.mockReset()
  })

  it("rejects an empty name", async () => {
    const user = userEvent.setup()

    const { findByText, getByLabelText, getByRole } = renderWithProviders(
      <ProfileForm user={makeUser()} />,
    )

    await user.clear(getByLabelText("settings.profile.name"))
    await user.click(getByRole("button", { name: "settings.common.save" }))

    expect(
      await findByText("settings.validation.nameRequired"),
    ).toBeInTheDocument()
    expect(updateOwnProfileMock).not.toHaveBeenCalled()
  })

  it("rejects names longer than 100 characters", async () => {
    const user = userEvent.setup()

    const { findByText, getByLabelText, getByRole } = renderWithProviders(
      <ProfileForm user={makeUser()} />,
    )

    await user.clear(getByLabelText("settings.profile.name"))
    await user.type(getByLabelText("settings.profile.name"), "a".repeat(101))
    await user.click(getByRole("button", { name: "settings.common.save" }))

    expect(await findByText("settings.validation.nameMax")).toBeInTheDocument()
    expect(updateOwnProfileMock).not.toHaveBeenCalled()
  })

  it("submits a valid 50-character name", async () => {
    const user = userEvent.setup()
    const name = "a".repeat(50)
    updateOwnProfileMock.mockResolvedValue(makeUser({ name }))

    const { getByLabelText, getByRole } = renderWithProviders(
      <ProfileForm user={makeUser()} />,
    )

    await user.clear(getByLabelText("settings.profile.name"))
    await user.type(getByLabelText("settings.profile.name"), name)
    await user.click(getByRole("button", { name: "settings.common.save" }))

    await waitFor(() => {
      expect(updateOwnProfileMock).toHaveBeenCalled()
    })
    expect(updateOwnProfileMock.mock.calls[0][0]).toEqual({ name })
  })
})
