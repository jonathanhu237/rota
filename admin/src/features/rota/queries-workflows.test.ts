import { QueryClient } from "@tanstack/react-query"
import { http, HttpResponse } from "msw"
import { describe, expect, it } from "vitest"

import { server } from "@/test/msw"
import { RotaApiError } from "./api"
import {
  adminAttendanceDayQueryOptions,
  adminAvailabilityBoardQueryOptions,
  adminAvailabilityDetailQueryOptions,
  createAssignment,
  createAvailabilitySubmission,
  createLeave,
  createShiftChangeRequest,
  leaderAttendanceQueryOptions,
  leavePoolQueryOptions,
  leavePreviewQueryOptions,
  myLeavesQueryOptions,
  publicationAssignmentBoardQueryOptions,
  publicationRosterQueryOptions,
  recordLeaderArrival,
  rosterCurrentQueryOptions,
} from "./queries"

describe("Rota workflow API contracts", () => {
  it("loads assignment, availability, roster, attendance, and leave resources", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const publication = { id: 7, state: "PUBLISHED", name: "Week 1" }

    server.use(
      http.get("/api/rota/publications/7/assignment-board", () =>
        HttpResponse.json({ publication, slots: [], employees: [] }),
      ),
      http.get("/api/rota/publications/7/availability-board", ({ request }) => {
        const url = new URL(request.url)
        expect(url.searchParams.get("page")).toBe("2")
        expect(url.searchParams.get("page_size")).toBe("25")
        expect(url.searchParams.get("search")).toBe("Alice")
        return HttpResponse.json({ publication, employees: [], pagination: {} })
      }),
      http.get("/api/rota/publications/7/availability-submissions/user-1", () =>
        HttpResponse.json({ publication, user: {}, positions: [], slots: [], submissions: [], cells: [] }),
      ),
      http.get("/api/rota/roster/current", () =>
        HttpResponse.json({ publication, week_start: "2026-09-14", weekdays: [] }),
      ),
      http.get("/api/rota/publications/7/roster", ({ request }) => {
        expect(new URL(request.url).searchParams.get("week")).toBe("2026-09-14")
        return HttpResponse.json({ publication, week_start: "2026-09-14", weekdays: [] })
      }),
      http.get("/api/rota/attendance/current", () =>
        HttpResponse.json({ publication, shifts: [] }),
      ),
      http.get("/api/rota/publications/7/attendance", ({ request }) => {
        expect(new URL(request.url).searchParams.get("date")).toBe("2026-09-14")
        return HttpResponse.json({ publication, date: "2026-09-14", shifts: [] })
      }),
      http.get("/api/rota/users/me/leaves", ({ request }) => {
        const url = new URL(request.url)
        expect(url.searchParams.get("page")).toBe("1")
        expect(url.searchParams.get("page_size")).toBe("20")
        return HttpResponse.json({ leaves: [] })
      }),
      http.get("/api/rota/leaves/pool", ({ request }) => {
        const url = new URL(request.url)
        expect(url.searchParams.get("state")).toBe("pending")
        return HttpResponse.json({ leaves: [], pagination: {} })
      }),
      http.get("/api/rota/users/me/leaves/preview", ({ request }) => {
        const url = new URL(request.url)
        expect(url.searchParams.get("from")).toBe("2026-09-14")
        expect(url.searchParams.get("to")).toBe("2026-09-21")
        return HttpResponse.json({ occurrences: [] })
      }),
    )

    await expect(
      queryClient.fetchQuery(publicationAssignmentBoardQueryOptions(7)),
    ).resolves.toMatchObject({ publication })
    await expect(
      queryClient.fetchQuery(adminAvailabilityBoardQueryOptions(7, 2, 25, " Alice ")),
    ).resolves.toMatchObject({ publication })
    await expect(
      queryClient.fetchQuery(adminAvailabilityDetailQueryOptions(7, "user-1")),
    ).resolves.toMatchObject({ publication })
    await expect(queryClient.fetchQuery(rosterCurrentQueryOptions)).resolves.toMatchObject({
      week_start: "2026-09-14",
    })
    await expect(
      queryClient.fetchQuery(publicationRosterQueryOptions(7, "2026-09-14")),
    ).resolves.toMatchObject({ publication })
    await expect(queryClient.fetchQuery(leaderAttendanceQueryOptions)).resolves.toMatchObject({
      publication,
    })
    await expect(
      queryClient.fetchQuery(adminAttendanceDayQueryOptions(7, "2026-09-14")),
    ).resolves.toMatchObject({ date: "2026-09-14" })
    await expect(queryClient.fetchQuery(myLeavesQueryOptions(1, 20))).resolves.toEqual([])
    await expect(
      queryClient.fetchQuery(leavePoolQueryOptions("pending", 1, 20)),
    ).resolves.toMatchObject({ leaves: [] })
    await expect(
      queryClient.fetchQuery(leavePreviewQueryOptions("2026-09-14", "2026-09-21")),
    ).resolves.toEqual([])
  })

  it("sends assignment, availability, attendance, leave, and shift-change writes to their canonical routes", async () => {
    const requests: Array<{ method: string; path: string; body: unknown }> = []
    server.use(
      http.post("/api/rota/publications/7/assignments", async ({ request }) => {
        requests.push({ method: request.method, path: new URL(request.url).pathname, body: await request.json() })
        return new HttpResponse(null, { status: 204 })
      }),
      http.post("/api/rota/publications/7/submissions", async ({ request }) => {
        requests.push({ method: request.method, path: new URL(request.url).pathname, body: await request.json() })
        return new HttpResponse(null, { status: 204 })
      }),
      http.post("/api/rota/attendance/arrivals", async ({ request }) => {
        requests.push({ method: request.method, path: new URL(request.url).pathname, body: await request.json() })
        return HttpResponse.json({ shift: { slot_id: 11 } })
      }),
      http.post("/api/rota/leaves", async ({ request }) => {
        requests.push({ method: request.method, path: new URL(request.url).pathname, body: await request.json() })
        return HttpResponse.json({ leave: { id: 31 } })
      }),
      http.post("/api/rota/publications/7/shift-changes", async ({ request }) => {
        requests.push({ method: request.method, path: new URL(request.url).pathname, body: await request.json() })
        return HttpResponse.json({ request: { id: 41 } })
      }),
    )

    await createAssignment(7, {
      user_id: "019535d9-3df7-79fb-b466-fa907fa17f90",
      slot_id: 11,
      weekday: 1,
      position_id: 3,
    })
    await createAvailabilitySubmission(7, 11, 1)
    await recordLeaderArrival({
      publication_id: 7,
      slot_id: 11,
      assignment_id: 21,
      occurrence_date: "2026-09-14",
      user_id: "019535d9-3df7-79fb-b466-fa907fa17f90",
    })
    await createLeave({
      assignment_id: 21,
      occurrence_date: "2026-09-14",
      type: "give_pool",
      category: "sick",
    })
    await createShiftChangeRequest(7, {
      type: "give_pool",
      requester_assignment_id: 21,
      occurrence_date: "2026-09-14",
    })

    expect(requests).toEqual([
      {
        method: "POST",
        path: "/api/rota/publications/7/assignments",
        body: {
          user_id: "019535d9-3df7-79fb-b466-fa907fa17f90",
          slot_id: 11,
          weekday: 1,
          position_id: 3,
        },
      },
      {
        method: "POST",
        path: "/api/rota/publications/7/submissions",
        body: { slot_id: 11, weekday: 1 },
      },
      {
        method: "POST",
        path: "/api/rota/attendance/arrivals",
        body: {
          publication_id: 7,
          slot_id: 11,
          assignment_id: 21,
          occurrence_date: "2026-09-14",
          user_id: "019535d9-3df7-79fb-b466-fa907fa17f90",
        },
      },
      {
        method: "POST",
        path: "/api/rota/leaves",
        body: {
          assignment_id: 21,
          occurrence_date: "2026-09-14",
          type: "give_pool",
          category: "sick",
        },
      },
      {
        method: "POST",
        path: "/api/rota/publications/7/shift-changes",
        body: {
          type: "give_pool",
          requester_assignment_id: 21,
          occurrence_date: "2026-09-14",
        },
      },
    ])
  })

  it("preserves a structured rejection from the Rota API", async () => {
    server.use(
      http.post("/api/rota/publications/7/assignments", () =>
        HttpResponse.json(
          { error: { code: "ASSIGNMENT_CONFLICT", message: "overlap" } },
          { status: 409 },
        ),
      ),
    )

    await expect(
      createAssignment(7, {
        user_id: "019535d9-3df7-79fb-b466-fa907fa17f90",
        slot_id: 11,
        weekday: 1,
        position_id: 3,
      }),
    ).rejects.toMatchObject({ status: 409, code: "ASSIGNMENT_CONFLICT" } satisfies Partial<RotaApiError>)
  })
})
