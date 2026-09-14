import { Outlet, createFileRoute, redirect } from "@tanstack/react-router"

import { currentUserQueryOptions, toRotaUser } from "@/features/rota/queries"

export const Route = createFileRoute(
  "/_authenticated/rota/publications/$publicationId/availability",
)({
  beforeLoad: async ({ context }) => {
    const user = toRotaUser(await context.queryClient.ensureQueryData(currentUserQueryOptions(context.api)))
    if (!user.is_admin && !user.can_manage) {
      throw redirect({ to: "/" })
    }
  },
  component: PublicationAvailabilityLayout,
})

export function PublicationAvailabilityLayout() {
  return <Outlet />
}
