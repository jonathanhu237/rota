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

const { getMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
}))

vi.mock("./axios", () => ({
  default: {
    delete: deleteMock,
    get: getMock,
    patch: patchMock,
    post: postMock,
    put: putMock,
  },
}))

import {
  autoAssignPublication,
  activatePublication,
  createAvailabilitySubmission,
  createAssignment,
  createPublication,
  deleteAvailabilitySubmission,
  deleteAssignment,
  endPublication,
  previewSetupToken,
  requestPasswordReset,
  replaceUserPositions,
  resendInvitation,
  setupPassword,
  updateTemplate,
} from "./queries"

describe("replaceUserPositions", () => {
  beforeEach(() => {
    deleteMock.mockReset()
    getMock.mockReset()
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
    getMock.mockReset()
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
    getMock.mockReset()
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
    getMock.mockReset()
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
    getMock.mockReset()
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
    getMock.mockReset()
    postMock.mockReset()
    patchMock.mockReset()
    putMock.mockReset()
  })

  it("creates an assignment with the user, slot, and position ids", async () => {
    postMock.mockResolvedValue({ data: undefined })

    await createAssignment(7, {
      user_id: 8,
      slot_id: 11,
      position_id: 101,
    })

    expect(postMock).toHaveBeenCalledWith("/publications/7/assignments", {
      user_id: 8,
      slot_id: 11,
      position_id: 101,
    })
  })

  it("deletes an assignment by assignment id", async () => {
    deleteMock.mockResolvedValue({ data: undefined })

    await deleteAssignment(7, 19)

    expect(deleteMock).toHaveBeenCalledWith("/publications/7/assignments/19")
  })
})

describe("auto assign publication", () => {
  beforeEach(() => {
    deleteMock.mockReset()
    getMock.mockReset()
    postMock.mockReset()
    patchMock.mockReset()
    putMock.mockReset()
  })

  it("posts to the auto-assign endpoint", async () => {
    postMock.mockResolvedValue({
      data: {
        publication: {
          id: 7,
          template_id: 2,
          template_name: "Weekday Template",
          name: "May Coverage",
          state: "ASSIGNING",
          submission_start_at: "2026-05-01T09:00:00Z",
          submission_end_at: "2026-05-03T09:00:00Z",
          planned_active_from: "2026-05-04T09:00:00Z",
          activated_at: null,
          ended_at: null,
          created_at: "2026-04-20T08:00:00Z",
          updated_at: "2026-04-20T08:00:00Z",
        },
        shifts: [],
      },
    })

    await autoAssignPublication(7)

    expect(postMock).toHaveBeenCalledWith("/publications/7/auto-assign")
  })
})

describe("password setup queries", () => {
  beforeEach(() => {
    deleteMock.mockReset()
    getMock.mockReset()
    postMock.mockReset()
    patchMock.mockReset()
    putMock.mockReset()
  })

  it("requests a password reset with the email payload", async () => {
    postMock.mockResolvedValue({ data: { message: "ok" } })

    await requestPasswordReset("worker@example.com")

    expect(postMock).toHaveBeenCalledWith("/auth/password-reset-request", {
      email: "worker@example.com",
    })
  })

  it("previews a setup token through query params", async () => {
    getMock.mockResolvedValue({
      data: {
        email: "worker@example.com",
        name: "Worker",
        purpose: "invitation",
      },
    })

    await previewSetupToken("token-123")

    expect(getMock).toHaveBeenCalledWith("/auth/setup-token", {
      params: { token: "token-123" },
    })
  })

  it("submits setup password payload unchanged", async () => {
    postMock.mockResolvedValue({ data: undefined })

    await setupPassword({
      token: "token-123",
      password: "pa55word",
    })

    expect(postMock).toHaveBeenCalledWith("/auth/setup-password", {
      token: "token-123",
      password: "pa55word",
    })
  })

  it("posts resend invitation to the user endpoint", async () => {
    postMock.mockResolvedValue({ data: undefined })

    await resendInvitation(12)

    expect(postMock).toHaveBeenCalledWith("/users/12/resend-invitation")
  })
})
