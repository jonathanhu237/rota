import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ToastProvider } from "@/components/ui/toast"
import type {
  AttendanceShift,
  LeaderAttendance,
  Publication,
} from "@/lib/types"

const { getMock, postMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  postMock: vi.fn(),
}))

vi.mock("@/lib/axios", () => ({
  default: {
    get: getMock,
    post: postMock,
  },
}))

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    )
  return {
    ...actual,
    createFileRoute: () => (options: object) => options,
  }
})

import { AttendancePage } from "./attendance"

describe("AttendancePage", () => {
  beforeEach(() => {
    getMock.mockReset()
    postMock.mockReset()
    getMock.mockResolvedValue({ data: { publication: null, shifts: [] } })
  })

  it("renders an empty state for non-leaders or unavailable shifts", () => {
    renderAttendancePage({ publication: makePublication(), shifts: [] })

    expect(screen.getByText("attendance.title")).toBeInTheDocument()
    expect(screen.getByText("attendance.empty")).toBeInTheDocument()
  })

  it("records arrival with the scheduled-start default and keeps recorded arrivals locked", async () => {
    const user = userEvent.setup()
    postMock.mockResolvedValue({ data: { shift: makeShift() } })
    renderAttendancePage({ publication: makePublication(), shifts: [makeShift()] })

    expect(screen.getByText("Leader")).toBeInTheDocument()
    expect(screen.getByText("Worker")).toBeInTheDocument()
    expect(screen.getAllByText("attendance.status.present")).toHaveLength(1)
    const scheduledStart = toDateTimeLocalForTest("2026-05-11T09:00:00Z")
    const arrivalInput = screen.getByDisplayValue(scheduledStart)
    expect(arrivalInput).toHaveAttribute("min", scheduledStart)
    expect(arrivalInput).toHaveAttribute("max")

    await user.click(screen.getByRole("button", { name: "attendance.recordArrival" }))

    await waitFor(() => {
      expect(postMock).toHaveBeenCalledWith("/attendance/arrivals", {
        publication_id: 7,
        slot_id: 21,
        assignment_id: 1002,
        occurrence_date: "2026-05-11",
        user_id: 2,
        arrived_at: "2026-05-11T09:00:00.000Z",
      })
    })
  })
})

function renderAttendancePage(data: LeaderAttendance) {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  })
  client.setQueryData(["attendance", "current"], data)

  return render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <AttendancePage />
      </ToastProvider>
    </QueryClientProvider>,
  )
}

function makePublication(): Publication {
  return {
    id: 7,
    template_id: 3,
    template_name: "Main Template",
    name: "May Roster",
    description: "",
    state: "ACTIVE",
    submission_start_at: "2026-04-20T00:00:00Z",
    submission_end_at: "2026-04-21T00:00:00Z",
    planned_active_from: "2026-05-01T00:00:00Z",
    planned_active_until: "2026-06-01T00:00:00Z",
    overtime_entry_window_hours: 24,
    activated_at: "2026-05-01T00:00:00Z",
    created_at: "2026-04-19T00:00:00Z",
    updated_at: "2026-04-19T00:00:00Z",
  }
}

function makeShift(): AttendanceShift {
  return {
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
    roster: [
      {
        assignment_id: 1001,
        position_id: 41,
        position_name: "负责人",
        attendance_responsible: true,
        user_id: 1,
        user_name: "Leader",
        user_email: "leader@example.com",
        status: "present",
        record: {
          id: 55,
          publication_id: 7,
          assignment_id: 1001,
          occurrence_date: "2026-05-11",
          user_id: 1,
          user_name: "Leader",
          user_email: "leader@example.com",
          arrived_at: "2026-05-11T09:00:00Z",
          recorded_by_user_id: 1,
          recorded_at: "2026-05-11T09:00:00Z",
          updated_by_user_id: 1,
          updated_at: "2026-05-11T09:00:00Z",
        },
      },
      {
        assignment_id: 1002,
        position_id: 42,
        position_name: "Front Desk",
        attendance_responsible: false,
        user_id: 2,
        user_name: "Worker",
        user_email: "worker@example.com",
        status: "pending",
        record: null,
      },
    ],
    orphan_arrivals: [],
    overtime_records: [],
  }
}

function toDateTimeLocalForTest(value: string) {
  const date = new Date(value)
  const offset = date.getTimezoneOffset()
  const local = new Date(date.getTime() - offset * 60_000)
  return local.toISOString().slice(0, 16)
}
