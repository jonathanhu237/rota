import {
  useCallback,
  useMemo,
  useState,
} from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  Link,
  createFileRoute,
  redirect,
  useBlocker,
} from "@tanstack/react-router"
import { ArrowLeft, Save } from "lucide-react"
import { useTranslation } from "react-i18next"

import { AdminAvailabilityGrid } from "@/components/availability/admin-availability-grid"
import { getSlotWeekdayKey } from "@/components/availability/admin-availability-keys"
import { PublicationStateBadge } from "@/components/publications/publication-state-badge"
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
import { useToast } from "@/components/ui/toast"
import { getTranslatedApiError } from "@/lib/api-error"
import {
  adminAvailabilityDetailQueryOptions,
  currentUserQueryOptions,
  replaceAdminAvailability,
} from "@/lib/queries"
import type { AdminAvailabilityDetail, SlotRef } from "@/lib/types"

export const Route = createFileRoute(
  "/_authenticated/publications/$publicationId/availability/$userId",
)({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(currentUserQueryOptions)
    if (!user.is_admin) {
      throw redirect({ to: "/" })
    }
  },
  component: PublicationAvailabilityEditorPage,
})

export function PublicationAvailabilityEditorPage() {
  const { publicationId, userId } = Route.useParams()
  const numericPublicationID = Number(publicationId)
  const numericUserID = Number(userId)

  const { t } = useTranslation()
  const detailQuery = useQuery(
    adminAvailabilityDetailQueryOptions(numericPublicationID, numericUserID),
  )
  const detail = detailQuery.data

  if (detailQuery.isLoading) {
    return (
      <div className="grid gap-4">
        <Skeleton className="h-36 w-full" />
        <Skeleton className="h-[520px] w-full" />
      </div>
    )
  }

  if (detailQuery.isError || !detail) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("adminAvailability.editor.title")}</CardTitle>
          <CardDescription>{t("adminAvailability.loadError")}</CardDescription>
        </CardHeader>
      </Card>
    )
  }

  return (
    <AvailabilityEditor
      key={`${detail.user.id}:${serializeSlotRefs(detail.submissions)}`}
      detail={detail}
      publicationId={publicationId}
      numericPublicationID={numericPublicationID}
      numericUserID={numericUserID}
    />
  )
}

