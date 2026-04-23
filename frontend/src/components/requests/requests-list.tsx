import { useMutation, useQueryClient } from "@tanstack/react-query"
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
import { useToast } from "@/components/ui/toast"
import { getTranslatedApiError } from "@/lib/api-error"
import {
  approveShiftChangeRequest,
  cancelShiftChangeRequest,
  rejectShiftChangeRequest,
  shiftChangeRequestsQueryOptions,
  unreadNotificationsQueryOptions,
} from "@/lib/queries"
import type {
  PublicationMember,
  RosterWeekday,
  ShiftChangeRequest,
  ShiftChangeState,
  ShiftChangeType,
} from "@/lib/types"

type RequestsListProps = {
  publicationID: number
  requests: ShiftChangeRequest[]
  members: PublicationMember[]
  currentUserID: number
  rosterWeekdays?: RosterWeekday[]
}

type Bucket = "sent" | "waiting" | "pool"

const typeLabelKey: Record<ShiftChangeType, string> = {
  swap: "requests.type.swap",
  give_direct: "requests.type.give_direct",
  give_pool: "requests.type.give_pool",
}

const stateLabelKey: Record<ShiftChangeState, string> = {
  pending: "requests.state.pending",
  approved: "requests.state.approved",
  rejected: "requests.state.rejected",
  cancelled: "requests.state.cancelled",
  expired: "requests.state.expired",
  invalidated: "requests.state.invalidated",
}

const stateVariant: Record<
  ShiftChangeState,
  "default" | "secondary" | "outline" | "destructive"
> = {
  pending: "outline",
  approved: "default",
  rejected: "destructive",
  cancelled: "secondary",
  expired: "secondary",
  invalidated: "destructive",
}

const weekdayKeyMap: Record<number, string> = {
  1: "templates.weekday.mon",
  2: "templates.weekday.tue",
  3: "templates.weekday.wed",
  4: "templates.weekday.thu",
  5: "templates.weekday.fri",
  6: "templates.weekday.sat",
  7: "templates.weekday.sun",
}

function partitionRequests(
  requests: ShiftChangeRequest[],
  currentUserID: number,
) {
  const sent: ShiftChangeRequest[] = []
  const waiting: ShiftChangeRequest[] = []
  const pool: ShiftChangeRequest[] = []
  const history: ShiftChangeRequest[] = []

  for (const request of requests) {
    if (request.state !== "pending") {
      history.push(request)
      continue
    }

    if (request.requester_user_id === currentUserID) {
      sent.push(request)
      continue
    }

    if (
      request.type === "give_pool" &&
      request.requester_user_id !== currentUserID
    ) {
      pool.push(request)
      continue
    }

    if (
      request.counterpart_user_id === currentUserID &&
      (request.type === "swap" || request.type === "give_direct")
    ) {
      waiting.push(request)
    }
  }

  return { sent, waiting, pool, history }
}

