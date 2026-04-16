import { describe, expect, it } from "vitest"

import {
  getPublicationLifecycleAction,
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

describe("getPublicationLifecycleAction", () => {
  it.each([
    ["DRAFT", null],
    ["COLLECTING", null],
    ["ASSIGNING", "activate"],
    ["ACTIVE", "end"],
    ["ENDED", null],
  ] satisfies [PublicationState, string | null][])(
    "maps %s to %s",
    (state, expectedAction) => {
      expect(getPublicationLifecycleAction(state)).toBe(expectedAction)
    },
  )
})
