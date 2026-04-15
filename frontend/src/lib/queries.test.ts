import { beforeEach, describe, expect, it, vi } from "vitest"

const { putMock } = vi.hoisted(() => ({
  putMock: vi.fn(),
}))

const { patchMock } = vi.hoisted(() => ({
  patchMock: vi.fn(),
}))

vi.mock("./axios", () => ({
  default: {
    patch: patchMock,
    put: putMock,
  },
}))

import { replaceUserPositions, updateTemplate } from "./queries"

describe("replaceUserPositions", () => {
  beforeEach(() => {
    patchMock.mockReset()
    putMock.mockReset()
  })

  it("sends a wrapped position_ids request body", async () => {
    putMock.mockResolvedValue({ data: undefined })

    await replaceUserPositions(7, [1, 2, 3])

    expect(putMock).toHaveBeenCalledWith("/users/7/positions", {
      position_ids: [1, 2, 3],
    })
  })
})

describe("updateTemplate", () => {
  beforeEach(() => {
    patchMock.mockReset()
    putMock.mockReset()
  })

  it("uses PUT with the full replacement payload", async () => {
    putMock.mockResolvedValue({
      data: {
        template: {
          id: 5,
          name: "Weekday Template",
          description: "Updated description",
        },
      },
    })

    await updateTemplate(5, {
      name: "Weekday Template",
      description: "Updated description",
    })

    expect(putMock).toHaveBeenCalledWith("/templates/5", {
      name: "Weekday Template",
      description: "Updated description",
    })
    expect(patchMock).not.toHaveBeenCalled()
  })
})
