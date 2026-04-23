import { useEffect, useEffectEvent, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import { CloneTemplateDialog } from "@/components/templates/clone-template-dialog"
import { DeleteTemplateEntryDialog } from "@/components/templates/delete-template-entry-dialog"
import { DeleteTemplateDialog } from "@/components/templates/delete-template-dialog"
import { groupTemplateSlotsByWeekday } from "@/components/templates/group-template-slots"
import { TemplateSlotDialog } from "@/components/templates/template-slot-dialog"
import { TemplateSlotPositionDialog } from "@/components/templates/template-slot-position-dialog"
import {
  createTemplateSchema,
  type TemplateFormValues,
  type TemplateSlotFormValues,
  type TemplateSlotPositionFormValues,
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
  createTemplateSlot,
  createTemplateSlotPosition,
  currentUserQueryOptions,
  deleteTemplate,
  deleteTemplateSlot,
  deleteTemplateSlotPosition,
  templateQueryOptions,
  updateTemplate,
  updateTemplateSlot,
  updateTemplateSlotPosition,
} from "@/lib/queries"
import type { TemplateSlot, TemplateSlotPosition } from "@/lib/types"

export const Route = createFileRoute("/_authenticated/templates/$templateId")({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(currentUserQueryOptions)
    if (!user.is_admin) {
      throw redirect({ to: "/" })
    }
  },
  component: TemplateDetailPage,
})

type SlotDialogState = {
  mode: "create" | "edit"
  initialWeekday?: number
  slot: TemplateSlot | null
}

