import { useEffect } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { isAxiosError } from "axios"
import {
  createFileRoute,
  redirect,
  Outlet,
  useNavigate,
} from "@tanstack/react-router"

import { AppSidebar } from "@/components/app-sidebar"
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar"
import { currentUserQueryOptions } from "@/lib/queries"

export const Route = createFileRoute("/_authenticated")({
  beforeLoad: async ({ context }) => {
    try {
      await context.queryClient.ensureQueryData(currentUserQueryOptions)
    } catch (error) {
      if (isAxiosError(error) && error.response?.status === 401) {
        throw redirect({ to: "/login" })
      }
      throw error
    }
  },
  component: AuthenticatedLayout,
})

function AuthenticatedLayout() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { error } = useQuery(currentUserQueryOptions)

  useEffect(() => {
    if (isAxiosError(error) && error.response?.status === 401) {
      queryClient.removeQueries({ queryKey: ["auth"] })
      navigate({ to: "/login", replace: true })
    }
  }, [error, navigate, queryClient])

  return (
    <SidebarProvider>
      <AppSidebar />
      <SidebarInset>
        <header className="sticky top-0 z-10 flex h-14 items-center border-b bg-background px-4 md:hidden">
          <SidebarTrigger aria-label="Open navigation" />
          <div className="ml-3 text-sm font-semibold">Rota</div>
        </header>
        <main className="flex-1 p-6">
          <Outlet />
        </main>
      </SidebarInset>
    </SidebarProvider>
  )
}
