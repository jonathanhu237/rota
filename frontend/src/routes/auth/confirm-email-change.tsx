import { createFileRoute } from "@tanstack/react-router"

import { ConfirmEmailChangePage } from "@/components/auth/confirm-email-change-page"

export const Route = createFileRoute("/auth/confirm-email-change")({
  validateSearch: (search: Record<string, unknown>) => ({
    token: typeof search.token === "string" ? search.token : "",
  }),
  component: ConfirmEmailChangeRoute,
})

function ConfirmEmailChangeRoute() {
  const { token } = Route.useSearch()

  return <ConfirmEmailChangePage token={token} />
}
