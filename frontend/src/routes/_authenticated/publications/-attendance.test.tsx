import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, screen } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ToastProvider } from "@/components/ui/toast"
import type {
  AdminAttendanceDay,
  AttendanceShift,
  Publication,
} from "@/lib/types"

const { getMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
}))

vi.mock("@/lib/axios", () => ({
  default: {
    delete: vi.fn(),
    get: getMock,
    patch: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
  },
}))

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    )
  return {
    ...actual,
    createFileRoute: () => (options: object) => ({
      ...options,
      useParams: () => ({ publicationId: "7" }),
    }),
  }
})

import { AdminAttendancePage } from "./$publicationId/attendance"

describe("AdminAttendancePage", () => {
  beforeEach(() => {
    getMock.mockReset()
    getMock.mockResolvedValue({
      data: {
        publication: makePublication(),
        date: todayKey(),
        shifts: [],
      },
    })
  })

  it("renders shift detail, correction controls, orphan arrivals, overtime controls, and settings", () => {
    renderAdminAttendancePage()

    expect(screen.getByText("attendance.adminTitle")).toBeInTheDocument()
    expect(screen.getAllByText("Worker").length).toBeGreaterThan(0)
    expect(screen.getByText(/Former Worker/)).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "attendance.setArrival" }),
    ).toBeInTheDocument()
    expect(
      screen.getAllByRole("button", { name: "attendance.clearArrival" })[0],
    ).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "attendance.addOvertime" }),
    ).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "attendance.updateOvertime" }),
    ).toBeInTheDocument()
    expect(
      screen.getByRole("button", { name: "attendance.saveSettings" }),
    ).toBeInTheDocument()
    expect(screen.getByDisplayValue("24")).toBeInTheDocument()
  })
})

function renderAdminAttendancePage() {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: Infinity,
      },
    },
  })
  const date = todayKey()
  client.setQueryData(["publications", 7, "attendance", date], {
    publication: makePublication(),
    date,
    shifts: [
      {
        slot_id: 21,
        weekday: 1,
        occurrence_date: date,
        scheduled_start: `${date}T09:00:00Z`,
        scheduled_end: `${date}T12:00:00Z`,
        roster_count: 1,
        pending_count: 1,
        present_count: 0,
        late_count: 0,
        absent_count: 0,
        orphan_count: 1,
        overtime_count: 1,
      },
    ],
  } satisfies AdminAttendanceDay)
  client.setQueryData(
    ["publications", 7, "attendance", "shift", 21, date],
    makeShift(date),
  )

  return render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <AdminAttendancePage />
      </ToastProvider>
    </QueryClientProvider>,
  )
}

function todayKey() {
  return new Date().toISOString().slice(0, 10)
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

function makeShift(date: string): AttendanceShift {
  return {
    publication_id: 7,
    slot_id: 21,
    weekday: 1,
    start_time: "09:00",
    end_time: "12:00",
    occurrence_date: date,
    scheduled_start: `${date}T09:00:00Z`,
    scheduled_end: `${date}T12:00:00Z`,
    arrival_window_open: false,
    overtime_window_open: false,
    roster: [
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
    orphan_arrivals: [
      {
        id: 55,
        publication_id: 7,
        assignment_id: 1002,
        occurrence_date: date,
        user_id: 3,
        user_name: "Former Worker",
        user_email: "former@example.com",
        arrived_at: `${date}T09:00:00Z`,
        recorded_by_user_id: 1,
        recorded_at: `${date}T09:00:00Z`,
        updated_by_user_id: 1,
        updated_at: `${date}T09:00:00Z`,
        status: "present",
      },
    ],
    overtime_records: [
      {
        id: 88,
        publication_id: 7,
        slot_id: 21,
        weekday: 1,
        occurrence_date: date,
        user_id: 2,
        user_name: "Worker",
        user_email: "worker@example.com",
        hours: 1.5,
        note: "cleanup",
        recorded_by_user_id: 1,
        recorded_at: `${date}T12:00:00Z`,
        updated_by_user_id: 1,
        updated_at: `${date}T12:00:00Z`,
      },
    ],
  }
}
