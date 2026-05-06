import { Outlet, createFileRoute, redirect } from "@tanstack/react-router"

import { currentUserQueryOptions } from "@/lib/queries"

export const Route = createFileRoute(
  "/_authenticated/publications/$publicationId/availability",
)({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(currentUserQueryOptions)
    if (!user.is_admin) {
      throw redirect({ to: "/" })
    }
  },
  component: PublicationAvailabilityLayout,
})

export function PublicationAvailabilityLayout() {
  return <Outlet />
}
