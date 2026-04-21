import { useEffect, useEffectEvent } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Controller, useForm } from "react-hook-form"
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
import { Label } from "@/components/ui/label"
import { useToast } from "@/components/ui/toast"
import { getTranslatedApiError } from "@/lib/api-error"
import {
  createShiftChangeRequest,
  shiftChangeRequestsQueryOptions,
  unreadNotificationsQueryOptions,
} from "@/lib/queries"
import type { PublicationMember } from "@/lib/types"

import {
  createGiveDirectSchema,
  type GiveDirectFormValues,
} from "./shift-change-schemas"

type GiveDirectDialogProps = {
  open: boolean
  publicationID: number
  myAssignmentID: number | null
  members: PublicationMember[]
  onOpenChange: (open: boolean) => void
}

const selectClassName =
  "border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex h-10 w-full rounded-md border px-3 py-2 text-sm focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"

export function GiveDirectDialog({
  open,
  publicationID,
  myAssignmentID,
  members,
  onOpenChange,
}: GiveDirectDialogProps) {
  const { t, i18n } = useTranslation()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const formSchema = createGiveDirectSchema(t)

  const {
    control,
    handleSubmit,
    reset,
    trigger,
    formState: { errors },
  } = useForm<GiveDirectFormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      counterpart_user_id: 0,
    },
  })

  useEffect(() => {
    if (open) {
      reset({ counterpart_user_id: 0 })
    }
  }, [open, reset, myAssignmentID])

  const revalidateVisibleErrors = useEffectEvent(() => {
    const errorFields = Object.keys(errors) as (keyof GiveDirectFormValues)[]
    if (errorFields.length > 0) {
      void trigger(errorFields)
    }
  })

  useEffect(() => {
    revalidateVisibleErrors()
  }, [i18n.language])

  const mutation = useMutation({
    mutationFn: async (values: GiveDirectFormValues) => {
      if (!myAssignmentID) {
        throw new Error("missing assignment")
      }
      return createShiftChangeRequest(publicationID, {
        type: "give_direct",
        requester_assignment_id: myAssignmentID,
        counterpart_user_id: values.counterpart_user_id,
      })
    },
    onSuccess: async () => {
      toast({
        variant: "default",
        description: t("requests.giveDirectDialog.success"),
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

  const submitHandler = handleSubmit((values) => {
    mutation.mutate(values)
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("requests.giveDirectDialog.title")}</DialogTitle>
          <DialogDescription>
            {t("requests.giveDirectDialog.description")}
          </DialogDescription>
        </DialogHeader>
        <form className="grid gap-4" onSubmit={submitHandler}>
          <div className="grid gap-2">
            <Label htmlFor="give-direct-user">
              {t("requests.giveDirectDialog.counterpartLabel")}
            </Label>
            <Controller
              control={control}
              name="counterpart_user_id"
              render={({ field }) => (
                <select
                  id="give-direct-user"
                  className={selectClassName}
                  value={field.value === 0 ? "" : String(field.value)}
                  onBlur={field.onBlur}
                  onChange={(event) => {
                    const next = Number(event.target.value)
                    field.onChange(Number.isFinite(next) ? next : 0)
                  }}
                >
                  <option value="">
                    {t("requests.giveDirectDialog.selectCounterpart")}
                  </option>
                  {members.map((member) => (
                    <option key={member.user_id} value={member.user_id}>
                      {member.name}
                    </option>
                  ))}
                </select>
              )}
            />
            {errors.counterpart_user_id && (
              <p className="text-sm text-destructive">
                {errors.counterpart_user_id.message}
              </p>
            )}
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              {t("common.cancel")}
            </Button>
            <Button
              type="submit"
              disabled={mutation.isPending || !myAssignmentID}
            >
              {mutation.isPending
                ? t("requests.giveDirectDialog.submitting")
                : t("requests.giveDirectDialog.submit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