function AvailabilityEditor({
  detail,
  publicationId,
  numericPublicationID,
  numericUserID,
}: {
  detail: AdminAvailabilityDetail
  publicationId: string
  numericPublicationID: number
  numericUserID: number
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { toast } = useToast()
  const initialSlots = useMemo(
    () => normalizeSlotRefs(detail.submissions),
    [detail.submissions],
  )
  const initialSignature = useMemo(
    () => serializeSlotRefs(initialSlots),
    [initialSlots],
  )
  const [draft, setDraft] = useState<SlotRef[]>(initialSlots)
  const draftSignature = useMemo(() => serializeSlotRefs(draft), [draft])
  const isDirty = draftSignature !== initialSignature

  const shouldBlockNavigation = useCallback(() => {
    if (!isDirty) {
      return false
    }

    return !window.confirm(t("adminAvailability.editor.confirmLeave"))
  }, [isDirty, t])

  useBlocker({
    shouldBlockFn: shouldBlockNavigation,
    enableBeforeUnload: () => isDirty,
    disabled: !isDirty,
  })

  const replaceMutation = useMutation({
    mutationFn: () =>
      replaceAdminAvailability(
        numericPublicationID,
        numericUserID,
        normalizeSlotRefs(draft),
      ),
    onSuccess: async (nextDetail) => {
      queryClient.setQueryData(
        adminAvailabilityDetailQueryOptions(
          numericPublicationID,
          numericUserID,
        ).queryKey,
        nextDetail,
      )
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: [
            "publications",
            "detail",
            numericPublicationID,
            "availability",
          ],
        }),
        queryClient.invalidateQueries({
          queryKey: [
            "publications",
            "detail",
            numericPublicationID,
            "board",
          ],
        }),
      ])
      toast({
        variant: "default",
        description: t("adminAvailability.editor.saveSuccess"),
      })
    },
    onError: (error) => {
      toast({
        variant: "destructive",
        description: getTranslatedApiError(
          t,
          error,
          "publications.errors",
          "publications.errors.INTERNAL_ERROR",
        ),
      })
    },
  })

  const isReadOnly =
    detail.publication.state !== "COLLECTING" &&
    detail.publication.state !== "ASSIGNING"
  const changedCount = countChangedSlots(initialSignature, draftSignature)
  const ineligibleSelectedCount = countIneligibleSelected(draft, detail.cells)

  return (
    <div className="grid gap-6 pb-24">
      <Card>
        <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-2">
            <Link
              className={buttonVariants({ variant: "ghost", size: "sm" })}
              params={{ publicationId }}
              to="/publications/$publicationId/availability"
            >
              <ArrowLeft data-icon="inline-start" />
              {t("adminAvailability.editor.back")}
            </Link>
            <div className="space-y-1">
              <CardTitle>
                {t("adminAvailability.editor.titleForUser", {
                  name: detail.user.name,
                })}
              </CardTitle>
              <CardDescription>
                {t("adminAvailability.editor.description", {
                  email: detail.user.email,
                })}
              </CardDescription>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <PublicationStateBadge state={detail.publication.state} />
              {detail.positions.map((position) => (
                <Badge key={position.id} variant="outline">
                  {position.name}
                </Badge>
              ))}
            </div>
          </div>
        </CardHeader>
        <CardContent className="grid gap-3">
          <div className="rounded-lg border border-border bg-muted/30 p-3 text-sm text-muted-foreground">
            {t("adminAvailability.editor.autoAssignNote")}
          </div>
          {isReadOnly && (
            <div className="rounded-lg border border-amber-300 bg-amber-50 p-3 text-sm text-amber-950">
              {t("adminAvailability.editor.readOnly")}
            </div>
          )}
          {!isReadOnly && ineligibleSelectedCount > 0 && (
            <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
              {t("adminAvailability.editor.ineligibleSelected", {
                count: ineligibleSelectedCount,
              })}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("adminAvailability.editor.gridTitle")}</CardTitle>
          <CardDescription>
            {t("adminAvailability.editor.gridDescription")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {detail.slots.length > 0 ? (
            <AdminAvailabilityGrid
              detail={detail}
              selectedSlots={draft}
              isPending={replaceMutation.isPending}
              isReadOnly={isReadOnly}
              onToggle={(slotID, weekday, checked) =>
                setDraft((current) => toggleSlotRef(current, slotID, weekday, checked))
              }
            />
          ) : (
            <div className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">
              {t("adminAvailability.editor.noSlots")}
            </div>
          )}
        </CardContent>
      </Card>

      {isDirty && !isReadOnly && (
        <div
          className="sticky bottom-4 z-20 rounded-lg border bg-background p-3 shadow-lg"
          data-testid="admin-availability-save-bar"
        >
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="text-sm text-muted-foreground">
              {t("adminAvailability.editor.unsavedCount", {
                count: changedCount,
              })}
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                variant="outline"
                disabled={replaceMutation.isPending}
                onClick={() =>
                  setDraft(deserializeSlotRefs(initialSignature))
                }
              >
                {t("adminAvailability.editor.discard")}
              </Button>
              <Button
                type="button"
                disabled={
                  replaceMutation.isPending || ineligibleSelectedCount > 0
                }
                onClick={() => replaceMutation.mutate()}
              >
                <Save data-icon="inline-start" />
                {replaceMutation.isPending
                  ? t("adminAvailability.editor.saving")
                  : t("adminAvailability.editor.save")}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function toggleSlotRef(
  current: SlotRef[],
  slotID: number,
  weekday: number,
  checked: boolean,
) {
  const next = new Map(
    current.map((slot) => [
      getSlotWeekdayKey(slot.slot_id, slot.weekday),
      slot,
    ]),
  )
  const key = getSlotWeekdayKey(slotID, weekday)

  if (checked) {
    next.set(key, { slot_id: slotID, weekday })
  } else {
    next.delete(key)
  }

  return normalizeSlotRefs([...next.values()])
}

function normalizeSlotRefs(slots: SlotRef[]) {
  const deduped = new Map<string, SlotRef>()
  for (const slot of slots) {
    deduped.set(getSlotWeekdayKey(slot.slot_id, slot.weekday), {
      slot_id: slot.slot_id,
      weekday: slot.weekday,
    })
  }

  return [...deduped.values()].sort((left, right) => {
    if (left.slot_id !== right.slot_id) {
      return left.slot_id - right.slot_id
    }
    return left.weekday - right.weekday
  })
}

function serializeSlotRefs(slots: SlotRef[]) {
  return normalizeSlotRefs(slots)
    .map((slot) => getSlotWeekdayKey(slot.slot_id, slot.weekday))
    .join("|")
}

function deserializeSlotRefs(signature: string) {
  if (!signature) {
    return []
  }

  return signature.split("|").map((key) => {
    const [slotID, weekday] = key.split(":").map(Number)
    return { slot_id: slotID, weekday }
  })
}

function countChangedSlots(leftSignature: string, rightSignature: string) {
  const left = new Set(leftSignature ? leftSignature.split("|") : [])
  const right = new Set(rightSignature ? rightSignature.split("|") : [])
  let count = 0

  for (const key of left) {
    if (!right.has(key)) {
      count += 1
    }
  }
  for (const key of right) {
    if (!left.has(key)) {
      count += 1
    }
  }

  return count
}

function countIneligibleSelected(slots: SlotRef[], cells: { slot_id: number; weekday: number; eligible: boolean }[]) {
  const eligibility = new Map(
    cells.map((cell) => [
      getSlotWeekdayKey(cell.slot_id, cell.weekday),
      cell.eligible,
    ]),
  )

  return slots.filter(
    (slot) => !eligibility.get(getSlotWeekdayKey(slot.slot_id, slot.weekday)),
  ).length
}
