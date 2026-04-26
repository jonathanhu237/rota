import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Link, createFileRoute, redirect } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import { AssignmentBoard } from "@/components/assignments/assignment-board"
import { isAssignmentBoardMutable } from "@/components/assignments/assignment-board-state"
import { AutoAssignDialog } from "@/components/publications/auto-assign-dialog"
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
import { getTranslatedApiError } from "@/lib/api-error"
import {
  autoAssignPublication,
  createAssignment,
  currentUserQueryOptions,
  deleteAssignment,
  publicationAssignmentBoardQueryOptions,
} from "@/lib/queries"

export const Route = createFileRoute(
  "/_authenticated/publications/$publicationId/assignments",
)({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(currentUserQueryOptions)
    if (!user.is_admin) {
      throw redirect({ to: "/" })
    }
  },
  component: PublicationAssignmentsPage,
})

function PublicationAssignmentsPage() {
  const { publicationId } = Route.useParams()
  const numericPublicationID = Number(publicationId)

  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { toast } = useToast()
  const [isAutoAssignDialogOpen, setIsAutoAssignDialogOpen] = useState(false)

  const boardQuery = useQuery(publicationAssignmentBoardQueryOptions(numericPublicationID))
  const board = boardQuery.data

  const invalidateBoard = async () => {
    await Promise.all([
      queryClient.invalidateQueries({
        queryKey: ["publications", "detail", numericPublicationID, "board"],
      }),
      queryClient.invalidateQueries({
        queryKey: ["publications", "detail", numericPublicationID],
      }),
      queryClient.invalidateQueries({ queryKey: ["publications", "list"] }),
    ])
  }

  const updateAssignmentMutation = useMutation({
    mutationFn: async (
      action:
        | {
            type: "assign"
            userID: number
            slotID: number
            weekday: number
            positionID: number
          }
        | { type: "unassign"; assignmentID: number },
    ) => {
      if (action.type === "assign") {
        await createAssignment(numericPublicationID, {
          user_id: action.userID,
          slot_id: action.slotID,
          weekday: action.weekday,
          position_id: action.positionID,
        })
        return
      }

      await deleteAssignment(numericPublicationID, action.assignmentID)
    },
    onSuccess: async () => {
      await invalidateBoard()
    },
    onError: async (error) => {
      await invalidateBoard()
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

  const autoAssignMutation = useMutation({
    mutationFn: () => autoAssignPublication(numericPublicationID),
    onSuccess: async () => {
      setIsAutoAssignDialogOpen(false)
      await invalidateBoard()
      toast({
        variant: "default",
        description: t("assignments.success.autoAssigned"),
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

  if (boardQuery.isLoading) {
    return (
      <div className="grid gap-4">
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-[520px] w-full" />
      </div>
    )
  }

  if (boardQuery.isError || !board) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("assignments.title")}</CardTitle>
          <CardDescription>{t("assignments.loadError")}</CardDescription>
        </CardHeader>
      </Card>
    )
  }

  const isReadOnly = !isAssignmentBoardMutable(board.publication.state)
  const canAutoAssign = board.publication.state === "ASSIGNING"
  const isPending =
    updateAssignmentMutation.isPending || autoAssignMutation.isPending

  return (
    <>
      <AutoAssignDialog
        open={isAutoAssignDialogOpen}
        publication={board.publication}
        isPending={autoAssignMutation.isPending}
        onConfirm={() => autoAssignMutation.mutate()}
        onOpenChange={setIsAutoAssignDialogOpen}
      />
      <div className="grid gap-6">
        <Card>
          <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div className="space-y-1">
              <CardTitle>{t("assignments.title")}</CardTitle>
              <CardDescription>
                {t(
                  isReadOnly
                    ? "assignments.descriptionReadOnly"
                    : "assignments.descriptionEditable",
                  {
                    name: board.publication.name,
                  },
                )}
              </CardDescription>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {canAutoAssign && (
                <Button
                  type="button"
                  onClick={() => setIsAutoAssignDialogOpen(true)}
                  disabled={isPending}
                >
                  {t("assignments.autoAssign")}
                </Button>
              )}
              <Link
                className="text-sm font-medium text-foreground underline underline-offset-4"
                params={{ publicationId: String(board.publication.id) }}
                to="/publications/$publicationId"
              >
                {t("assignments.backToPublication")}
              </Link>
            </div>
          </CardHeader>
          <CardContent>
            {board.publication.state === "PUBLISHED" && (
              <div className="mb-4 rounded-xl border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-950">
                {t("publications.assignmentBoard.publishedWarning")}
              </div>
            )}
            {board.publication.state === "ACTIVE" && (
              <div className="mb-4 rounded-xl border border-orange-300 bg-orange-50 px-4 py-3 text-sm text-orange-950">
                {t("publications.assignmentBoard.activeWarning")}
              </div>
            )}
            <AssignmentBoard
              slots={board.slots}
              isPending={isPending}
              isReadOnly={isReadOnly}
              onAssign={(userID, slotID, weekday, positionID) =>
                updateAssignmentMutation.mutate({
                  type: "assign",
                  userID,
                  slotID,
                  weekday,
                  positionID,
                })
              }
              onDraftAssign={(userID, slotID, weekday, positionID) =>
                createAssignment(numericPublicationID, {
                  user_id: userID,
                  slot_id: slotID,
                  weekday,
                  position_id: positionID,
                })
              }
              onDraftRefresh={invalidateBoard}
              onDraftUnassign={(assignmentID) =>
                deleteAssignment(numericPublicationID, assignmentID)
              }
              onUnassign={(assignmentID) =>
                updateAssignmentMutation.mutate({
                  type: "unassign",
                  assignmentID,
                })
              }
            />
          </CardContent>
        </Card>
      </div>
    </>
  )
}
