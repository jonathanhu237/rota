import { useEffect } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { isAxiosError } from "axios"
import {
  createFileRoute,
  redirect,
  Outlet,
  useNavigate,
} from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import { AppBreadcrumbs } from "@/components/app-breadcrumbs"
import { AppSidebar } from "@/components/app-sidebar"
import { ThemeProvider } from "@/components/theme-provider"
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar"
import { Separator } from "@/components/ui/separator"
import { applyLanguagePreference } from "@/i18n"
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

export function AuthenticatedLayout() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { data: user, error } = useQuery(currentUserQueryOptions)

  useEffect(() => {
    if (isAxiosError(error) && error.response?.status === 401) {
      queryClient.removeQueries({ queryKey: ["auth"] })
      navigate({ to: "/login", replace: true })
    }
  }, [error, navigate, queryClient])

  useEffect(() => {
    if (user?.language_preference) {
      void applyLanguagePreference(user.language_preference)
    }
  }, [user?.language_preference])

  return (
    <ThemeProvider>
      <SidebarProvider>
        <AppSidebar />
        <SidebarInset>
          <header className="flex h-16 shrink-0 items-center gap-2 px-4">
            <SidebarTrigger
              aria-label={t("sidebar.toggleNavigation")}
              className="-ml-1"
            />
            <Separator
              orientation="vertical"
              className="mr-2 data-vertical:h-4 data-vertical:self-auto"
            />
            <AppBreadcrumbs />
          </header>
          <main className="flex-1 p-6">
            <Outlet />
          </main>
        </SidebarInset>
      </SidebarProvider>
    </ThemeProvider>
  )
}
