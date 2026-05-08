import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Link, createFileRoute } from "@tanstack/react-router"
import { Check, HandHelping, Plus, X } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button, buttonVariants } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useToast } from "@/components/ui/toast"
import { getTranslatedApiError } from "@/lib/api-error"
import {
  approveShiftChangeRequest,
  cancelLeave,
  leavePoolQueryOptions,
  rejectShiftChangeRequest,
} from "@/lib/queries"
import type { Leave, LeavePoolState, LeaveState } from "@/lib/types"
import { cn } from "@/lib/utils"

export const Route = createFileRoute("/_authenticated/leaves/")({
  component: LeavesWorkbenchPage,
})

const pageSize = 20
const stateFilters: LeavePoolState[] = [
  "pending",
  "all",
  "completed",
  "cancelled",
  "failed",
]

const stateVariant: Record<
  LeaveState,
  "default" | "secondary" | "outline" | "destructive"
> = {
  pending: "outline",
  completed: "default",
  failed: "destructive",
  cancelled: "secondary",
}

type LeaveAction = "claim" | "approve" | "reject" | "cancel"

export function LeavesWorkbenchPage() {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const { toast } = useToast()
  const [state, setState] = useState<LeavePoolState>("pending")
  const [page, setPage] = useState(1)
  const poolQuery = useQuery(leavePoolQueryOptions(state, page, pageSize))
  const dateFormatter = new Intl.DateTimeFormat(i18n.resolvedLanguage, {
    dateStyle: "medium",
  })
  const timeFormatter = new Intl.DateTimeFormat(i18n.resolvedLanguage, {
    timeStyle: "short",
  })

  const invalidateLeaves = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["leaves"] }),
      queryClient.invalidateQueries({ queryKey: ["me", "leaves"] }),
      queryClient.invalidateQueries({
        queryKey: ["me", "notifications", "unread-count"],
      }),
    ])
  }

  const actionMutation = useMutation({
    mutationFn: async ({
      leave,
      action,
    }: {
      leave: Leave
      action: LeaveAction
    }) => {
      if (action === "cancel") {
        await cancelLeave(leave.id)
        return action
      }
      if (action === "reject") {
        await rejectShiftChangeRequest(leave.publication_id, leave.request.id)
        return action
      }
      await approveShiftChangeRequest(leave.publication_id, leave.request.id)
      return action
    },
    onSuccess: async (action) => {
      await invalidateLeaves()
      toast({
        variant: "default",
        description: t(`leaves.workbench.toast.${action}`),
      })
    },
    onError: (error) => {
      toast({
        variant: "destructive",
        description: getTranslatedApiError(
          t,
          error,
          "leave.errors",
          "leave.errors.INTERNAL_ERROR",
        ),
      })
    },
  })

  const poolData = poolQuery.data
  const totalCount = poolData?.total_count ?? 0
  const hasNext = page * pageSize < totalCount

  return (
    <div className="grid gap-6">
      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="grid gap-1">
              <CardTitle>{t("leaves.workbench.title")}</CardTitle>
              <CardDescription>
                {t("leaves.workbench.description")}
              </CardDescription>
            </div>
            <Link to="/leaves/new" className={buttonVariants()}>
              <Plus data-icon="inline-start" />
              {t("leaves.requestCta")}
            </Link>
          </div>
        </CardHeader>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex flex-wrap gap-2">
              {stateFilters.map((filter) => (
                <Button
                  key={filter}
                  type="button"
                  size="sm"
                  variant={state === filter ? "default" : "outline"}
                  onClick={() => {
                    setState(filter)
                    setPage(1)
                  }}
                >
                  {t(`leaves.workbench.filters.${filter}`)}
                </Button>
              ))}
            </div>
            <div className="text-sm text-muted-foreground">
              {t("leaves.workbench.total", { total: totalCount })}
            </div>
          </div>
        </CardHeader>
        <CardContent className="grid gap-4">
          {poolQuery.isLoading ? (
            <div className="grid gap-3">
              <Skeleton className="h-12 w-full" />
              <Skeleton className="h-12 w-full" />
              <Skeleton className="h-12 w-full" />
            </div>
          ) : poolQuery.isError ? (
            <div className="rounded-md border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
              {getTranslatedApiError(
                t,
                poolQuery.error,
                "leave.errors",
                "leave.errors.INTERNAL_ERROR",
              )}
            </div>
          ) : !poolData || poolData.leaves.length === 0 ? (
            <div className="rounded-md border border-dashed p-6 text-sm text-muted-foreground">
              {t("leaves.workbench.empty")}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("leaves.workbench.columns.requester")}</TableHead>
                  <TableHead>{t("leaves.workbench.columns.shift")}</TableHead>
                  <TableHead>{t("leaves.workbench.columns.type")}</TableHead>
                  <TableHead>{t("leaves.workbench.columns.status")}</TableHead>
                  <TableHead>{t("leaves.workbench.columns.coverage")}</TableHead>
                  <TableHead className="text-right">
                    {t("leaves.workbench.columns.actions")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {poolData.leaves.map((leave) => (
                  <LeavePoolRow
                    key={leave.id}
                    leave={leave}
                    dateFormatter={dateFormatter}
                    timeFormatter={timeFormatter}
                    isPending={actionMutation.isPending}
                    onAction={(action) =>
                      actionMutation.mutate({ leave, action })
                    }
                  />
                ))}
              </TableBody>
            </Table>
          )}

          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="text-sm text-muted-foreground">
              {t("leaves.workbench.page", { page })}
            </div>
            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                disabled={page === 1}
                onClick={() => setPage((current) => Math.max(1, current - 1))}
              >
                {t("leaves.workbench.previous")}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={!hasNext}
                onClick={() => setPage((current) => current + 1)}
              >
                {t("leaves.workbench.next")}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

function LeavePoolRow({
  leave,
  dateFormatter,
  timeFormatter,
  isPending,
  onAction,
}: {
  leave: Leave
  dateFormatter: Intl.DateTimeFormat
  timeFormatter: Intl.DateTimeFormat
  isPending: boolean
  onAction: (action: LeaveAction) => void
}) {
  const { t } = useTranslation()
  const shift = leave.shift
  const actions = leave.actions
  const coverage =
    leave.substitute_name ??
    leave.counterpart_name ??
    t("leaves.workbench.coverage.open")

  return (
    <TableRow
      className={cn(
        leave.urgency?.starts_within_24_hours &&
          "bg-muted/50 hover:bg-muted/70",
      )}
    >
      <TableCell>
        <div className="font-medium">
          {leave.requester_name || `#${leave.user_id}`}
        </div>
        <div className="text-muted-foreground">
          {t(`leave.category.${leave.category}`)}
        </div>
      </TableCell>
      <TableCell>
        <div className="font-medium">
          {shift
            ? dateFormatter.format(new Date(shift.occurrence_start))
            : leave.request.occurrence_date}
        </div>
        <div className="text-muted-foreground">
          {shift
            ? `${timeFormatter.format(
                new Date(shift.occurrence_start),
              )} - ${timeFormatter.format(new Date(shift.occurrence_end))} · ${
                shift.position_name
              }`
            : t("common.notAvailable")}
        </div>
        {leave.urgency?.starts_within_24_hours && (
          <Badge variant="secondary">
            {t("leaves.workbench.urgent")}
          </Badge>
        )}
        {leave.state === "pending" && leave.urgency && (
          <div className="text-muted-foreground">
            {t("leaves.workbench.urgency.remaining", {
              hours: Math.max(
                1,
                Math.ceil(leave.urgency.seconds_until_start / 3600),
              ),
            })}
          </div>
        )}
      </TableCell>
      <TableCell>{t(`leave.type.${leave.request.type}`)}</TableCell>
      <TableCell>
        <Badge variant={stateVariant[leave.state]}>
          {t(`leave.state.${leave.state}`)}
        </Badge>
      </TableCell>
      <TableCell>{coverage}</TableCell>
      <TableCell>
        <div className="flex flex-wrap justify-end gap-2">
          {actions?.can_claim && (
            <Button
              type="button"
              size="sm"
              disabled={isPending}
              onClick={() => onAction("claim")}
            >
              <HandHelping data-icon="inline-start" />
              {t("leaves.workbench.actions.claim")}
            </Button>
          )}
          {actions?.can_approve && (
            <Button
              type="button"
              size="sm"
              disabled={isPending}
              onClick={() => onAction("approve")}
            >
              <Check data-icon="inline-start" />
              {t("leaves.workbench.actions.approve")}
            </Button>
          )}
          {actions?.can_reject && (
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={isPending}
              onClick={() => onAction("reject")}
            >
              <X data-icon="inline-start" />
              {t("leaves.workbench.actions.reject")}
            </Button>
          )}
          {actions?.can_cancel && (
            <Button
              type="button"
              size="sm"
              variant="destructive"
              disabled={isPending}
              onClick={() => onAction("cancel")}
            >
              {t("leaves.workbench.actions.cancel")}
            </Button>
          )}
          {actions?.disabled_reason && (
            <div className="text-sm text-muted-foreground">
              {t(`leaves.workbench.disabled.${actions.disabled_reason}`)}
            </div>
          )}
          <Link
            to="/leaves/$leaveId"
            params={{ leaveId: String(leave.id) }}
            className={buttonVariants({ variant: "outline", size: "sm" })}
          >
            {t("leaves.workbench.actions.detail")}
          </Link>
        </div>
      </TableCell>
    </TableRow>
  )
}
