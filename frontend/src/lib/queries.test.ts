import { beforeEach, describe, expect, it, vi } from "vitest"

const { putMock } = vi.hoisted(() => ({
  putMock: vi.fn(),
}))

vi.mock("./axios", () => ({
  default: {
    put: putMock,
  },
}))

import { replaceUserPositions } from "./queries"

describe("replaceUserPositions", () => {
  beforeEach(() => {
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
