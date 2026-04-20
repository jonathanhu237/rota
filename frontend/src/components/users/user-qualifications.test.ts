import { describe, expect, it } from "vitest"

import {
  hasQualificationSelectionChanged,
  normalizeQualificationPositionIDs,
  shouldInitializeQualificationSelection,
} from "./user-qualification-helpers"

describe("normalizeQualificationPositionIDs", () => {
  it("dedupes and sorts selected position IDs", () => {
    expect(normalizeQualificationPositionIDs([4, 2, 4, 1, 2])).toEqual([
      1, 2, 4,
    ])
  })
})

describe("hasQualificationSelectionChanged", () => {
  it("treats equivalent sets as unchanged regardless of order or duplicates", () => {
    expect(hasQualificationSelectionChanged([3, 1], [1, 3, 3])).toBe(false)
  })

  it("detects when the selected set changed", () => {
    expect(hasQualificationSelectionChanged([1, 2], [1, 4])).toBe(true)
  })
})

describe("shouldInitializeQualificationSelection", () => {
  it("initializes when the dialog opens for a user", () => {
    expect(
      shouldInitializeQualificationSelection({
        open: true,
        wasOpen: false,
        userID: 3,
        initializedUserID: null,
      }),
    ).toBe(true)
  })

  it("does not reinitialize on a background refetch for the same open user", () => {
    expect(
      shouldInitializeQualificationSelection({
        open: true,
        wasOpen: true,
        userID: 3,
        initializedUserID: 3,
      }),
    ).toBe(false)
  })

  it("reinitializes when switching to a different user while open", () => {
    expect(
      shouldInitializeQualificationSelection({
        open: true,
        wasOpen: true,
        userID: 4,
        initializedUserID: 3,
      }),
    ).toBe(true)
  })
})
