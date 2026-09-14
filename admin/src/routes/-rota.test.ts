import { QueryClient } from "@tanstack/react-query"
import { describe, expect, it, vi } from "vitest"
import { createAppRouter } from "@/app/router"
import type { ApiClient } from "@/shared/api/client"

function routeApi(): ApiClient {
  return {
    getSetupStatus: vi.fn(),
    setup: vi.fn(),
    login: vi.fn(),
    me: vi.fn(),
    logout: vi.fn(),
    requestPasswordReset: vi.fn(),
    completePasswordReset: vi.fn(),
  }
}

describe("Rota routes", () => {
  it("registers employee and management workflows under the Rota boundary", () => {
    const router = createAppRouter({
      api: routeApi(),
      queryClient: new QueryClient(),
    })

    const expectedRoutes = [
      "/rota",
      "/rota/availability",
      "/rota/attendance",
      "/rota/requests",
      "/rota/roster",
      "/rota/leaves",
      "/rota/leaves/new",
      "/rota/leaves/$leaveId",
      "/rota/publications",
      "/rota/publications/$publicationId",
      "/rota/publications/$publicationId/availability",
      "/rota/publications/$publicationId/availability/$userId",
      "/rota/publications/$publicationId/assignments",
      "/rota/publications/$publicationId/attendance",
      "/rota/publications/$publicationId/shift-changes",
      "/rota/positions",
      "/rota/templates",
      "/rota/templates/$templateId",
    ] as const

    for (const path of expectedRoutes) {
      const route = router.routesByPath[path]
      expect(route, `missing route ${path}`).toBeDefined()
      expect(route.id).toContain("/_authenticated/rota")
      expect(route.options.component).toBeTypeOf("function")
    }
  })

  it("keeps the canonical notification destinations in the authenticated tree", () => {
    const router = createAppRouter({
      api: routeApi(),
      queryClient: new QueryClient(),
    })

    expect(router.routesByPath["/rota/requests"].fullPath).toBe("/rota/requests")
    expect(router.routesByPath["/rota/leaves/$leaveId"].fullPath).toBe(
      "/rota/leaves/$leaveId",
    )
  })
})
