import { useState } from "react"
import { useMutation, useQuery } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { DownloadIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { GiveDirectDialog } from "@/components/shift-changes/give-direct-dialog"
import { GivePoolDialog } from "@/components/shift-changes/give-pool-dialog"
import {
  SwapDialog,
  type SwapDialogMyShift,
} from "@/components/shift-changes/swap-dialog"
import {
  WeeklyRoster,
  type WeeklyRosterOwnShift,
  type WeeklyRosterShiftAction,
} from "@/components/roster/weekly-roster"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useToast } from "@/components/ui/toast"
import {
  downloadPublicationScheduleXLSX,
  normalizeScheduleExportLanguage,
} from "@/lib/publications"
import {
  currentUserQueryOptions,
  publicationMembersQueryOptions,
  publicationRosterQueryOptions,
  rosterCurrentQueryOptions,
} from "@/lib/queries"

export const Route = createFileRoute("/_authenticated/roster")({
  component: RosterPage,
})

type DialogKind = "swap" | "give_direct" | "give_pool" | null

export function RosterPage() {
  const { t, i18n } = useTranslation()
  const { toast } = useToast()
  const { data: currentUser } = useQuery(currentUserQueryOptions)
  const rosterQuery = useQuery(rosterCurrentQueryOptions)
  const [weekStart, setWeekStart] = useState<string | null>(null)

  const publicationID = rosterQuery.data?.publication?.id ?? 0
  const selectedRosterQuery = useQuery({
    ...publicationRosterQueryOptions(publicationID, weekStart ?? undefined),
    enabled: publicationID > 0 && weekStart != null,
  })
  const activeRosterQuery = weekStart ? selectedRosterQuery : rosterQuery
  const activeRoster = activeRosterQuery.data
  const isPublished = rosterQuery.data?.publication?.state === "PUBLISHED"

  // Eagerly load members during PUBLISHED so dialogs open without a flash.
  const membersQuery = useQuery({
    ...publicationMembersQueryOptions(publicationID),
    enabled: publicationID > 0 && isPublished,
  })

  const [activeDialog, setActiveDialog] = useState<DialogKind>(null)
  const [activeShift, setActiveShift] = useState<WeeklyRosterOwnShift | null>(
    null,
  )

  const scheduleDownloadMutation = useMutation({
    mutationFn: async () => {
      if (!activeRoster?.publication) {
        throw new Error("Missing publication")
      }
      await downloadPublicationScheduleXLSX(
        activeRoster.publication,
        normalizeScheduleExportLanguage(i18n.resolvedLanguage ?? i18n.language),
      )
    },
    onError: () => {
      toast({
        variant: "destructive",
        description: t("roster.downloadFailed"),
      })
    },
  })

  const handleShiftAction = (
    shift: WeeklyRosterOwnShift,
    action: WeeklyRosterShiftAction,
  ) => {
    setActiveShift(shift)
    setActiveDialog(action.type)
  }

  const closeDialog = () => {
    setActiveDialog(null)
  }

  if (activeRosterQuery.isLoading) {
    return (
      <div className="grid gap-4">
        <Skeleton className="h-28 w-full" />
        <Skeleton className="h-[520px] w-full" />
      </div>
    )
  }

  if (activeRosterQuery.isError) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("roster.title")}</CardTitle>
          <CardDescription>{t("roster.loadError")}</CardDescription>
        </CardHeader>
      </Card>
    )
  }

  const roster = activeRoster
  if (!roster?.publication) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("roster.title")}</CardTitle>
          <CardDescription>{t("roster.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">
            {t("roster.empty")}
          </div>
        </CardContent>
      </Card>
    )
  }

  const members = membersQuery.data ?? []
  const otherMembers = currentUser
    ? members.filter((member) => member.user_id !== currentUser.id)
    : members

  const swapMyShift: SwapDialogMyShift | null = activeShift
    ? {
        assignmentID: activeShift.assignmentID,
        weekday: activeShift.weekday,
        occurrenceDate: activeShift.occurrenceDate,
        slot: activeShift.slot,
        position: activeShift.position,
      }
    : null

  return (
    <div className="grid gap-6">
      <Card>
        <CardHeader>
          <CardTitle>{t("roster.title")}</CardTitle>
          <CardDescription>
            {t("roster.descriptionWithPublication", {
              name: roster.publication.name,
            })}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={() => setWeekStart(addDays(roster.week_start, -7))}
          >
            {t("roster.previousWeek")}
          </Button>
          <div className="text-sm text-muted-foreground">
            {formatDate(roster.week_start)}
          </div>
          <Button
            type="button"
            variant="outline"
            onClick={() => setWeekStart(addDays(roster.week_start, 7))}
          >
            {t("roster.nextWeek")}
          </Button>
          <Button
            type="button"
            variant="outline"
            className="sm:ml-auto"
            disabled={scheduleDownloadMutation.isPending}
            onClick={() => scheduleDownloadMutation.mutate()}
          >
            <DownloadIcon data-icon="inline-start" />
            {t(
              scheduleDownloadMutation.isPending
                ? "roster.downloading"
                : "roster.downloadExcel",
            )}
          </Button>
        </CardContent>
      </Card>
      <WeeklyRoster
        weekdays={roster.weekdays}
        currentUserID={currentUser?.id}
        publication={roster.publication}
        onShiftAction={isPublished ? handleShiftAction : undefined}
      />
      <SwapDialog
        open={activeDialog === "swap"}
        publicationID={roster.publication.id}
        myShift={swapMyShift}
        members={otherMembers}
        rosterWeekdays={roster.weekdays}
        onOpenChange={(open) => {
          if (!open) {
            closeDialog()
          }
        }}
      />
      <GiveDirectDialog
        open={activeDialog === "give_direct"}
        publicationID={roster.publication.id}
        myAssignmentID={activeShift?.assignmentID ?? null}
        occurrenceDate={activeShift?.occurrenceDate ?? null}
        members={otherMembers}
        onOpenChange={(open) => {
          if (!open) {
            closeDialog()
          }
        }}
      />
      <GivePoolDialog
        open={activeDialog === "give_pool"}
        publicationID={roster.publication.id}
        myAssignmentID={activeShift?.assignmentID ?? null}
        occurrenceDate={activeShift?.occurrenceDate ?? null}
        onOpenChange={(open) => {
          if (!open) {
            closeDialog()
          }
        }}
      />
    </div>
  )
}

function addDays(dateValue: string, days: number) {
  const date = new Date(`${dateValue}T00:00:00Z`)
  date.setUTCDate(date.getUTCDate() + days)
  return date.toISOString().slice(0, 10)
}

function formatDate(dateValue: string) {
  return new Date(`${dateValue}T00:00:00Z`).toLocaleDateString()
}
