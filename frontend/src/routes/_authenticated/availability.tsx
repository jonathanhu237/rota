import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import { AvailabilityGrid } from "@/components/availability/availability-grid"
import { PublicationStateBadge } from "@/components/publications/publication-state-badge"
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
  createAvailabilitySubmission,
  currentPublicationQueryOptions,
  deleteAvailabilitySubmission,
  myPublicationSubmissionsQueryOptions,
  publicationShiftsQueryOptions,
} from "@/lib/queries"

export const Route = createFileRoute("/_authenticated/availability")({
  component: AvailabilityPage,
})

function AvailabilityPage() {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const { toast } = useToast()

  const currentPublicationQuery = useQuery(currentPublicationQueryOptions)
  const currentPublication = currentPublicationQuery.data

  const isCollecting = currentPublication?.state === "COLLECTING"
  const publicationID = currentPublication?.id ?? 0

  const shiftsQuery = useQuery({
    ...publicationShiftsQueryOptions(publicationID),
    enabled: isCollecting,
  })
  const submissionsQuery = useQuery({
    ...myPublicationSubmissionsQueryOptions(publicationID),
    enabled: isCollecting,
  })

  const formatter = new Intl.DateTimeFormat(i18n.resolvedLanguage, {
    dateStyle: "medium",
    timeStyle: "short",
  })

  const invalidateAvailability = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["publications", "current"] }),
      queryClient.invalidateQueries({
        queryKey: ["publications", "current", "submissions"],
      }),
    ])
  }

  const toggleSubmissionMutation = useMutation({
    mutationFn: async ({
      slotID,
      positionID,
      checked,
    }: {
      slotID: number
      positionID: number
      checked: boolean
    }) => {
      if (!currentPublication) {
        return
      }

      if (checked) {
        await createAvailabilitySubmission(
          currentPublication.id,
          slotID,
          positionID,
        )
        return
      }

      await deleteAvailabilitySubmission(
        currentPublication.id,
        slotID,
        positionID,
      )
    },
    onSuccess: async () => {
      await invalidateAvailability()
    },
    onError: async (error) => {
      await invalidateAvailability()
      toast({
        variant: "destructive",
        description: getTranslatedApiError(
          t,
          error,
          "availability.errors",
          "availability.errors.INTERNAL_ERROR",
        ),
      })
    },
  })

  if (currentPublicationQuery.isLoading) {
    return (
      <div className="grid gap-4">
        <Skeleton className="h-40 w-full" />
        <Skeleton className="h-72 w-full" />
      </div>
    )
  }

  if (
    currentPublicationQuery.isError ||
    currentPublication == null ||
    currentPublication.state === "ENDED"
  ) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("availability.title")}</CardTitle>
          <CardDescription>{t("availability.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">
            {t("availability.empty")}
          </div>
        </CardContent>
      </Card>
    )
  }

  const formatTimestamp = (value: string | null) =>
    value ? formatter.format(new Date(value)) : t("common.notAvailable")

  const stateMessage = (() => {
    switch (currentPublication.state) {
      case "DRAFT":
        return t("availability.stateMessage.draft", {
          time: formatTimestamp(currentPublication.submission_start_at),
        })
      case "COLLECTING":
        return t("availability.stateMessage.collecting", {
          time: formatTimestamp(currentPublication.submission_end_at),
        })
      case "ASSIGNING":
        return t("availability.stateMessage.assigning")
      case "ACTIVE":
        return t("availability.stateMessage.active")
    }
  })()

  return (
    <div className="grid gap-6">
      <Card>
        <CardHeader>
          <CardTitle>{t("availability.title")}</CardTitle>
          <CardDescription>{t("availability.description")}</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3">
          <div className="flex flex-wrap items-center gap-3">
            <PublicationStateBadge state={currentPublication.state} />
            <span className="text-sm text-muted-foreground">{stateMessage}</span>
          </div>
          <div className="grid gap-1 text-sm">
            <div>
              <span className="text-muted-foreground">
                {t("availability.currentPublication")}:
              </span>{" "}
              <span className="font-medium">{currentPublication.name}</span>
            </div>
            <div>
              <span className="text-muted-foreground">
                {t("availability.template")}:
              </span>{" "}
              <span>{currentPublication.template_name}</span>
            </div>
          </div>
        </CardContent>
      </Card>

      {currentPublication.state === "COLLECTING" ? (
        <Card>
          <CardHeader>
            <CardTitle>{t("availability.gridTitle")}</CardTitle>
            <CardDescription>{t("availability.gridDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            {shiftsQuery.isLoading || submissionsQuery.isLoading ? (
              <div className="grid gap-3">
                {Array.from({ length: 4 }).map((_, index) => (
                  <Skeleton key={index} className="h-28 w-full" />
                ))}
              </div>
            ) : shiftsQuery.isError || submissionsQuery.isError ? (
              <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
                {shiftsQuery.isError
                  ? getTranslatedApiError(
                      t,
                      shiftsQuery.error,
                      "availability.errors",
                      "availability.errors.INTERNAL_ERROR",
                    )
                  : getTranslatedApiError(
                      t,
                      submissionsQuery.error,
                      "availability.errors",
                      "availability.errors.INTERNAL_ERROR",
                    )}
              </div>
            ) : shiftsQuery.data && shiftsQuery.data.length > 0 ? (
              <AvailabilityGrid
                shifts={shiftsQuery.data}
                selectedSlotPositions={submissionsQuery.data ?? []}
                isPending={toggleSubmissionMutation.isPending}
                onToggle={(slotID, positionID, checked) =>
                  toggleSubmissionMutation.mutate({
                    slotID,
                    positionID,
                    checked,
                  })
                }
              />
            ) : (
              <div className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">
                {t("availability.noQualifiedShifts")}
              </div>
            )}
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardContent className="pt-4">
            <div className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">
              {stateMessage}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
