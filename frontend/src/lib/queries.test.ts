import { beforeEach, describe, expect, it, vi } from "vitest"
import { QueryClient } from "@tanstack/react-query"

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
  adminAttendanceDayQueryOptions,
  adminAttendanceShiftQueryOptions,
  adminAvailabilityBoardQueryOptions,
  adminAvailabilityDetailQueryOptions,
  adminClearArrival,
  adminCreateOvertime,
  adminDeleteOvertime,
  adminUpdateOvertime,
  adminUpsertArrival,
  autoAssignPublication,
  activatePublication,
  confirmEmailChange,
  createAvailabilitySubmission,
  createAssignment,
  createPublication,
  deleteAvailabilitySubmission,
  deleteAssignment,
  endPublication,
  brandingFallback,
  getBranding,
  leavePoolQueryOptions,
  leavePreviewQueryOptions,
  leaderAttendanceQueryOptions,
  previewSetupToken,
  recordLeaderArrival,
  recordLeaderOvertime,
  requestEmailChange,
  requestPasswordReset,
  replaceAdminAvailability,
  replaceUserPositions,
  resendInvitation,
  setupPassword,
  updateAttendanceSettings,
  updateBranding,
  updatePublication,
  updateTemplate,
} from "./queries"

describe("branding queries", () => {
  beforeEach(() => {
    deleteMock.mockReset()
    getMock.mockReset()
    postMock.mockReset()
    patchMock.mockReset()
    putMock.mockReset()
  })

  it("returns API branding when available", async () => {
    getMock.mockResolvedValue({
      data: {
        product_name: "排班系统",
        organization_name: "Acme",
        version: 2,
        created_at: "2026-05-04T00:00:00Z",
        updated_at: "2026-05-04T00:01:00Z",
      },
    })

    await expect(getBranding()).resolves.toEqual({
      product_name: "排班系统",
      organization_name: "Acme",
      version: 2,
      created_at: "2026-05-04T00:00:00Z",
      updated_at: "2026-05-04T00:01:00Z",
    })
    expect(getMock).toHaveBeenCalledWith("/branding")
  })

  it("falls back to Rota when public branding cannot be loaded", async () => {
    getMock.mockRejectedValue(new Error("network"))

    await expect(getBranding()).resolves.toEqual(brandingFallback)
  })

  it("updates branding through the admin endpoint", async () => {
    putMock.mockResolvedValue({
      data: {
        product_name: "OpsHub",
        organization_name: "",
        version: 3,
        created_at: "",
        updated_at: "",
      },
    })

    await updateBranding({
      product_name: "OpsHub",
      organization_name: "",
      version: 2,
    })

    expect(putMock).toHaveBeenCalledWith("/branding", {
      product_name: "OpsHub",
      organization_name: "",
      version: 2,
    })
  })
})

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

const toIso = (value: string) => new Date(value).toISOString()

