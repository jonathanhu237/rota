import { describe, expect, it } from "vitest"

import {
  getPublicationStateTranslationKey,
  type PublicationState,
} from "./publications"

describe("getPublicationStateTranslationKey", () => {
  it.each([
    ["DRAFT", "publications.state.draft"],
    ["COLLECTING", "publications.state.collecting"],
    ["ASSIGNING", "publications.state.assigning"],
    ["ACTIVE", "publications.state.active"],
    ["ENDED", "publications.state.ended"],
  ] satisfies [PublicationState, string][])(
    "maps %s to %s",
    (state, expectedKey) => {
      expect(getPublicationStateTranslationKey(state)).toBe(expectedKey)
    },
  )
})