type SlotPositionDialogState = {
  mode: "create" | "edit"
  slot: TemplateSlot
  positionEntry: TemplateSlotPosition | null
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
  const [slotPendingDeletion, setSlotPendingDeletion] = useState<TemplateSlot | null>(
    null,
  )
  const [slotPositionPendingDeletion, setSlotPositionPendingDeletion] = useState<{
    slot: TemplateSlot
    positionEntry: TemplateSlotPosition
  } | null>(null)
  const [slotDialogState, setSlotDialogState] = useState<SlotDialogState | null>(
    null,
  )
  const [slotPositionDialogState, setSlotPositionDialogState] =
    useState<SlotPositionDialogState | null>(null)

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

  const showTemplateError = (error: unknown) => {
    toast({
      variant: "destructive",
      description: getTranslatedApiError(
        t,
        error,
        "templates.errors",
        "templates.errors.INTERNAL_ERROR",
      ),
    })
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
    onError: showTemplateError,
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
    onError: showTemplateError,
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
    onError: showTemplateError,
  })

  const createSlotMutation = useMutation({
    mutationFn: (values: TemplateSlotFormValues) =>
      createTemplateSlot(numericTemplateID, values),
    onSuccess: async () => {
      setSlotDialogState(null)
      await invalidateTemplateDetail()
      toast({
        variant: "default",
        description: t("templates.success.slotCreated"),
      })
    },
    onError: showTemplateError,
  })

  const updateSlotMutation = useMutation({
    mutationFn: ({
      slotID,
      values,
    }: {
      slotID: number
      values: TemplateSlotFormValues
    }) => updateTemplateSlot(numericTemplateID, slotID, values),
    onSuccess: async () => {
      setSlotDialogState(null)
      await invalidateTemplateDetail()
      toast({
        variant: "default",
        description: t("templates.success.slotUpdated"),
      })
    },
    onError: showTemplateError,
  })

  const deleteSlotMutation = useMutation({
    mutationFn: (slotID: number) => deleteTemplateSlot(numericTemplateID, slotID),
    onSuccess: async () => {
      setSlotPendingDeletion(null)
      await invalidateTemplateDetail()
      toast({
        variant: "default",
        description: t("templates.success.slotDeleted"),
      })
    },
    onError: showTemplateError,
  })

  const createSlotPositionMutation = useMutation({
    mutationFn: ({
      slotID,
      values,
    }: {
      slotID: number
      values: TemplateSlotPositionFormValues
    }) => createTemplateSlotPosition(numericTemplateID, slotID, values),
    onSuccess: async () => {
      setSlotPositionDialogState(null)
      await invalidateTemplateDetail()
      toast({
        variant: "default",
        description: t("templates.success.positionCreated"),
      })
    },
    onError: showTemplateError,
  })

  const updateSlotPositionMutation = useMutation({
    mutationFn: ({
      slotID,
      positionEntryID,
      values,
    }: {
      slotID: number
      positionEntryID: number
      values: TemplateSlotPositionFormValues
    }) =>
      updateTemplateSlotPosition(
        numericTemplateID,
        slotID,
        positionEntryID,
        values,
      ),
    onSuccess: async () => {
      setSlotPositionDialogState(null)
      await invalidateTemplateDetail()
      toast({
        variant: "default",
        description: t("templates.success.positionUpdated"),
      })
    },
    onError: showTemplateError,
  })

  const deleteSlotPositionMutation = useMutation({
    mutationFn: ({
      slotID,
      positionEntryID,
    }: {
      slotID: number
      positionEntryID: number
    }) =>
      deleteTemplateSlotPosition(numericTemplateID, slotID, positionEntryID),
    onSuccess: async () => {
      setSlotPositionPendingDeletion(null)
      await invalidateTemplateDetail()
      toast({
        variant: "default",
        description: t("templates.success.positionDeleted"),
      })
    },
    onError: showTemplateError,
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
  const groupedSlots = groupTemplateSlotsByWeekday(template.slots ?? [])
  const isLocked = template.is_locked
  const canManagePositions =
    !positionsQuery.isLoading &&
    !positionsQuery.isError &&
    (positionsQuery.data?.length ?? 0) > 0
  const slotMutationPending =
    createSlotMutation.isPending || updateSlotMutation.isPending
  const slotPositionMutationPending =
    createSlotPositionMutation.isPending || updateSlotPositionMutation.isPending

  const getSlotSummary = (slot: TemplateSlot) =>
    t("templates.deleteSlotDialog.summary", {
      weekday: t(weekdayKeyMap[slot.weekday]),
      startTime: slot.start_time,
      endTime: slot.end_time,
    })

  const getSlotPositionSummary = (
    slot: TemplateSlot,
    positionEntry: TemplateSlotPosition,
  ) => {
    const positionName =
      positionsByID.get(positionEntry.position_id)?.name ??
      t("templates.unknownPosition")

    return t("templates.deletePositionDialog.summary", {
      positionName,
      startTime: slot.start_time,
      endTime: slot.end_time,
      headcount: positionEntry.required_headcount,
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
              <CardTitle>{t("templates.slotsTitle")}</CardTitle>
              <CardDescription>{t("templates.slotsDescription")}</CardDescription>
            </div>
            <Button
              type="button"
              onClick={() =>
                setSlotDialogState({
                  mode: "create",
                  initialWeekday: 1,
                  slot: null,
                })
              }
              disabled={isLocked}
            >
              {t("templates.actions.addSlot")}
            </Button>
          </CardHeader>
          <CardContent className="grid gap-6">
            {positionsQuery.isError && (
              <p className="text-sm text-destructive">
                {t("templates.positionsLoadError")}
              </p>
            )}
            {!canManagePositions && !positionsQuery.isLoading && (
              <p className="text-sm text-muted-foreground">
                {t("templates.noPositions")}
              </p>
            )}
            {weekdayList.map((weekday) => {
              const slots = groupedSlots[weekday]

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
                        setSlotDialogState({
                          mode: "create",
                          initialWeekday: weekday,
                          slot: null,
                        })
                      }
                      disabled={isLocked}
                    >
                      {t("templates.actions.addSlot")}
                    </Button>
                  </div>
                  {slots.length === 0 && (
                    <div className="rounded-xl border border-dashed p-4 text-sm text-muted-foreground">
                      {t("templates.noSlotsForWeekday")}
                    </div>
                  )}
                  {slots.map((slot) => (
                    <article
                      key={slot.id}
                      className="grid gap-4 rounded-xl border p-4"
                    >
                      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                        <div className="grid gap-1">
                          <div className="font-medium">
                            {t("templates.slot.summary", {
                              startTime: slot.start_time,
                              endTime: slot.end_time,
                            })}
                          </div>
                          <div className="text-sm text-muted-foreground">
                            {slot.positions.length === 0
                              ? t("templates.noPositionsForSlot")
                              : t("templates.slot.positionsCount", {
                                  count: slot.positions.length,
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
                              setSlotDialogState({
                                mode: "edit",
                                initialWeekday: slot.weekday,
                                slot,
                              })
                            }
                          >
                            {t("templates.actions.editSlot")}
                          </Button>
                          <Button
                            type="button"
                            size="sm"
                            variant="outline"
                            disabled={isLocked || !canManagePositions}
                            onClick={() =>
                              setSlotPositionDialogState({
                                mode: "create",
                                slot,
                                positionEntry: null,
                              })
                            }
                          >
                            {t("templates.actions.addPosition")}
                          </Button>
                          <Button
                            type="button"
                            size="sm"
                            variant="destructive"
                            disabled={isLocked || deleteSlotMutation.isPending}
                            onClick={() => setSlotPendingDeletion(slot)}
                          >
                            {t("templates.actions.deleteSlot")}
                          </Button>
                        </div>
                      </div>

                      {slot.positions.length === 0 ? (
                        <div className="rounded-xl border border-dashed p-4 text-sm text-muted-foreground">
                          {t("templates.noPositionsForSlot")}
                        </div>
                      ) : (
                        <div className="grid gap-3">
                          {slot.positions.map((positionEntry) => {
                            const positionName =
                              positionsByID.get(positionEntry.position_id)?.name ??
                              t("templates.unknownPosition")

                            return (
                              <div
                                key={positionEntry.id}
                                className="flex flex-col gap-3 rounded-xl border border-dashed p-4 sm:flex-row sm:items-center sm:justify-between"
                              >
                                <div className="grid gap-1">
                                  <div className="font-medium">{positionName}</div>
                                  <div className="text-sm text-muted-foreground">
                                    {t("templates.position.summary", {
                                      headcount: positionEntry.required_headcount,
                                    })}
                                  </div>
                                </div>
                                <div className="flex flex-wrap gap-2">
                                  <Button
                                    type="button"
                                    size="sm"
                                    variant="outline"
                                    disabled={isLocked || !canManagePositions}
                                    onClick={() =>
                                      setSlotPositionDialogState({
                                        mode: "edit",
                                        slot,
                                        positionEntry,
                                      })
                                    }
                                  >
                                    {t("templates.actions.editPosition")}
                                  </Button>
                                  <Button
                                    type="button"
                                    size="sm"
                                    variant="destructive"
                                    disabled={
                                      isLocked || deleteSlotPositionMutation.isPending
                                    }
                                    onClick={() =>
                                      setSlotPositionPendingDeletion({
                                        slot,
                                        positionEntry,
                                      })
                                    }
                                  >
                                    {t("templates.actions.deletePosition")}
                                  </Button>
                                </div>
                              </div>
                            )
                          })}
                        </div>
                      )}
                    </article>
                  ))}
                </section>
              )
            })}
          </CardContent>
        </Card>
      </div>

      <TemplateSlotDialog
        mode={slotDialogState?.mode ?? "create"}
        open={slotDialogState !== null}
        initialWeekday={slotDialogState?.initialWeekday}
        slot={slotDialogState?.slot ?? null}
        isPending={slotMutationPending}
        onOpenChange={(open) => {
          if (!open) {
            setSlotDialogState(null)
          }
        }}
        onSubmit={(values) => {
          if (slotDialogState?.mode === "edit" && slotDialogState.slot) {
            updateSlotMutation.mutate({
              slotID: slotDialogState.slot.id,
              values,
            })
            return
          }

          createSlotMutation.mutate(values)
        }}
      />

      <TemplateSlotPositionDialog
        mode={slotPositionDialogState?.mode ?? "create"}
        open={slotPositionDialogState !== null}
        positions={positionsQuery.data ?? []}
        positionEntry={slotPositionDialogState?.positionEntry ?? null}
        isPending={slotPositionMutationPending}
        onOpenChange={(open) => {
          if (!open) {
            setSlotPositionDialogState(null)
          }
        }}
        onSubmit={(values) => {
          if (!slotPositionDialogState) {
            return
          }

          if (
            slotPositionDialogState.mode === "edit" &&
            slotPositionDialogState.positionEntry
          ) {
            updateSlotPositionMutation.mutate({
              slotID: slotPositionDialogState.slot.id,
              positionEntryID: slotPositionDialogState.positionEntry.id,
              values,
            })
            return
          }

          createSlotPositionMutation.mutate({
            slotID: slotPositionDialogState.slot.id,
            values,
          })
        }}
      />

      <CloneTemplateDialog
        open={isCloneDialogOpen}
        template={template}
        isPending={cloneTemplateMutation.isPending}
        onConfirm={() => cloneTemplateMutation.mutate()}
        onOpenChange={setIsCloneDialogOpen}
      />

      <DeleteTemplateEntryDialog
        open={slotPendingDeletion !== null}
        isPending={deleteSlotMutation.isPending}
        titleKey="templates.deleteSlotDialog.title"
        descriptionKey="templates.deleteSlotDialog.description"
        confirmKey="templates.deleteSlotDialog.confirm"
        pendingKey="templates.deleteSlotDialog.submitting"
        summary={slotPendingDeletion ? getSlotSummary(slotPendingDeletion) : ""}
        onConfirm={() => {
          if (!slotPendingDeletion) {
            return
          }

          deleteSlotMutation.mutate(slotPendingDeletion.id)
        }}
        onOpenChange={(open) => {
          if (!open) {
            setSlotPendingDeletion(null)
          }
        }}
      />

      <DeleteTemplateEntryDialog
        open={slotPositionPendingDeletion !== null}
        isPending={deleteSlotPositionMutation.isPending}
        titleKey="templates.deletePositionDialog.title"
        descriptionKey="templates.deletePositionDialog.description"
        confirmKey="templates.deletePositionDialog.confirm"
        pendingKey="templates.deletePositionDialog.submitting"
        summary={
          slotPositionPendingDeletion
            ? getSlotPositionSummary(
                slotPositionPendingDeletion.slot,
                slotPositionPendingDeletion.positionEntry,
              )
            : ""
        }
        onConfirm={() => {
          if (!slotPositionPendingDeletion) {
            return
          }

          deleteSlotPositionMutation.mutate({
            slotID: slotPositionPendingDeletion.slot.id,
            positionEntryID: slotPositionPendingDeletion.positionEntry.id,
          })
        }}
        onOpenChange={(open) => {
          if (!open) {
            setSlotPositionPendingDeletion(null)
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
