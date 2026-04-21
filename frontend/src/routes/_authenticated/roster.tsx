import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
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
import { Skeleton } from "@/components/ui/skeleton"
import {
  currentUserQueryOptions,
  publicationMembersQueryOptions,
  rosterCurrentQueryOptions,
} from "@/lib/queries"

export const Route = createFileRoute("/_authenticated/roster")({
  component: RosterPage,
})

type DialogKind = "swap" | "give_direct" | "give_pool" | null

function RosterPage() {
  const { t } = useTranslation()
  const { data: currentUser } = useQuery(currentUserQueryOptions)
  const rosterQuery = useQuery(rosterCurrentQueryOptions)

  const publicationID = rosterQuery.data?.publication?.id ?? 0
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

  if (rosterQuery.isLoading) {
    return (
      <div className="grid gap-4">
        <Skeleton className="h-28 w-full" />
        <Skeleton className="h-[520px] w-full" />
      </div>
    )
  }

  if (rosterQuery.isError) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("roster.title")}</CardTitle>
          <CardDescription>{t("roster.loadError")}</CardDescription>
        </CardHeader>
      </Card>
    )
  }

  const roster = rosterQuery.data
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
        shift: activeShift.shift,
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
        onOpenChange={(open) => {
          if (!open) {
            closeDialog()
          }
        }}
      />
    </div>
  )
}
