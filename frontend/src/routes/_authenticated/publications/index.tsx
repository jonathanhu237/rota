import { useEffect, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Link, createFileRoute, redirect, useNavigate } from "@tanstack/react-router"
import { Plus } from "lucide-react"
import { useTranslation } from "react-i18next"

import { ActivatePublicationDialog } from "@/components/publications/activate-publication-dialog"
import { CreatePublicationDialog } from "@/components/publications/create-publication-dialog"
import { EndPublicationDialog } from "@/components/publications/end-publication-dialog"
import { PublicationsTable } from "@/components/publications/publications-table"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { useToast } from "@/components/ui/toast"
import {
  getApiErrorDetails,
  getTranslatedApiError,
} from "@/lib/api-error"
import {
  activatePublication,
  allTemplatesQueryOptions,
  createPublication,
  currentPublicationQueryOptions,
  currentUserQueryOptions,
  endPublication,
  publicationsQueryOptions,
} from "@/lib/queries"
import type { Publication } from "@/lib/types"

const pageSize = 10

export const Route = createFileRoute("/_authenticated/publications/")({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(currentUserQueryOptions)
    if (!user.is_admin) {
      throw redirect({ to: "/" })
    }
  },
  component: PublicationsPage,
})

function PublicationsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { toast } = useToast()

  const [page, setPage] = useState(1)
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [activationTarget, setActivationTarget] = useState<Publication | null>(
    null,
  )
  const [endTarget, setEndTarget] = useState<Publication | null>(null)

  const { data: currentUser } = useQuery(currentUserQueryOptions)
  const publicationsQuery = useQuery(publicationsQueryOptions(page, pageSize))
  const templatesQuery = useQuery({
    ...allTemplatesQueryOptions(),
    enabled: isCreateDialogOpen,
  })

  useEffect(() => {
    if (currentUser && !currentUser.is_admin) {
      navigate({ to: "/", replace: true })
    }
  }, [currentUser, navigate])

  const createPublicationMutation = useMutation({
    mutationFn: createPublication,
    onSuccess: async (publication) => {
      setIsCreateDialogOpen(false)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["publications", "list"] }),
        queryClient.invalidateQueries({ queryKey: ["publications", "current"] }),
      ])
      toast({
        variant: "default",
        description: t("publications.success.created"),
      })

      if (publication) {
        navigate({
          to: "/publications/$publicationId",
          params: { publicationId: String(publication.id) },
        })
      }
    },
    onError: async (error) => {
      const apiError = getApiErrorDetails(error)

      if (apiError?.code === "PUBLICATION_ALREADY_EXISTS") {
        const currentPublication = await queryClient.fetchQuery(
          currentPublicationQueryOptions,
        )

        toast({
          variant: "destructive",
          description: currentPublication ? (
            <span>
              {t("publications.errors.PUBLICATION_ALREADY_EXISTS")}{" "}
              <Link
                className="font-medium text-foreground underline underline-offset-4"
                params={{ publicationId: String(currentPublication.id) }}
                to="/publications/$publicationId"
              >
                {t("publications.actions.viewExisting")}
              </Link>
            </span>
          ) : (
            t("publications.errors.PUBLICATION_ALREADY_EXISTS")
          ),
        })
        return
      }

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

  const invalidatePublicationState = async (publicationID: number) => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["publications", "list"] }),
      queryClient.invalidateQueries({ queryKey: ["publications", "detail", publicationID] }),
      queryClient.invalidateQueries({
        queryKey: ["publications", "detail", publicationID, "board"],
      }),
      queryClient.invalidateQueries({ queryKey: ["publications", "current"] }),
      queryClient.invalidateQueries({ queryKey: ["roster", "current"] }),
    ])
  }

  const activatePublicationMutation = useMutation({
    mutationFn: (publicationID: number) => activatePublication(publicationID),
    onSuccess: async (_, publicationID) => {
      setActivationTarget(null)
      await invalidatePublicationState(publicationID)
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
    mutationFn: (publicationID: number) => endPublication(publicationID),
    onSuccess: async (_, publicationID) => {
      setEndTarget(null)
      await invalidatePublicationState(publicationID)
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

  const openPublication = (publication: Publication) => {
    navigate({
      to: "/publications/$publicationId",
      params: { publicationId: String(publication.id) },
    })
  }

  return (
    <>
      <Card>
        <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <CardTitle>{t("publications.title")}</CardTitle>
            <CardDescription>{t("publications.description")}</CardDescription>
          </div>
          <Button onClick={() => setIsCreateDialogOpen(true)}>
            <Plus />
            {t("publications.createPublication")}
          </Button>
        </CardHeader>
        <CardContent>
          {publicationsQuery.isError ? (
            <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
              {getTranslatedApiError(
                t,
                publicationsQuery.error,
                "publications.errors",
                "publications.errors.INTERNAL_ERROR",
              )}
            </div>
          ) : (
            <PublicationsTable
              publications={publicationsQuery.data?.publications ?? []}
              pagination={publicationsQuery.data?.pagination}
              isLoading={publicationsQuery.isLoading}
              isFetching={publicationsQuery.isFetching}
              onOpen={openPublication}
              onLifecycleAction={(publication, action) => {
                if (action === "activate") {
                  setActivationTarget(publication)
                  return
                }

                setEndTarget(publication)
              }}
              onPageChange={setPage}
            />
          )}
        </CardContent>
      </Card>
      <CreatePublicationDialog
        open={isCreateDialogOpen}
        templates={templatesQuery.data ?? []}
        isPending={createPublicationMutation.isPending}
        isTemplatesLoading={templatesQuery.isLoading}
        onOpenChange={setIsCreateDialogOpen}
        onSubmit={(values) => createPublicationMutation.mutate(values)}
      />
      <ActivatePublicationDialog
        open={activationTarget !== null}
        publication={activationTarget}
        isPending={activatePublicationMutation.isPending}
        onConfirm={() => {
          if (!activationTarget) {
            return
          }

          activatePublicationMutation.mutate(activationTarget.id)
        }}
        onOpenChange={(open: boolean) => {
          if (!open) {
            setActivationTarget(null)
          }
        }}
      />
      <EndPublicationDialog
        open={endTarget !== null}
        publication={endTarget}
        isPending={endPublicationMutation.isPending}
        onConfirm={() => {
          if (!endTarget) {
            return
          }

          endPublicationMutation.mutate(endTarget.id)
        }}
        onOpenChange={(open: boolean) => {
          if (!open) {
            setEndTarget(null)
          }
        }}
      />
    </>
  )
}
