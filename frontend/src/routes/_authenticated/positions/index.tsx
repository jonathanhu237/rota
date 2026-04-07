import { useEffect, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router"
import { BriefcaseBusiness } from "lucide-react"
import { useTranslation } from "react-i18next"

import { DeletePositionDialog } from "@/components/positions/delete-position-dialog"
import {
  PositionFormDialog,
  type PositionFormValues,
} from "@/components/positions/position-form-dialog"
import { PositionsTable } from "@/components/positions/positions-table"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { useToast } from "@/components/ui/toast"
import { getTranslatedApiError } from "@/lib/api-error"
import {
  createPosition,
  currentUserQueryOptions,
  deletePosition,
  positionsQueryOptions,
  updatePosition,
} from "@/lib/queries"
import type { Position } from "@/lib/types"

const pageSize = 10

export const Route = createFileRoute("/_authenticated/positions/")({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(currentUserQueryOptions)
    if (!user.is_admin) {
      throw redirect({ to: "/" })
    }
  },
  component: PositionsPage,
})

function PositionsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { toast } = useToast()

  const [page, setPage] = useState(1)
  const [formMode, setFormMode] = useState<"create" | "edit">("create")
  const [selectedPosition, setSelectedPosition] = useState<Position | null>(null)
  const [isFormOpen, setIsFormOpen] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)

  const { data: currentUser } = useQuery(currentUserQueryOptions)
  const positionsQuery = useQuery(positionsQueryOptions(page, pageSize))

  useEffect(() => {
    if (currentUser && !currentUser.is_admin) {
      navigate({ to: "/", replace: true })
    }
  }, [currentUser, navigate])

  const handleMutationError = (error: unknown) => {
    toast({
      variant: "destructive",
      description: getTranslatedApiError(
        t,
        error,
        "positions.errors",
        "positions.errors.INTERNAL_ERROR",
      ),
    })
  }

  const invalidatePositions = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["positions"] }),
      queryClient.invalidateQueries({ queryKey: ["auth"] }),
    ])
  }

  const createPositionMutation = useMutation({
    mutationFn: createPosition,
    onSuccess: async () => {
      const total = positionsQuery.data?.pagination.total ?? 0
      const nextPage = Math.max(1, Math.ceil((total + 1) / pageSize))

      setIsFormOpen(false)
      setSelectedPosition(null)
      setPage(nextPage)
      await invalidatePositions()
      toast({
        variant: "default",
        description: t("positions.success.created"),
      })
    },
    onError: handleMutationError,
  })

  const updatePositionMutation = useMutation({
    mutationFn: ({
      positionID,
      values,
    }: {
      positionID: number
      values: PositionFormValues
    }) =>
      updatePosition(positionID, {
        name: values.name,
        description: values.description,
      }),
    onSuccess: async () => {
      setIsFormOpen(false)
      setSelectedPosition(null)
      await invalidatePositions()
      toast({
        variant: "default",
        description: t("positions.success.updated"),
      })
    },
    onError: handleMutationError,
  })

  const deletePositionMutation = useMutation({
    mutationFn: (positionID: number) => deletePosition(positionID),
    onSuccess: async () => {
      const currentPageTotal = positionsQuery.data?.positions.length ?? 0
      const nextPage =
        currentPageTotal <= 1 && page > 1 ? page - 1 : page

      setIsDeleteDialogOpen(false)
      setSelectedPosition(null)
      setPage(nextPage)
      await invalidatePositions()
      toast({
        variant: "default",
        description: t("positions.success.deleted"),
      })
    },
    onError: handleMutationError,
  })

  const openCreateDialog = () => {
    setFormMode("create")
    setSelectedPosition(null)
    setIsFormOpen(true)
  }

  const openEditDialog = (position: Position) => {
    setFormMode("edit")
    setSelectedPosition(position)
    setIsFormOpen(true)
  }

  const openDeleteDialog = (position: Position) => {
    setSelectedPosition(position)
    setIsDeleteDialogOpen(true)
  }

  const handlePositionFormSubmit = (values: PositionFormValues) => {
    if (formMode === "create") {
      createPositionMutation.mutate(values)
      return
    }

    if (!selectedPosition) {
      return
    }

    updatePositionMutation.mutate({
      positionID: selectedPosition.id,
      values,
    })
  }

  return (
    <>
      <Card>
        <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <CardTitle>{t("positions.title")}</CardTitle>
            <CardDescription>{t("positions.description")}</CardDescription>
          </div>
          <Button onClick={openCreateDialog}>
            <BriefcaseBusiness />
            {t("positions.createPosition")}
          </Button>
        </CardHeader>
        <CardContent>
          <PositionsTable
            positions={positionsQuery.data?.positions ?? []}
            pagination={positionsQuery.data?.pagination}
            isLoading={positionsQuery.isLoading}
            isFetching={positionsQuery.isFetching}
            onPageChange={setPage}
            onEdit={openEditDialog}
            onDelete={openDeleteDialog}
          />
        </CardContent>
      </Card>
      <PositionFormDialog
        mode={formMode}
        open={isFormOpen}
        position={selectedPosition}
        isPending={
          formMode === "create"
            ? createPositionMutation.isPending
            : updatePositionMutation.isPending
        }
        onOpenChange={(open) => {
          setIsFormOpen(open)
          if (!open) {
            setSelectedPosition(null)
          }
        }}
        onSubmit={handlePositionFormSubmit}
      />
      <DeletePositionDialog
        open={isDeleteDialogOpen}
        position={selectedPosition}
        isPending={deletePositionMutation.isPending}
        onOpenChange={(open) => {
          setIsDeleteDialogOpen(open)
          if (!open) {
            setSelectedPosition(null)
          }
        }}
        onConfirm={() => {
          if (!selectedPosition) {
            return
          }

          deletePositionMutation.mutate(selectedPosition.id)
        }}
      />
    </>
  )
}
