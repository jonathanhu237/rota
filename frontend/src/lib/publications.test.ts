import { beforeEach, describe, expect, it, vi } from "vitest"

const { getMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
}))

vi.mock("./axios", () => ({
  default: {
    get: getMock,
  },
}))

import {
  fetchPublicationScheduleXLSX,
  formatScheduleExportFilename,
  getPublicationLifecycleAction,
  getPublicationStateTranslationKey,
  normalizeScheduleExportLanguage,
  type PublicationState,
} from "./publications"

describe("getPublicationStateTranslationKey", () => {
  it.each([
    ["DRAFT", "publications.state.draft"],
    ["COLLECTING", "publications.state.collecting"],
    ["ASSIGNING", "publications.state.assigning"],
    ["PUBLISHED", "publications.state.published"],
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
    ["ASSIGNING", "publish"],
    ["PUBLISHED", "activate"],
    ["ACTIVE", "end"],
    ["ENDED", null],
  ] satisfies [PublicationState, string | null][])(
    "maps %s to %s",
    (state, expectedAction) => {
      expect(getPublicationLifecycleAction(state)).toBe(expectedAction)
    },
  )
})

describe("fetchPublicationScheduleXLSX", () => {
  beforeEach(() => {
    getMock.mockReset()
  })

  it("requests the publication schedule workbook as a localized blob", async () => {
    const blob = new Blob(["xlsx"])
    getMock.mockResolvedValue({ data: blob })

    await expect(fetchPublicationScheduleXLSX(7, "en")).resolves.toBe(blob)

    expect(getMock).toHaveBeenCalledWith(
      "/publications/7/schedule.xlsx",
      {
        params: {
          lang: "en",
        },
        responseType: "blob",
      },
    )
  })
})

describe("formatScheduleExportFilename", () => {
  it("uses the localized roster label and client-local timestamp", () => {
    expect(
      formatScheduleExportFilename(
        "Spring/Rota",
        "en",
        new Date(2026, 4, 4, 15, 30),
      ),
    ).toBe("Spring-Rota-roster-20260504-1530.xlsx")
  })

  it("uses the zh roster label", () => {
    expect(
      formatScheduleExportFilename(
        "春季排班",
        "zh",
        new Date(2026, 4, 4, 9, 5),
      ),
    ).toBe("春季排班-排班表-20260504-0905.xlsx")
  })

  it("replaces filename-unsafe characters with dashes", () => {
    expect(
      formatScheduleExportFilename(
        'A:B/C*D?E"F<G>H|I\\J',
        "en",
        new Date(2026, 4, 4, 15, 30),
      ),
    ).toBe("A-B-C-D-E-F-G-H-I-J-roster-20260504-1530.xlsx")
  })
})

describe("normalizeScheduleExportLanguage", () => {
  it.each([
    ["zh-CN", "zh"],
    ["zh", "zh"],
    ["en-US", "en"],
    [undefined, "en"],
  ] satisfies [string | undefined, string][])(
    "normalizes %s to %s",
    (language, expected) => {
      expect(normalizeScheduleExportLanguage(language)).toBe(expected)
    },
  )
})