describe("createPublication", () => {
  beforeEach(() => {
    deleteMock.mockReset()
    getMock.mockReset()
    postMock.mockReset()
    patchMock.mockReset()
    putMock.mockReset()
  })

  it("posts the publication payload with RFC3339 timestamps", async () => {
    postMock.mockResolvedValue({
      data: {
        publication: {
          id: 9,
          template_id: 2,
          template_name: "Weekday Template",
          name: "May Coverage",
          description: "",
          state: "DRAFT",
          submission_start_at: "2026-05-01T09:00:00Z",
          submission_end_at: "2026-05-03T09:00:00Z",
          planned_active_from: "2026-05-04T09:00:00Z",
          planned_active_until: "2026-06-29T09:00:00Z",
          activated_at: null,
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
      planned_active_until: "2026-06-29T09:00",
    })

    expect(postMock).toHaveBeenCalledWith("/publications", {
      template_id: 2,
      name: "May Coverage",
      submission_start_at: toIso("2026-05-01T09:00"),
      submission_end_at: toIso("2026-05-03T09:00"),
      planned_active_from: toIso("2026-05-04T09:00"),
      planned_active_until: toIso("2026-06-29T09:00"),
    })
  })

  it("patches publication fields with the partial payload", async () => {
    patchMock.mockResolvedValue({
      data: {
        publication: {
          id: 9,
          template_id: 2,
          template_name: "Weekday Template",
          name: "May Coverage",
          description: "Updated",
          state: "ACTIVE",
          submission_start_at: "2026-05-01T09:00:00Z",
          submission_end_at: "2026-05-03T09:00:00Z",
          planned_active_from: "2026-05-04T09:00:00Z",
          planned_active_until: "2026-07-06T09:00:00Z",
          activated_at: "2026-05-04T09:00:00Z",
          created_at: "2026-04-20T08:00:00Z",
          updated_at: "2026-04-21T08:00:00Z",
        },
      },
    })

    await updatePublication(9, {
      planned_active_until: "2026-07-06T09:00:00Z",
    })

    expect(patchMock).toHaveBeenCalledWith("/publications/9", {
      planned_active_until: "2026-07-06T09:00:00Z",
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

  it("creates a submission using the slot and weekday payload", async () => {
    postMock.mockResolvedValue({ data: undefined })

    await createAvailabilitySubmission(7, 21, 2)

    expect(postMock).toHaveBeenCalledWith("/publications/7/submissions", {
      slot_id: 21,
      weekday: 2,
    })
  })

  it("deletes a submission using the slot and weekday path params", async () => {
    deleteMock.mockResolvedValue({ data: undefined })

    await deleteAvailabilitySubmission(7, 21, 2)

    expect(deleteMock).toHaveBeenCalledWith("/publications/7/submissions/21/2")
  })
})

describe("admin availability queries", () => {
  beforeEach(() => {
    deleteMock.mockReset()
    getMock.mockReset()
    postMock.mockReset()
    patchMock.mockReset()
    putMock.mockReset()
  })

  it("requests the paginated board with normalized query params", async () => {
    getMock.mockResolvedValue({
      data: {
        publication: null,
        employees: [],
        pagination: {
          page: 2,
          page_size: 10,
          total: 0,
          total_pages: 0,
        },
      },
    })
    const client = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })

    await client.fetchQuery(
      adminAvailabilityBoardQueryOptions(7, 2, 10, " alice "),
    )

    expect(getMock).toHaveBeenCalledWith(
      "/publications/7/availability-board",
      {
        params: {
          page: 2,
          page_size: 10,
          search: "alice",
        },
      },
    )
  })

  it("omits empty admin availability search terms", async () => {
    getMock.mockResolvedValue({
      data: {
        publication: null,
        employees: [],
        pagination: {
          page: 1,
          page_size: 10,
          total: 0,
          total_pages: 0,
        },
      },
    })
    const client = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })

    await client.fetchQuery(adminAvailabilityBoardQueryOptions(7, 1, 10, " "))

    expect(getMock).toHaveBeenCalledWith(
      "/publications/7/availability-board",
      {
        params: {
          page: 1,
          page_size: 10,
        },
      },
    )
  })

  it("requests a single user's admin availability detail", async () => {
    getMock.mockResolvedValue({
      data: {
        publication: null,
        user: null,
        positions: [],
        slots: [],
        submissions: [],
        cells: [],
      },
    })
    const client = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })

    await client.fetchQuery(adminAvailabilityDetailQueryOptions(7, 12))

    expect(getMock).toHaveBeenCalledWith(
      "/publications/7/availability-submissions/12",
    )
  })

  it("replaces admin availability with a complete submissions set", async () => {
    putMock.mockResolvedValue({
      data: {
        publication: null,
        user: null,
        positions: [],
        slots: [],
        submissions: [{ slot_id: 21, weekday: 3 }],
        cells: [],
      },
    })

    await replaceAdminAvailability(7, 12, [{ slot_id: 21, weekday: 3 }])

    expect(putMock).toHaveBeenCalledWith(
      "/publications/7/availability-submissions/12",
      {
        submissions: [{ slot_id: 21, weekday: 3 }],
      },
    )
  })
})

