import { useEffect, useEffectEvent, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import { CloneTemplateDialog } from "@/components/templates/clone-template-dialog"
import { DeleteTemplateShiftDialog } from "@/components/templates/delete-template-shift-dialog"
import { DeleteTemplateDialog } from "@/components/templates/delete-template-dialog"
import { groupTemplateShiftsByWeekday } from "@/components/templates/group-template-shifts"
import { TemplateShiftDialog } from "@/components/templates/template-shift-dialog"
import {
  createTemplateSchema,
  type TemplateFormValues,
  type TemplateShiftFormValues,
} from "@/components/templates/template-schemas"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { useToast } from "@/components/ui/toast"
import { getTranslatedApiError } from "@/lib/api-error"
import {
  allPositionsQueryOptions,
  cloneTemplate,
  createTemplateShift,
  currentUserQueryOptions,
  deleteTemplate,
  deleteTemplateShift,
  templateQueryOptions,
  updateTemplate,
  updateTemplateShift,
} from "@/lib/queries"
import type { TemplateShift } from "@/lib/types"

export const Route = createFileRoute("/_authenticated/templates/$templateId")({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(currentUserQueryOptions)
    if (!user.is_admin) {
      throw redirect({ to: "/" })
    }
  },
  component: TemplateDetailPage,
})

type ShiftDialogState = {
  mode: "create" | "edit"
  initialWeekday?: number
  shift: TemplateShift | null
}

