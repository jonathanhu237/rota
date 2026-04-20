import { createFileRoute, useNavigate } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import { SetupPasswordPage } from "@/components/auth/setup-password-page"
import { useToast } from "@/components/ui/toast"

export const Route = createFileRoute("/setup-password")({
  validateSearch: (search: Record<string, unknown>) => ({
    token: typeof search.token === "string" ? search.token : "",
  }),
  component: SetupPasswordRoute,
})

function SetupPasswordRoute() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { toast } = useToast()
  const { token } = Route.useSearch()

  return (
    <SetupPasswordPage
      token={token}
      onSuccess={() => {
        toast({
          variant: "default",
          description: t("setupPassword.success"),
        })
        navigate({ to: "/login", replace: true })
      }}
    />
  )
}
