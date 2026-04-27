import { useQuery } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { buttonVariants } from "@/components/ui/button"
import { getTranslatedApiError } from "@/lib/api-error"
import {
  confirmEmailChange as confirmEmailChangeQuery,
  type ConfirmEmailChangeInput,
} from "@/lib/queries"

type ConfirmEmailChangePageProps = {
  token?: string
  confirmEmailChange?: (input: ConfirmEmailChangeInput) => Promise<unknown>
}

export function ConfirmEmailChangePage({
  token,
  confirmEmailChange = confirmEmailChangeQuery,
}: ConfirmEmailChangePageProps) {
  const { t } = useTranslation()
  const confirmQuery = useQuery({
    queryKey: ["auth", "confirm-email-change", token],
    queryFn: async () => {
      await confirmEmailChange({ token: token ?? "" })
      return true
    },
    enabled: Boolean(token),
    retry: false,
    staleTime: Number.POSITIVE_INFINITY,
  })

  const errorMessage = !token
    ? t("confirmEmailChange.errors.INVALID_LINK")
    : confirmQuery.error
      ? getTranslatedApiError(
          t,
          confirmQuery.error,
          "confirmEmailChange.errors",
          "confirmEmailChange.unexpectedError",
        )
      : null

  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>
            {confirmQuery.isSuccess
              ? t("confirmEmailChange.successTitle")
              : t("confirmEmailChange.title")}
          </CardTitle>
          <CardDescription>
            {confirmQuery.isSuccess
              ? t("confirmEmailChange.successDescription")
              : t("confirmEmailChange.description")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {!token || confirmQuery.isError ? (
            <div className="grid gap-4">
              <p className="text-sm text-destructive">{errorMessage}</p>
              <a className={buttonVariants()} href="/login">
                {t("confirmEmailChange.backToLogin")}
              </a>
            </div>
          ) : confirmQuery.isLoading ? (
            <p className="text-sm text-muted-foreground">
              {t("confirmEmailChange.loading")}
            </p>
          ) : (
            <div className="grid gap-4">
              <p className="text-sm text-muted-foreground">
                {t("confirmEmailChange.successBody")}
              </p>
              <a className={buttonVariants()} href="/login">
                {t("confirmEmailChange.backToLogin")}
              </a>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
