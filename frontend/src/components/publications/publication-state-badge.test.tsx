import { describe, expect, it } from "vitest"

import { renderWithProviders } from "@/test-utils/render"

import { PublicationStateBadge } from "./publication-state-badge"

describe("PublicationStateBadge", () => {
  it.each([
    ["DRAFT", "publications.state.draft"],
    ["COLLECTING", "publications.state.collecting"],
    ["ASSIGNING", "publications.state.assigning"],
    ["PUBLISHED", "publications.state.published"],
    ["ACTIVE", "publications.state.active"],
    ["ENDED", "publications.state.ended"],
  ] as const)("renders %s with %s", (state, translationKey) => {
    const { getByText } = renderWithProviders(
      <PublicationStateBadge state={state} />,
    )

    expect(getByText(translationKey)).toBeInTheDocument()
  })
})
