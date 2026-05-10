import type { QueryClient } from "@tanstack/react-query"
import { QueryClientProvider } from "@tanstack/react-query"
import type { ErrorComponentProps } from "@tanstack/react-router"
import { createRootRouteWithContext, Outlet } from "@tanstack/react-router"
import { LogIn, RefreshCw } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button, buttonVariants } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { ToastProvider } from "@/components/ui/toast"
import { TooltipProvider } from "@/components/ui/tooltip"

type RouterContext = {
  queryClient: QueryClient
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RouteComponent,
  errorComponent: RootErrorComponent,
})

function RouteComponent() {
  const { queryClient } = Route.useRouteContext()

  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <TooltipProvider>
          <Outlet />
        </TooltipProvider>
      </ToastProvider>
    </QueryClientProvider>
  )
}

type RootErrorComponentProps = ErrorComponentProps & {
  onRetry?: () => void
}

export function RootErrorComponent({ onRetry }: RootErrorComponentProps) {
  const { t } = useTranslation()

  const retry = () => {
    if (onRetry) {
      onRetry()
      return
    }

    window.location.reload()
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-6 text-foreground">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle>{t("rootError.title")}</CardTitle>
          <CardDescription>{t("rootError.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex flex-col gap-3 sm:flex-row">
            <Button type="button" onClick={retry}>
              <RefreshCw data-icon="inline-start" />
              {t("rootError.retry")}
            </Button>
            <a
              className={buttonVariants({ variant: "outline" })}
              href="/login"
            >
              <LogIn data-icon="inline-start" />
              {t("rootError.login")}
            </a>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
