import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { useToast } from "@/components/ui/toast"
import { getTranslatedApiError } from "@/lib/api-error"
import {
  createShiftChangeRequest,
  shiftChangeRequestsQueryOptions,
  unreadNotificationsQueryOptions,
} from "@/lib/queries"

type GivePoolDialogProps = {
  open: boolean
  publicationID: number
  myAssignmentID: number | null
  occurrenceDate: string | null
  onOpenChange: (open: boolean) => void
}

export function GivePoolDialog({
  open,
  publicationID,
  myAssignmentID,
  occurrenceDate,
  onOpenChange,
}: GivePoolDialogProps) {
  const { t } = useTranslation()
  const { toast } = useToast()
  const queryClient = useQueryClient()

  const mutation = useMutation({
    mutationFn: async () => {
      if (!myAssignmentID || !occurrenceDate) {
        throw new Error("missing assignment")
      }
      return createShiftChangeRequest(publicationID, {
        type: "give_pool",
        requester_assignment_id: myAssignmentID,
        occurrence_date: occurrenceDate,
      })
    },
    onSuccess: async () => {
      toast({
        variant: "default",
        description: t("requests.givePoolDialog.success"),
      })
      onOpenChange(false)
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: shiftChangeRequestsQueryOptions(publicationID).queryKey,
        }),
        queryClient.invalidateQueries({
          queryKey: unreadNotificationsQueryOptions.queryKey,
        }),
        queryClient.invalidateQueries({ queryKey: ["roster", "current"] }),
      ])
    },
    onError: (error) => {
      toast({
        variant: "destructive",
        description: getTranslatedApiError(
          t,
          error,
          "requests.errors",
          "requests.errors.INTERNAL_ERROR",
        ),
      })
    },
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("requests.givePoolDialog.title")}</DialogTitle>
          <DialogDescription>
            {t("requests.givePoolDialog.description")}
          </DialogDescription>
        </DialogHeader>
        {occurrenceDate && (
          <div className="rounded-lg border bg-muted/40 p-3 text-sm">
            {t("requests.occurrenceLabel", { date: occurrenceDate })}
          </div>
        )}
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            {t("common.cancel")}
          </Button>
          <Button
            type="button"
            onClick={() => mutation.mutate()}
            disabled={mutation.isPending || !myAssignmentID || !occurrenceDate}
          >
            {mutation.isPending
              ? t("requests.givePoolDialog.submitting")
              : t("requests.givePoolDialog.submit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