describe("attendance queries and mutations", () => {
  beforeEach(() => {
    deleteMock.mockReset()
    getMock.mockReset()
    postMock.mockReset()
    patchMock.mockReset()
    putMock.mockReset()
  })

  it("requests current leader attendance", async () => {
    getMock.mockResolvedValue({
      data: {
        publication: null,
        shifts: [],
      },
    })
    const client = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })

    await client.fetchQuery(leaderAttendanceQueryOptions)

    expect(getMock).toHaveBeenCalledWith("/attendance/current")
  })

  it("requests admin attendance day and shift detail with stable keys", async () => {
    getMock
      .mockResolvedValueOnce({
        data: {
          publication: null,
          date: "2026-05-11",
          shifts: [],
        },
      })
      .mockResolvedValueOnce({
        data: {
          shift: {
            publication_id: 7,
            slot_id: 21,
            weekday: 1,
            start_time: "09:00",
            end_time: "12:00",
            occurrence_date: "2026-05-11",
            scheduled_start: "2026-05-11T09:00:00Z",
            scheduled_end: "2026-05-11T12:00:00Z",
            arrival_window_open: true,
            overtime_window_open: true,
            roster: [],
            orphan_arrivals: [],
            overtime_records: [],
          },
        },
      })
    const client = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })

    await client.fetchQuery(adminAttendanceDayQueryOptions(7, "2026-05-11"))
    await client.fetchQuery(
      adminAttendanceShiftQueryOptions(7, 21, "2026-05-11"),
    )

    expect(getMock).toHaveBeenNthCalledWith(
      1,
      "/publications/7/attendance",
      { params: { date: "2026-05-11" } },
    )
    expect(getMock).toHaveBeenNthCalledWith(
      2,
      "/publications/7/attendance/shifts/21/2026-05-11",
    )
  })

  it("writes leader arrivals and overtime", async () => {
    postMock
      .mockResolvedValueOnce({ data: { shift: { slot_id: 21 } } })
      .mockResolvedValueOnce({ data: { overtime: { id: 88 } } })

    await recordLeaderArrival({
      publication_id: 7,
      slot_id: 21,
      assignment_id: 1002,
      occurrence_date: "2026-05-11",
      user_id: 2,
      arrived_at: "2026-05-11T09:00:00Z",
    })
    await recordLeaderOvertime({
      publication_id: 7,
      slot_id: 21,
      occurrence_date: "2026-05-11",
      user_id: 2,
      hours: 1.5,
      note: "cleanup",
    })

    expect(postMock).toHaveBeenNthCalledWith(1, "/attendance/arrivals", {
      publication_id: 7,
      slot_id: 21,
      assignment_id: 1002,
      occurrence_date: "2026-05-11",
      user_id: 2,
      arrived_at: "2026-05-11T09:00:00Z",
    })
    expect(postMock).toHaveBeenNthCalledWith(2, "/attendance/overtime", {
      publication_id: 7,
      slot_id: 21,
      occurrence_date: "2026-05-11",
      user_id: 2,
      hours: 1.5,
      note: "cleanup",
    })
  })

  it("writes admin attendance corrections, overtime, and settings", async () => {
    putMock.mockResolvedValue({ data: { shift: { slot_id: 21 } } })
    postMock.mockResolvedValue({ data: { overtime: { id: 88 } } })
    patchMock
      .mockResolvedValueOnce({ data: { overtime: { id: 88 } } })
      .mockResolvedValueOnce({ data: { publication: { id: 7 } } })
    deleteMock.mockResolvedValue({ data: undefined })

    await adminUpsertArrival(7, {
      slot_id: 21,
      assignment_id: 1002,
      occurrence_date: "2026-05-11",
      user_id: 2,
      arrived_at: "2026-05-11T09:00:00Z",
    })
    await adminClearArrival(7, 55)
    await adminCreateOvertime(7, {
      slot_id: 21,
      occurrence_date: "2026-05-11",
      user_id: 2,
      hours: 1,
      note: "cover",
    })
    await adminUpdateOvertime(7, 88, { hours: 1.5, note: "updated" })
    await adminDeleteOvertime(7, 88)
    await updateAttendanceSettings(7, 12.5)

    expect(putMock).toHaveBeenCalledWith(
      "/publications/7/attendance/arrivals",
      {
        slot_id: 21,
        assignment_id: 1002,
        occurrence_date: "2026-05-11",
        user_id: 2,
        arrived_at: "2026-05-11T09:00:00Z",
      },
    )
    expect(deleteMock).toHaveBeenNthCalledWith(
      1,
      "/publications/7/attendance/arrivals/55",
    )
    expect(postMock).toHaveBeenCalledWith(
      "/publications/7/attendance/overtime",
      {
        publication_id: 7,
        slot_id: 21,
        occurrence_date: "2026-05-11",
        user_id: 2,
        hours: 1,
        note: "cover",
      },
    )
    expect(patchMock).toHaveBeenNthCalledWith(
      1,
      "/publications/7/attendance/overtime/88",
      { hours: 1.5, note: "updated" },
    )
    expect(deleteMock).toHaveBeenNthCalledWith(
      2,
      "/publications/7/attendance/overtime/88",
    )
    expect(patchMock).toHaveBeenNthCalledWith(
      2,
      "/publications/7/attendance/settings",
      { overtime_entry_window_hours: 12.5 },
    )
  })
})