export function RequestsList({
  publicationID,
  requests,
  members,
  currentUserID,
  rosterWeekdays = [],
}: RequestsListProps) {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const { toast } = useToast()

  const memberLookup = new Map<number, string>()
  for (const member of members) {
    memberLookup.set(member.user_id, member.name)
  }

  const formatter = new Intl.DateTimeFormat(i18n.resolvedLanguage, {
    dateStyle: "medium",
    timeStyle: "short",
  })
  const assignmentLookup = new Map<
    number,
    {
      weekday: number
      startTime: string
      endTime: string
      positionName: string
    }
  >()

  for (const weekday of rosterWeekdays) {
    for (const slot of weekday.slots) {
      for (const position of slot.positions) {
        for (const assignment of position.assignments) {
          assignmentLookup.set(assignment.assignment_id, {
            weekday: weekday.weekday,
            startTime: slot.slot.start_time,
            endTime: slot.slot.end_time,
            positionName: position.position.name,
          })
        }
      }
    }
  }

  const invalidateAfterMutation = async () => {
    await Promise.all([
      queryClient.invalidateQueries({
        queryKey: shiftChangeRequestsQueryOptions(publicationID).queryKey,
      }),
      queryClient.invalidateQueries({
        queryKey: unreadNotificationsQueryOptions.queryKey,
      }),
    ])
  }

  const handleError = (error: unknown) => {
    toast({
      variant: "destructive",
      description: getTranslatedApiError(
        t,
        error,
        "requests.errors",
        "requests.errors.INTERNAL_ERROR",
      ),
    })
  }

  const approveMutation = useMutation({
    mutationFn: (requestID: number) =>
      approveShiftChangeRequest(publicationID, requestID),
    onSuccess: async (_, requestID) => {
      await invalidateAfterMutation()
      const request = requests.find((item) => item.id === requestID)
      toast({
        variant: "default",
        description:
          request?.type === "give_pool"
            ? t("requests.toast.claimed")
            : t("requests.toast.approved"),
      })
    },
    onError: handleError,
  })

  const rejectMutation = useMutation({
    mutationFn: (requestID: number) =>
      rejectShiftChangeRequest(publicationID, requestID),
    onSuccess: async () => {
      await invalidateAfterMutation()
      toast({ variant: "default", description: t("requests.toast.rejected") })
    },
    onError: handleError,
  })

  const cancelMutation = useMutation({
    mutationFn: (requestID: number) =>
      cancelShiftChangeRequest(publicationID, requestID),
    onSuccess: async () => {
      await invalidateAfterMutation()
      toast({ variant: "default", description: t("requests.toast.cancelled") })
    },
    onError: handleError,
  })

  const { sent, waiting, pool, history } = partitionRequests(
    requests,
    currentUserID,
  )

  const formatTimestamp = (value: string) => formatter.format(new Date(value))

  const requesterName = (request: ShiftChangeRequest) =>
    memberLookup.get(request.requester_user_id) ??
    t("requests.unknownUser", { defaultValue: `#${request.requester_user_id}` })

  const renderAssignmentSummary = (assignmentID: number) => {
    const summary = assignmentLookup.get(assignmentID)
    if (!summary) {
      return t("requests.card.shift", {
        id: assignmentID,
      })
    }

    return t("requests.card.shiftSummary", {
      weekday: t(weekdayKeyMap[summary.weekday]),
      positionName: summary.positionName,
      startTime: summary.startTime,
      endTime: summary.endTime,
    })
  }

  const renderShiftSummary = (request: ShiftChangeRequest) => {
    const requesterShift = renderAssignmentSummary(
      request.requester_assignment_id,
    )

    if (request.type === "swap" && request.counterpart_assignment_id != null) {
      const counterpartShift = renderAssignmentSummary(
        request.counterpart_assignment_id,
      )
      return t("requests.card.swapSummary", {
        requesterShift,
        counterpartShift,
      })
    }

    return requesterShift
  }

  const renderCard = (
    request: ShiftChangeRequest,
    bucket: Bucket | "history",
  ) => {
    const isBusy =
      approveMutation.isPending ||
      rejectMutation.isPending ||
      cancelMutation.isPending
    const historyReason =
      bucket === "history" && request.state === "invalidated"
        ? t("requests.history.invalidatedReason")
        : null

    return (
      <Card key={request.id}>
        <CardHeader>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="outline">{t(typeLabelKey[request.type])}</Badge>
            <Badge variant={stateVariant[request.state]}>
              {t(stateLabelKey[request.state])}
            </Badge>
          </div>
          <CardTitle className="mt-2">{renderShiftSummary(request)}</CardTitle>
          <CardDescription>
            {t("requests.card.metadata", {
              requester: requesterName(request),
              createdAt: formatTimestamp(request.created_at),
            })}
          </CardDescription>
        </CardHeader>
        {(bucket !== "history" || historyReason) && (
          <CardContent>
            {bucket !== "history" && (
              <div className="flex flex-wrap gap-2">
                {bucket === "sent" && (
                  <Button
                    type="button"
                    variant="destructive"
                    disabled={isBusy}
                    onClick={() => cancelMutation.mutate(request.id)}
                  >
                    {t("requests.actions.cancel")}
                  </Button>
                )}
                {bucket === "waiting" && (
                  <>
                    <Button
                      type="button"
                      disabled={isBusy}
                      onClick={() => approveMutation.mutate(request.id)}
                    >
                      {t("requests.actions.approve")}
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      disabled={isBusy}
                      onClick={() => rejectMutation.mutate(request.id)}
                    >
                      {t("requests.actions.reject")}
                    </Button>
                  </>
                )}
                {bucket === "pool" && (
                  <Button
                    type="button"
                    disabled={isBusy}
                    onClick={() => approveMutation.mutate(request.id)}
                  >
                    {t("requests.actions.claim")}
                  </Button>
                )}
              </div>
            )}
            {historyReason && (
              <p className="text-sm text-muted-foreground">{historyReason}</p>
            )}
          </CardContent>
        )}
      </Card>
    )
  }

  const renderSection = (
    title: string,
    items: ShiftChangeRequest[],
    bucket: Bucket | "history",
    emptyKey: string,
  ) => (
    <section className="grid gap-3">
      <div className="flex items-center gap-2">
        <h2 className="text-base font-semibold">{title}</h2>
        <Badge variant="secondary">{items.length}</Badge>
      </div>
      {items.length === 0 ? (
        <div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
          {t(emptyKey)}
        </div>
      ) : (
        <div className="grid gap-3">
          {items.map((request) => renderCard(request, bucket))}
        </div>
      )}
    </section>
  )

  return (
    <div className="grid gap-6">
      {renderSection(
        t("requests.sections.waiting"),
        waiting,
        "waiting",
        "requests.sections.emptyWaiting",
      )}
      {renderSection(
        t("requests.sections.sent"),
        sent,
        "sent",
        "requests.sections.emptySent",
      )}
      {renderSection(
        t("requests.sections.pool"),
        pool,
        "pool",
        "requests.sections.emptyPool",
      )}
      {renderSection(
        t("requests.sections.history"),
        history,
        "history",
        "requests.sections.emptyHistory",
      )}
    </div>
  )
}
