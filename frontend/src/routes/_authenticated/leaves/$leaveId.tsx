import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { useToast } from "@/components/ui/toast"
import { getTranslatedApiError } from "@/lib/api-error"
import {
  approveShiftChangeRequest,
  cancelLeave,
  currentUserQueryOptions,
  leaveQueryOptions,
  myLeavesQueryOptions,
  rejectShiftChangeRequest,
} from "@/lib/queries"
import type { Leave, LeaveState } from "@/lib/types"

export const Route = createFileRoute("/_authenticated/leaves/$leaveId")({
  component: LeaveDetailPage,
})

const stateVariant: Record<
  LeaveState,
  "default" | "secondary" | "outline" | "destructive"
> = {
  pending: "outline",
  completed: "default",
  failed: "destructive",
  cancelled: "secondary",
}

function LeaveDetailPage() {
  const { t, i18n } = useTranslation()
  const params = Route.useParams()
  const leaveID = Number(params.leaveId)
  const queryClient = useQueryClient()
  const { toast } = useToast()
  const leaveQuery = useQuery(leaveQueryOptions(leaveID))
  const currentUserQuery = useQuery(currentUserQueryOptions)
  const formatter = new Intl.DateTimeFormat(i18n.resolvedLanguage, {
    dateStyle: "medium",
    timeStyle: "short",
  })

  const invalidateLeave = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: leaveQueryOptions(leaveID).queryKey }),
      queryClient.invalidateQueries({ queryKey: myLeavesQueryOptions(1, 10).queryKey }),
      queryClient.invalidateQueries({ queryKey: ["me", "leaves"] }),
    ])
  }

  const actionError = (error: unknown) => {
    toast({
      variant: "destructive",
      description: getTranslatedApiError(
        t,
        error,
        "leave.errors",
        "leave.errors.INTERNAL_ERROR",
      ),
    })
  }

  const approveMutation = useMutation({
    mutationFn: (leave: Leave) =>
      approveShiftChangeRequest(leave.publication_id, leave.request.id),
    onSuccess: async () => {
      await invalidateLeave()
      toast({ variant: "default", description: t("leaveDetail.toast.approved") })
    },
    onError: actionError,
  })

  const rejectMutation = useMutation({
    mutationFn: (leave: Leave) =>
      rejectShiftChangeRequest(leave.publication_id, leave.request.id),
    onSuccess: async () => {
      await invalidateLeave()
      toast({ variant: "default", description: t("leaveDetail.toast.rejected") })
    },
    onError: actionError,
  })

  const cancelMutation = useMutation({
    mutationFn: (leave: Leave) => cancelLeave(leave.id),
    onSuccess: async () => {
      await invalidateLeave()
      toast({ variant: "default", description: t("leaveDetail.toast.cancelled") })
    },
    onError: actionError,
  })

  if (leaveQuery.isLoading) {
    return (
      <div className="grid gap-4">
        <Skeleton className="h-40 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  if (leaveQuery.isError || !leaveQuery.data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("leaveDetail.title")}</CardTitle>
          <CardDescription>
            {getTranslatedApiError(
              t,
              leaveQuery.error,
              "leave.errors",
              "leave.errors.LEAVE_NOT_FOUND",
            )}
          </CardDescription>
        </CardHeader>
      </Card>
    )
  }

  const leave = leaveQuery.data
  const userID = currentUserQuery.data?.id
  const request = leave.request
  const isPending = request.state === "pending"
  const isRequester = userID === leave.user_id
  const isCounterpart = request.counterpart_user_id === userID
  const canClaim =
    isPending && request.type === "give_pool" && userID !== request.requester_user_id
  const canApprove =
    isPending &&
    (canClaim ||
      ((request.type === "swap" || request.type === "give_direct") &&
        isCounterpart))
  const isBusy =
    approveMutation.isPending ||
    rejectMutation.isPending ||
    cancelMutation.isPending

  return (
    <div className="grid gap-6">
      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={stateVariant[leave.state]}>
              {t(`leave.state.${leave.state}`)}
            </Badge>
            <Badge variant="outline">{t(`leave.category.${leave.category}`)}</Badge>
          </div>
          <CardTitle>{t("leaveDetail.title")}</CardTitle>
          <CardDescription>
            {request.occurrence_date} · {t(`leave.type.${request.type}`)}
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 text-sm">
          <DetailRow label={t("leaveDetail.reason")} value={leave.reason || t("common.notAvailable")} />
          <DetailRow label={t("leaveDetail.createdAt")} value={formatter.format(new Date(leave.created_at))} />
          <DetailRow label={t("leaveDetail.expiresAt")} value={formatter.format(new Date(request.expires_at))} />
          <DetailRow label={t("leaveDetail.requester")} value={`#${leave.user_id}`} />
          {request.counterpart_user_id != null && (
            <DetailRow label={t("leaveDetail.counterpart")} value={`#${request.counterpart_user_id}`} />
          )}
        </CardContent>
      </Card>

      {isPending && (
        <Card>
          <CardHeader>
            <CardTitle>{t("leaveDetail.actionsTitle")}</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-wrap gap-2">
            {canApprove && (
              <Button
                type="button"
                disabled={isBusy}
                onClick={() => approveMutation.mutate(leave)}
              >
                {canClaim
                  ? t("leaveDetail.actions.claim")
                  : t("leaveDetail.actions.approve")}
              </Button>
            )}
            {isCounterpart && request.type !== "give_pool" && (
              <Button
                type="button"
                variant="outline"
                disabled={isBusy}
                onClick={() => rejectMutation.mutate(leave)}
              >
                {t("leaveDetail.actions.reject")}
              </Button>
            )}
            {isRequester && (
              <Button
                type="button"
                variant="destructive"
                disabled={isBusy}
                onClick={() => cancelMutation.mutate(leave)}
              >
                {t("leaveDetail.actions.cancel")}
              </Button>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid gap-1 sm:grid-cols-[160px_minmax(0,1fr)]">
      <div className="text-muted-foreground">{label}</div>
      <div className="font-medium">{value}</div>
    </div>
  )
}
