import { beforeEach, describe, expect, it, vi } from "vitest"

const { putMock } = vi.hoisted(() => ({
  putMock: vi.fn(),
}))

const { patchMock } = vi.hoisted(() => ({
  patchMock: vi.fn(),
}))

const { postMock } = vi.hoisted(() => ({
  postMock: vi.fn(),
}))

const { deleteMock } = vi.hoisted(() => ({
  deleteMock: vi.fn(),
}))

vi.mock("./axios", () => ({
  default: {
    delete: deleteMock,
    patch: patchMock,
    post: postMock,
    put: putMock,
  },
}))

import {
  activatePublication,
  createAvailabilitySubmission,
  createAssignment,
  createPublication,
  deleteAvailabilitySubmission,
  deleteAssignment,
  endPublication,
  replaceUserPositions,
  updateTemplate,
} from "./queries"

describe("replaceUserPositions", () => {
  beforeEach(() => {
    deleteMock.mockReset()
    postMock.mockReset()
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
    deleteMock.mockReset()
    postMock.mockReset()
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

describe("createPublication", () => {
  beforeEach(() => {
    deleteMock.mockReset()
    postMock.mockReset()
    patchMock.mockReset()
    putMock.mockReset()
  })

  it("posts the publication payload unchanged", async () => {
    postMock.mockResolvedValue({
      data: {
        publication: {
          id: 9,
          template_id: 2,
          template_name: "Weekday Template",
          name: "May Coverage",
          state: "DRAFT",
          submission_start_at: "2026-05-01T09:00:00Z",
          submission_end_at: "2026-05-03T09:00:00Z",
          planned_active_from: "2026-05-04T09:00:00Z",
          activated_at: null,
          ended_at: null,
          created_at: "2026-04-20T08:00:00Z",
          updated_at: "2026-04-20T08:00:00Z",
        },
      },
    })

    await createPublication({
      template_id: 2,
      name: "May Coverage",
      submission_start_at: "2026-05-01T09:00",
      submission_end_at: "2026-05-03T09:00",
      planned_active_from: "2026-05-04T09:00",
    })

    expect(postMock).toHaveBeenCalledWith("/publications", {
      template_id: 2,
      name: "May Coverage",
      submission_start_at: "2026-05-01T09:00",
      submission_end_at: "2026-05-03T09:00",
      planned_active_from: "2026-05-04T09:00",
    })
  })
})

describe("availability submissions", () => {
  beforeEach(() => {
    deleteMock.mockReset()
    postMock.mockReset()
    patchMock.mockReset()
    putMock.mockReset()
  })

  it("creates a submission using the shift id payload", async () => {
    postMock.mockResolvedValue({ data: undefined })

    await createAvailabilitySubmission(7, 11)

    expect(postMock).toHaveBeenCalledWith("/publications/7/submissions", {
      template_shift_id: 11,
    })
  })

  it("deletes a submission using the shift id path param", async () => {
    deleteMock.mockResolvedValue({ data: undefined })

    await deleteAvailabilitySubmission(7, 11)

    expect(deleteMock).toHaveBeenCalledWith("/publications/7/submissions/11")
  })
})

describe("publication lifecycle actions", () => {
  beforeEach(() => {
    deleteMock.mockReset()
    postMock.mockReset()
    patchMock.mockReset()
    putMock.mockReset()
  })

  it("activates a publication with the activate endpoint", async () => {
    postMock.mockResolvedValue({ data: undefined })

    await activatePublication(7)

    expect(postMock).toHaveBeenCalledWith("/publications/7/activate")
  })

  it("ends a publication with the end endpoint", async () => {
    postMock.mockResolvedValue({ data: undefined })

    await endPublication(7)

    expect(postMock).toHaveBeenCalledWith("/publications/7/end")
  })
})

describe("assignment mutations", () => {
  beforeEach(() => {
    deleteMock.mockReset()
    postMock.mockReset()
    patchMock.mockReset()
    putMock.mockReset()
  })

  it("creates an assignment with the user and template shift ids", async () => {
    postMock.mockResolvedValue({ data: undefined })

    await createAssignment(7, {
      user_id: 8,
      template_shift_id: 11,
    })

    expect(postMock).toHaveBeenCalledWith("/publications/7/assignments", {
      user_id: 8,
      template_shift_id: 11,
    })
  })

  it("deletes an assignment by assignment id", async () => {
    deleteMock.mockResolvedValue({ data: undefined })

    await deleteAssignment(7, 19)

    expect(deleteMock).toHaveBeenCalledWith("/publications/7/assignments/19")
  })
})