describe("leave previews", () => {
  beforeEach(() => {
    deleteMock.mockReset()
    getMock.mockReset()
    postMock.mockReset()
    patchMock.mockReset()
    putMock.mockReset()
  })

  it("requests preview occurrences using YYYY-MM-DD query params", async () => {
    getMock.mockResolvedValue({ data: { occurrences: [] } })
    const client = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })

    await client.fetchQuery(
      leavePreviewQueryOptions("2026-05-01", "2026-05-15"),
    )

    expect(getMock).toHaveBeenCalledWith("/users/me/leaves/preview", {
      params: {
        from: "2026-05-01",
        to: "2026-05-15",
      },
    })
  })

  it("requests leave pool rows with pagination metadata", async () => {
    getMock.mockResolvedValue({
      data: { leaves: [], page: 2, page_size: 20, total_count: 30 },
    })
    const client = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })

    const result = await client.fetchQuery(
      leavePoolQueryOptions("completed", 2, 20),
    )

    expect(getMock).toHaveBeenCalledWith("/leaves/pool", {
      params: {
        state: "completed",
        page: 2,
        page_size: 20,
      },
    })
    expect(result.total_count).toBe(30)
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

  it("creates an assignment with the user, slot, weekday, and position ids", async () => {
    postMock.mockResolvedValue({ data: undefined })

    await createAssignment(7, {
      user_id: 8,
      slot_id: 11,
      weekday: 3,
      position_id: 101,
    })

    expect(postMock).toHaveBeenCalledWith("/publications/7/assignments", {
      user_id: 8,
      slot_id: 11,
      weekday: 3,
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
          description: "",
          state: "ASSIGNING",
          submission_start_at: "2026-05-01T09:00:00Z",
          submission_end_at: "2026-05-03T09:00:00Z",
          planned_active_from: "2026-05-04T09:00:00Z",
          planned_active_until: "2026-06-29T09:00:00Z",
          activated_at: null,
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

  it("submits email change confirmation payload unchanged", async () => {
    postMock.mockResolvedValue({ data: undefined })

    await confirmEmailChange({
      token: "token-123",
    })

    expect(postMock).toHaveBeenCalledWith("/auth/confirm-email-change", {
      token: "token-123",
    })
  })

  it("requests an email change with the new email and password payload", async () => {
    postMock.mockResolvedValue({ data: undefined })

    await requestEmailChange({
      new_email: "new@example.com",
      current_password: "pa55word",
    })

    expect(postMock).toHaveBeenCalledWith("/users/me/email-change-request", {
      new_email: "new@example.com",
      current_password: "pa55word",
    })
  })

  it("posts resend invitation to the user endpoint", async () => {
    postMock.mockResolvedValue({ data: undefined })

    await resendInvitation(12)

    expect(postMock).toHaveBeenCalledWith("/users/12/resend-invitation")
  })
})