function TemplateDetailPage() {
  const { templateId } = Route.useParams()
  const numericTemplateID = Number(templateId)

  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { toast } = useToast()

  const [isCloneDialogOpen, setIsCloneDialogOpen] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [shiftPendingDeletion, setShiftPendingDeletion] = useState<TemplateShift | null>(
    null,
  )
  const [shiftDialogState, setShiftDialogState] = useState<ShiftDialogState | null>(
    null,
  )

  const { data: currentUser } = useQuery(currentUserQueryOptions)
  const templateQuery = useQuery(templateQueryOptions(numericTemplateID))
  const positionsQuery = useQuery(allPositionsQueryOptions())
  const template = templateQuery.data
  const formSchema = createTemplateSchema(t)

  const {
    register,
    handleSubmit,
    reset,
    trigger,
    formState: { errors, isDirty },
  } = useForm<TemplateFormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      name: template?.name ?? "",
      description: template?.description ?? "",
    },
  })

  useEffect(() => {
    if (template) {
      reset({
        name: template.name,
        description: template.description,
      })
    }
  }, [reset, template])

  const revalidateVisibleErrors = useEffectEvent(() => {
    const errorFields = Object.keys(errors) as (keyof TemplateFormValues)[]
    if (errorFields.length > 0) {
      void trigger(errorFields)
    }
  })

  useEffect(() => {
    revalidateVisibleErrors()
  }, [i18n.language])

  useEffect(() => {
    if (currentUser && !currentUser.is_admin) {
      navigate({ to: "/", replace: true })
    }
  }, [currentUser, navigate])

  const invalidateTemplateDetail = async () => {
    await Promise.all([
      queryClient.invalidateQueries({
        queryKey: ["templates", "detail", numericTemplateID],
      }),
      queryClient.invalidateQueries({ queryKey: ["templates", "list"] }),
    ])
  }

  const updateTemplateMutation = useMutation({
    mutationFn: (values: TemplateFormValues) =>
      updateTemplate(numericTemplateID, values),
    onSuccess: async () => {
      await invalidateTemplateDetail()
      toast({
        variant: "default",
        description: t("templates.success.updated"),
      })
    },
    onError: (error) => {
      toast({
        variant: "destructive",
        description: getTranslatedApiError(
          t,
          error,
          "templates.errors",
          "templates.errors.INTERNAL_ERROR",
        ),
      })
    },
  })

  const cloneTemplateMutation = useMutation({
    mutationFn: () => cloneTemplate(numericTemplateID),
    onSuccess: async (clonedTemplate) => {
      setIsCloneDialogOpen(false)
      await queryClient.invalidateQueries({ queryKey: ["templates", "list"] })
      toast({
        variant: "default",
        description: t("templates.success.cloned"),
      })
      navigate({
        to: "/templates/$templateId",
        params: { templateId: String(clonedTemplate.id) },
      })
    },
    onError: (error) => {
      toast({
        variant: "destructive",
        description: getTranslatedApiError(
          t,
          error,
          "templates.errors",
          "templates.errors.INTERNAL_ERROR",
        ),
      })
    },
  })

  const deleteTemplateMutation = useMutation({
    mutationFn: () => deleteTemplate(numericTemplateID),
    onSuccess: async () => {
      setIsDeleteDialogOpen(false)
      await queryClient.invalidateQueries({ queryKey: ["templates", "list"] })
      toast({
        variant: "default",
        description: t("templates.success.deleted"),
      })
      navigate({ to: "/templates" })
    },
    onError: (error) => {
      toast({
        variant: "destructive",
        description: getTranslatedApiError(
          t,
          error,
          "templates.errors",
          "templates.errors.INTERNAL_ERROR",
        ),
      })
    },
  })

  const createShiftMutation = useMutation({
    mutationFn: (values: TemplateShiftFormValues) =>
      createTemplateShift(numericTemplateID, values),
    onSuccess: async () => {
      setShiftDialogState(null)
      await invalidateTemplateDetail()
      toast({
        variant: "default",
        description: t("templates.success.shiftCreated"),
      })
    },
    onError: (error) => {
      toast({
        variant: "destructive",
        description: getTranslatedApiError(
          t,
          error,
          "templates.errors",
          "templates.errors.INTERNAL_ERROR",
        ),
      })
    },
  })

  const updateShiftMutation = useMutation({
    mutationFn: ({
      shiftID,
      values,
    }: {
      shiftID: number
      values: TemplateShiftFormValues
    }) => updateTemplateShift(numericTemplateID, shiftID, values),
    onSuccess: async () => {
      setShiftDialogState(null)
      await invalidateTemplateDetail()
      toast({
        variant: "default",
        description: t("templates.success.shiftUpdated"),
      })
    },
    onError: (error) => {
      toast({
        variant: "destructive",
        description: getTranslatedApiError(
          t,
          error,
          "templates.errors",
          "templates.errors.INTERNAL_ERROR",
        ),
      })
    },
  })

  const deleteShiftMutation = useMutation({
    mutationFn: (shiftID: number) => deleteTemplateShift(numericTemplateID, shiftID),
    onSuccess: async () => {
      setShiftPendingDeletion(null)
      await invalidateTemplateDetail()
      toast({
        variant: "default",
        description: t("templates.success.shiftDeleted"),
      })
    },
    onError: (error) => {
      toast({
        variant: "destructive",
        description: getTranslatedApiError(
          t,
          error,
          "templates.errors",
          "templates.errors.INTERNAL_ERROR",
        ),
      })
    },
  })

  if (templateQuery.isLoading) {
    return (
      <div className="grid gap-4">
        <Skeleton className="h-36 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  if (templateQuery.isError || !template) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("templates.detail.loadErrorTitle")}</CardTitle>
          <CardDescription>{t("templates.detail.loadErrorDescription")}</CardDescription>
        </CardHeader>
      </Card>
    )
  }

  const positionsByID = new Map(
    (positionsQuery.data ?? []).map((position) => [position.id, position]),
  )
  const groupedShifts = groupTemplateShiftsByWeekday(template.shifts ?? [])
  const isLocked = template.is_locked
  const shiftMutationPending =
    createShiftMutation.isPending || updateShiftMutation.isPending

  const getShiftSummary = (shift: TemplateShift) => {
    const positionName =
      positionsByID.get(shift.position_id)?.name ?? t("templates.unknownPosition")

    return t("templates.deleteShiftDialog.summary", {
      positionName,
      weekday: t(weekdayKeyMap[shift.weekday]),
      startTime: shift.start_time,
      endTime: shift.end_time,
      headcount: shift.required_headcount,
    })
  }

  return (
    <>
      <div className="grid gap-6">
        <Card>
          <CardHeader className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="space-y-2">
              <div className="flex flex-wrap items-center gap-2">
                <CardTitle>{template.name}</CardTitle>
                <Badge variant={isLocked ? "destructive" : "secondary"}>
                  {isLocked ? t("templates.locked") : t("templates.unlocked")}
                </Badge>
              </div>
              <CardDescription>{t("templates.detail.description")}</CardDescription>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => setIsCloneDialogOpen(true)}
                disabled={cloneTemplateMutation.isPending}
              >
                {cloneTemplateMutation.isPending
                  ? t("templates.cloneDialog.submitting")
                  : t("templates.actions.clone")}
              </Button>
              <Button
                type="button"
                variant="destructive"
                onClick={() => setIsDeleteDialogOpen(true)}
                disabled={isLocked || deleteTemplateMutation.isPending}
              >
                {deleteTemplateMutation.isPending
                  ? t("templates.deleteDialog.submitting")
                  : t("templates.actions.delete")}
              </Button>
            </div>
          </CardHeader>
          <CardContent className="grid gap-4">
            {isLocked && (
              <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
                {t("templates.lockedBanner")}
              </div>
            )}
            <form
              className="grid gap-4"
              onSubmit={handleSubmit((values) => updateTemplateMutation.mutate(values))}
            >
              <div className="grid gap-2">
                <Label htmlFor="template-editor-name">{t("templates.name")}</Label>
                <Input
                  id="template-editor-name"
                  disabled={isLocked || updateTemplateMutation.isPending}
                  {...register("name")}
                />
                {errors.name && (
                  <p className="text-sm text-destructive">{errors.name.message}</p>
                )}
              </div>
              <div className="grid gap-2">
                <Label htmlFor="template-editor-description">
                  {t("templates.descriptionLabel")}
                </Label>
                <Textarea
                  id="template-editor-description"
                  rows={4}
                  disabled={isLocked || updateTemplateMutation.isPending}
                  {...register("description")}
                />
                {errors.description && (
                  <p className="text-sm text-destructive">
                    {errors.description.message}
                  </p>
                )}
              </div>
              <div className="flex justify-end">
                <Button
                  type="submit"
                  disabled={isLocked || !isDirty || updateTemplateMutation.isPending}
                >
                  {updateTemplateMutation.isPending
                    ? t("templates.form.submittingEdit")
                    : t("templates.form.submitEdit")}
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div className="space-y-1">
              <CardTitle>{t("templates.shiftsTitle")}</CardTitle>
              <CardDescription>{t("templates.shiftsDescription")}</CardDescription>
            </div>
            <Button
              type="button"
              onClick={() =>
                setShiftDialogState({
                  mode: "create",
                  initialWeekday: 1,
                  shift: null,
                })
              }
              disabled={
                isLocked ||
                positionsQuery.isLoading ||
                positionsQuery.isError ||
                (positionsQuery.data?.length ?? 0) === 0
              }
            >
              {t("templates.actions.addShift")}
            </Button>
          </CardHeader>
          <CardContent className="grid gap-6">
            {positionsQuery.isError && (
              <p className="text-sm text-destructive">
                {t("templates.positionsLoadError")}
              </p>
            )}
            {(positionsQuery.data?.length ?? 0) === 0 && !positionsQuery.isLoading && (
              <p className="text-sm text-muted-foreground">
                {t("templates.noPositions")}
              </p>
            )}
            {weekdayList.map((weekday) => {
              const shifts = groupedShifts[weekday]

              return (
                <section key={weekday} className="grid gap-3">
                  <div className="flex items-center justify-between gap-3">
                    <h3 className="text-base font-semibold">
                      {t(weekdayKeyMap[weekday])}
                    </h3>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() =>
                        setShiftDialogState({
                          mode: "create",
                          initialWeekday: weekday,
                          shift: null,
                        })
                      }
                      disabled={
                        isLocked ||
                        positionsQuery.isLoading ||
                        positionsQuery.isError ||
                        (positionsQuery.data?.length ?? 0) === 0
                      }
                    >
                      {t("templates.actions.addShift")}
                    </Button>
                  </div>
                  {shifts.length === 0 && (
                    <div className="rounded-xl border border-dashed p-4 text-sm text-muted-foreground">
                      {t("templates.noShiftsForWeekday")}
                    </div>
                  )}
                  {shifts.map((shift) => {
                    const position = positionsByID.get(shift.position_id)

                    return (
                      <div
                        key={shift.id}
                        className="flex flex-col gap-3 rounded-xl border p-4 sm:flex-row sm:items-center sm:justify-between"
                      >
                        <div className="grid gap-1">
                          <div className="font-medium">
                            {position?.name ?? t("templates.unknownPosition")}
                          </div>
                          <div className="text-sm text-muted-foreground">
                            {t("templates.shift.summary", {
                              startTime: shift.start_time,
                              endTime: shift.end_time,
                              headcount: shift.required_headcount,
                            })}
                          </div>
                        </div>
                        <div className="flex flex-wrap gap-2">
                          <Button
                            type="button"
                            size="sm"
                            variant="outline"
                            disabled={isLocked}
                            onClick={() =>
                              setShiftDialogState({
                                mode: "edit",
                                initialWeekday: shift.weekday,
                                shift,
                              })
                            }
                          >
                            {t("templates.actions.editShift")}
                          </Button>
                          <Button
                            type="button"
                            size="sm"
                            variant="destructive"
                            disabled={isLocked || deleteShiftMutation.isPending}
                            onClick={() => setShiftPendingDeletion(shift)}
                          >
                            {t("templates.actions.deleteShift")}
                          </Button>
                        </div>
                      </div>
                    )
                  })}
                </section>
              )
            })}
          </CardContent>
        </Card>
      </div>

      <TemplateShiftDialog
        mode={shiftDialogState?.mode ?? "create"}
        open={shiftDialogState !== null}
        initialWeekday={shiftDialogState?.initialWeekday}
        positions={positionsQuery.data ?? []}
        shift={shiftDialogState?.shift ?? null}
        isPending={shiftMutationPending}
        onOpenChange={(open) => {
          if (!open) {
            setShiftDialogState(null)
          }
        }}
        onSubmit={(values) => {
          if (shiftDialogState?.mode === "edit" && shiftDialogState.shift) {
            updateShiftMutation.mutate({
              shiftID: shiftDialogState.shift.id,
              values,
            })
            return
          }

          createShiftMutation.mutate(values)
        }}
      />

      <CloneTemplateDialog
        open={isCloneDialogOpen}
        template={template}
        isPending={cloneTemplateMutation.isPending}
        onConfirm={() => cloneTemplateMutation.mutate()}
        onOpenChange={setIsCloneDialogOpen}
      />

      <DeleteTemplateShiftDialog
        open={shiftPendingDeletion !== null}
        isPending={deleteShiftMutation.isPending}
        summary={
          shiftPendingDeletion ? getShiftSummary(shiftPendingDeletion) : ""
        }
        onConfirm={() => {
          if (!shiftPendingDeletion) {
            return
          }

          deleteShiftMutation.mutate(shiftPendingDeletion.id)
        }}
        onOpenChange={(open) => {
          if (!open) {
            setShiftPendingDeletion(null)
          }
        }}
      />

      <DeleteTemplateDialog
        open={isDeleteDialogOpen}
        template={template}
        isPending={deleteTemplateMutation.isPending}
        onConfirm={() => deleteTemplateMutation.mutate()}
        onOpenChange={setIsDeleteDialogOpen}
      />
    </>
  )
}

const weekdayList = [1, 2, 3, 4, 5, 6, 7] as const

const weekdayKeyMap: Record<number, string> = {
  1: "templates.weekday.mon",
  2: "templates.weekday.tue",
  3: "templates.weekday.wed",
  4: "templates.weekday.thu",
  5: "templates.weekday.fri",
  6: "templates.weekday.sat",
  7: "templates.weekday.sun",
}
