import { useEffect, useState, type ReactNode } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Link, createFileRoute, redirect, useNavigate } from "@tanstack/react-router"
import { Trash2 } from "lucide-react"
import { useTranslation } from "react-i18next"

import { ActivatePublicationDialog } from "@/components/publications/activate-publication-dialog"
import { DeletePublicationDialog } from "@/components/publications/delete-publication-dialog"
import { EndPublicationDialog } from "@/components/publications/end-publication-dialog"
import { PublicationStateBadge } from "@/components/publications/publication-state-badge"
import { PublishPublicationDialog } from "@/components/publications/publish-publication-dialog"
import { Button, buttonVariants } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useToast } from "@/components/ui/toast"
import { getTranslatedApiError } from "@/lib/api-error"
import { getPublicationLifecycleAction } from "@/lib/publications"
import {
  activatePublication,
  currentUserQueryOptions,
  deletePublication,
  endPublication,
  publicationQueryOptions,
  publishPublication,
  updatePublication,
} from "@/lib/queries"

export const Route = createFileRoute("/_authenticated/publications/$publicationId")({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(currentUserQueryOptions)
    if (!user.is_admin) {
      throw redirect({ to: "/" })
    }
  },
  component: PublicationDetailPage,
})

function PublicationDetailPage() {
  const { publicationId } = Route.useParams()
  const numericPublicationID = Number(publicationId)

  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { toast } = useToast()

  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [isPublishDialogOpen, setIsPublishDialogOpen] = useState(false)
  const [isActivateDialogOpen, setIsActivateDialogOpen] = useState(false)
  const [isEndDialogOpen, setIsEndDialogOpen] = useState(false)
  const [plannedUntilDraft, setPlannedUntilDraft] = useState<{
    publicationID: number
    value: string
  } | null>(null)

  const { data: currentUser } = useQuery(currentUserQueryOptions)
  const publicationQuery = useQuery(publicationQueryOptions(numericPublicationID))
  const publication = publicationQuery.data

  const plannedUntilInput =
    publication && plannedUntilDraft?.publicationID === publication.id
      ? plannedUntilDraft.value
      : publication
        ? toDateTimeLocal(publication.planned_active_until)
        : ""
  const plannedUntilTimestamp = Date.parse(plannedUntilInput)

  useEffect(() => {
    if (currentUser && !currentUser.is_admin) {
      navigate({ to: "/", replace: true })
    }
  }, [currentUser, navigate])

  const formatter = new Intl.DateTimeFormat(i18n.resolvedLanguage, {
    dateStyle: "medium",
    timeStyle: "short",
  })

  const formatTimestamp = (value: string | null) =>
    value ? formatter.format(new Date(value)) : t("common.notAvailable")

  const deletePublicationMutation = useMutation({
    mutationFn: () => deletePublication(numericPublicationID),
    onSuccess: async () => {
      setIsDeleteDialogOpen(false)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["publications", "list"] }),
        queryClient.invalidateQueries({
          queryKey: ["publications", "detail", numericPublicationID],
        }),
        queryClient.invalidateQueries({ queryKey: ["publications", "current"] }),
      ])
      toast({
        variant: "default",
        description: t("publications.success.deleted"),
      })
      navigate({ to: "/publications" })
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

  const invalidatePublicationState = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["publications", "list"] }),
      queryClient.invalidateQueries({
        queryKey: ["publications", "detail", numericPublicationID],
      }),
      queryClient.invalidateQueries({
        queryKey: ["publications", "detail", numericPublicationID, "board"],
      }),
      queryClient.invalidateQueries({ queryKey: ["publications", "current"] }),
      queryClient.invalidateQueries({ queryKey: ["roster", "current"] }),
    ])
  }

  const publishPublicationMutation = useMutation({
    mutationFn: () => publishPublication(numericPublicationID),
    onSuccess: async () => {
      setIsPublishDialogOpen(false)
      await invalidatePublicationState()
      toast({
        variant: "default",
        description: t("publications.success.published"),
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

  const activatePublicationMutation = useMutation({
    mutationFn: () => activatePublication(numericPublicationID),
    onSuccess: async () => {
      setIsActivateDialogOpen(false)
      await invalidatePublicationState()
      toast({
        variant: "default",
        description: t("publications.success.activated"),
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

  const endPublicationMutation = useMutation({
    mutationFn: () => endPublication(numericPublicationID),
    onSuccess: async () => {
      setIsEndDialogOpen(false)
      await invalidatePublicationState()
      toast({
        variant: "default",
        description: t("publications.success.ended"),
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

  const updatePublicationMutation = useMutation({
    mutationFn: () => {
      if (!publication || Number.isNaN(plannedUntilTimestamp)) {
        throw new Error("invalid planned_active_until")
      }
      return updatePublication(numericPublicationID, {
        planned_active_until: new Date(plannedUntilTimestamp).toISOString(),
      })
    },
    onSuccess: async () => {
      setPlannedUntilDraft(null)
      await invalidatePublicationState()
      toast({
        variant: "default",
        description: t("publications.success.updated"),
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

  const getStateDescription = () => {
    if (!publication) {
      return ""
    }

    switch (publication.state) {
      case "DRAFT":
        return t("publications.detail.stateDescription.draft", {
          time: formatTimestamp(publication.submission_start_at),
        })
      case "COLLECTING":
        return t("publications.detail.stateDescription.collecting", {
          time: formatTimestamp(publication.submission_end_at),
        })
      case "ASSIGNING":
        return t("publications.detail.stateDescription.assigning")
      case "PUBLISHED":
        return t("publications.detail.stateDescription.published")
      case "ACTIVE":
        return t("publications.detail.stateDescription.active", {
          time: formatTimestamp(publication.activated_at),
        })
      case "ENDED":
        return t("publications.detail.stateDescription.ended", {
          time: formatTimestamp(publication.planned_active_until),
        })
    }
  }

  const lifecycleAction = publication
    ? getPublicationLifecycleAction(publication.state)
    : null

  if (publicationQuery.isLoading) {
    return (
      <div className="grid gap-4">
        <Skeleton className="h-40 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  if (publicationQuery.isError || !publication) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("publications.detail.loadErrorTitle")}</CardTitle>
          <CardDescription>
            {t("publications.detail.loadErrorDescription")}
          </CardDescription>
        </CardHeader>
      </Card>
    )
  }

  return (
    <>
      <div className="grid gap-6">
        <Card>
          <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div className="space-y-1">
              <CardTitle>{publication.name}</CardTitle>
              <CardDescription>{t("publications.detail.description")}</CardDescription>
            </div>
            <div className="flex flex-wrap gap-2">
              {(publication.state === "ASSIGNING" ||
                publication.state === "PUBLISHED" ||
                publication.state === "ACTIVE") && (
                <Link
                  className={buttonVariants({ variant: "outline" })}
                  params={{ publicationId: String(publication.id) }}
                  to="/publications/$publicationId/assignments"
                >
                  {t("publications.actions.openAssignmentBoard")}
                </Link>
              )}
              {(publication.state === "PUBLISHED" ||
                publication.state === "ACTIVE") && (
                <Link
                  className={buttonVariants({ variant: "outline" })}
                  params={{ publicationId: String(publication.id) }}
                  to="/publications/$publicationId/shift-changes"
                >
                  {t("publications.actions.viewShiftChanges")}
                </Link>
              )}
              {lifecycleAction === "publish" && (
                <Button onClick={() => setIsPublishDialogOpen(true)}>
                  {t("publications.actions.publish")}
                </Button>
              )}
              {lifecycleAction === "activate" && (
                <Button onClick={() => setIsActivateDialogOpen(true)}>
                  {t("publications.actions.activate")}
                </Button>
              )}
              {lifecycleAction === "end" && (
                <Button
                  variant="destructive"
                  onClick={() => setIsEndDialogOpen(true)}
                >
                  {t("publications.actions.end")}
                </Button>
              )}
              {publication.state === "DRAFT" && (
                <Button
                  variant="destructive"
                  onClick={() => setIsDeleteDialogOpen(true)}
                >
                  <Trash2 />
                  {t("publications.actions.delete")}
                </Button>
              )}
            </div>
          </CardHeader>
          <CardContent className="grid gap-4">
            <div className="flex flex-wrap items-center gap-3">
              <PublicationStateBadge state={publication.state} />
              <span className="text-sm text-muted-foreground">
                {getStateDescription()}
              </span>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <MetadataItem
                label={t("publications.detail.template")}
                value={
                  <Link
                    className="font-medium text-foreground underline underline-offset-4"
                    params={{ templateId: String(publication.template_id) }}
                    to="/templates/$templateId"
                  >
                    {publication.template_name}
                  </Link>
                }
              />
              <MetadataItem
                label={t("publications.detail.state")}
                value={t(`publications.state.${publication.state.toLowerCase()}`)}
              />
              <MetadataItem
                label={t("publications.detail.submissionStartAt")}
                value={formatTimestamp(publication.submission_start_at)}
              />
              <MetadataItem
                label={t("publications.detail.submissionEndAt")}
                value={formatTimestamp(publication.submission_end_at)}
              />
              <MetadataItem
                label={t("publications.detail.plannedActiveFrom")}
                value={formatTimestamp(publication.planned_active_from)}
              />
              <MetadataItem
                label={t("publications.detail.plannedActiveUntil")}
                value={formatTimestamp(publication.planned_active_until)}
              />
              <MetadataItem
                label={t("publications.detail.activatedAt")}
                value={formatTimestamp(publication.activated_at)}
              />
              <MetadataItem
                label={t("publications.detail.createdAt")}
                value={formatTimestamp(publication.created_at)}
              />
              <MetadataItem
                label={t("publications.detail.updatedAt")}
                value={formatTimestamp(publication.updated_at)}
              />
            </div>
            <form
              className="grid gap-3 rounded-lg border p-3 sm:max-w-md"
              onSubmit={(event) => {
                event.preventDefault()
                updatePublicationMutation.mutate()
              }}
            >
              <Label htmlFor="publication-planned-until-edit">
                {t("publications.detail.editPlannedActiveUntil")}
              </Label>
              <div className="flex flex-col gap-2 sm:flex-row">
                <Input
                  id="publication-planned-until-edit"
                  type="datetime-local"
                  value={plannedUntilInput}
                  onChange={(event) =>
                    setPlannedUntilDraft({
                      publicationID: publication.id,
                      value: event.target.value,
                    })
                  }
                />
                <Button
                  type="submit"
                  disabled={
                    updatePublicationMutation.isPending ||
                    !plannedUntilInput ||
                    Number.isNaN(plannedUntilTimestamp)
                  }
                >
                  {updatePublicationMutation.isPending
                    ? t("publications.detail.saving")
                    : t("publications.detail.save")}
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      </div>
      <DeletePublicationDialog
        open={isDeleteDialogOpen}
        publication={publication}
        isPending={deletePublicationMutation.isPending}
        onConfirm={() => deletePublicationMutation.mutate()}
        onOpenChange={setIsDeleteDialogOpen}
      />
      <PublishPublicationDialog
        open={isPublishDialogOpen}
        publication={publication}
        isPending={publishPublicationMutation.isPending}
        onConfirm={() => publishPublicationMutation.mutate()}
        onOpenChange={setIsPublishDialogOpen}
      />
      <ActivatePublicationDialog
        open={isActivateDialogOpen}
        publication={publication}
        isPending={activatePublicationMutation.isPending}
        onConfirm={() => activatePublicationMutation.mutate()}
        onOpenChange={setIsActivateDialogOpen}
      />
      <EndPublicationDialog
        open={isEndDialogOpen}
        publication={publication}
        isPending={endPublicationMutation.isPending}
        onConfirm={() => endPublicationMutation.mutate()}
        onOpenChange={setIsEndDialogOpen}
      />
    </>
  )
}

function toDateTimeLocal(value: string) {
  const date = new Date(value)
  const offsetMs = date.getTimezoneOffset() * 60 * 1000
  return new Date(date.getTime() - offsetMs).toISOString().slice(0, 16)
}

function MetadataItem({
  label,
  value,
}: {
  label: string
  value: ReactNode
}) {
  return (
    <div className="grid gap-1 rounded-lg border p-3">
      <div className="text-sm text-muted-foreground">{label}</div>
      <div>{value}</div>
    </div>
  )
}
