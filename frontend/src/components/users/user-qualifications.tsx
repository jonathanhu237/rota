import { useEffect, useRef, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { useToast } from "@/components/ui/toast"
import { getTranslatedApiError } from "@/lib/api-error"
import {
  allPositionsQueryOptions,
  replaceUserPositions,
  userPositionsQueryOptions,
} from "@/lib/queries"
import type { Position, User } from "@/lib/types"

type UserQualificationsProps = {
  open: boolean
  user: User
}

export function normalizeQualificationPositionIDs(positionIDs: number[]) {
  return Array.from(new Set(positionIDs)).sort((left, right) => left - right)
}

export function hasQualificationSelectionChanged(
  initialPositionIDs: number[],
  nextPositionIDs: number[],
) {
  const normalizedInitial = normalizeQualificationPositionIDs(initialPositionIDs)
  const normalizedNext = normalizeQualificationPositionIDs(nextPositionIDs)

  if (normalizedInitial.length !== normalizedNext.length) {
    return true
  }

  return normalizedInitial.some((positionID, index) => {
    return positionID !== normalizedNext[index]
  })
}

type ShouldInitializeQualificationSelectionInput = {
  open: boolean
  wasOpen: boolean
  userID: number
  initializedUserID: number | null
}

export function shouldInitializeQualificationSelection({
  open,
  wasOpen,
  userID,
  initializedUserID,
}: ShouldInitializeQualificationSelectionInput) {
  if (!open) {
    return false
  }

  if (!wasOpen) {
    return true
  }

  return initializedUserID !== userID
}

export function UserQualifications({
  open,
  user,
}: UserQualificationsProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { toast } = useToast()
  const [selectedPositionIDs, setSelectedPositionIDs] = useState<number[]>([])
  const wasOpenRef = useRef(false)
  const initializedUserIDRef = useRef<number | null>(null)

  const positionsQuery = useQuery({
    ...allPositionsQueryOptions(),
    enabled: open,
  })
  const userPositionsQuery = useQuery({
    ...userPositionsQueryOptions(user.id),
    enabled: open && user.id > 0,
  })

  useEffect(() => {
    if (!open) {
      wasOpenRef.current = false
      initializedUserIDRef.current = null
      return
    }

    if (userPositionsQuery.data === undefined) {
      wasOpenRef.current = true
      return
    }

    if (
      shouldInitializeQualificationSelection({
        open,
        wasOpen: wasOpenRef.current,
        userID: user.id,
        initializedUserID: initializedUserIDRef.current,
      })
    ) {
      setSelectedPositionIDs(
        normalizeQualificationPositionIDs(
          userPositionsQuery.data.map((position) => position.id),
        ),
      )
      initializedUserIDRef.current = user.id
    }

    wasOpenRef.current = true
  }, [open, user.id, userPositionsQuery.data])

  const currentPositionIDs = normalizeQualificationPositionIDs(
    (userPositionsQuery.data ?? []).map((position) => position.id),
  )

  const saveQualificationsMutation = useMutation({
    mutationFn: (positionIDs: number[]) => replaceUserPositions(user.id, positionIDs),
    onSuccess: async (_, positionIDs) => {
      const normalizedPositionIDs =
        normalizeQualificationPositionIDs(positionIDs)
      const positionsByID = new Map(
        (positionsQuery.data ?? []).map((position) => [position.id, position]),
      )
      const nextUserPositions = normalizedPositionIDs.flatMap((positionID) => {
        const position = positionsByID.get(positionID)
        return position ? [position] : []
      })

      initializedUserIDRef.current = user.id
      setSelectedPositionIDs(normalizedPositionIDs)
      queryClient.setQueryData<Position[]>(
        ["users", "positions", user.id],
        nextUserPositions,
      )
      await queryClient.invalidateQueries({
        queryKey: ["users", "positions", user.id],
      })
      toast({
        variant: "default",
        description: t("users.qualifications.success.saved"),
      })
    },
    onError: (error) => {
      toast({
        variant: "destructive",
        description: getTranslatedApiError(
          t,
          error,
          "users.errors",
          "users.errors.INTERNAL_ERROR",
        ),
      })
    },
  })

  const togglePosition = (positionID: number, checked: boolean) => {
    setSelectedPositionIDs((currentPositionIDs) => {
      if (checked) {
        return normalizeQualificationPositionIDs([
          ...currentPositionIDs,
          positionID,
        ])
      }

      return currentPositionIDs.filter(
        (currentPositionID) => currentPositionID !== positionID,
      )
    })
  }

  const isLoading = positionsQuery.isLoading || userPositionsQuery.isLoading
  const isDisabled =
    saveQualificationsMutation.isPending ||
    positionsQuery.isFetching ||
    userPositionsQuery.isFetching
  const hasChanges = hasQualificationSelectionChanged(
    currentPositionIDs,
    selectedPositionIDs,
  )

  return (
    <>
      <Separator />
      <div className="grid gap-4">
        <div className="space-y-1">
          <h3 className="text-sm font-semibold">
            {t("users.qualifications.title")}
          </h3>
          <p className="text-sm text-muted-foreground">
            {t("users.qualifications.description")}
          </p>
        </div>

        {isLoading && (
          <div className="grid gap-3">
            {Array.from({ length: 3 }).map((_, index) => (
              <Skeleton key={index} className="h-12 w-full" />
            ))}
          </div>
        )}

        {!isLoading && (positionsQuery.isError || userPositionsQuery.isError) && (
          <p className="text-sm text-destructive">
            {t("users.qualifications.loadError")}
          </p>
        )}

        {!isLoading &&
          !positionsQuery.isError &&
          !userPositionsQuery.isError &&
          positionsQuery.data?.length === 0 && (
            <p className="text-sm text-muted-foreground">
              {t("users.qualifications.empty")}
            </p>
          )}

        {!isLoading &&
          !positionsQuery.isError &&
          !userPositionsQuery.isError &&
          (positionsQuery.data?.length ?? 0) > 0 && (
            <div className="grid max-h-56 gap-2 overflow-y-auto rounded-xl border p-3">
              {positionsQuery.data?.map((position) => {
                const checked = selectedPositionIDs.includes(position.id)

                return (
                  <label
                    key={position.id}
                    className="flex items-start gap-3 rounded-lg border border-transparent px-2 py-2 text-sm hover:border-border hover:bg-muted/30"
                  >
                    <Checkbox
                      checked={checked}
                      disabled={isDisabled}
                      onChange={(event) =>
                        togglePosition(position.id, event.target.checked)
                      }
                    />
                    <div className="grid gap-1">
                      <span className="font-medium">{position.name}</span>
                      <span className="text-muted-foreground">
                        {position.description || t("positions.noDescription")}
                      </span>
                    </div>
                  </label>
                )
              })}
            </div>
          )}

        <div className="flex items-center justify-between gap-3">
          <p className="text-sm text-muted-foreground">
            {t("users.qualifications.selectedCount", {
              count: selectedPositionIDs.length,
            })}
          </p>
          <Button
            type="button"
            disabled={!hasChanges || isDisabled}
            onClick={() =>
              saveQualificationsMutation.mutate(selectedPositionIDs)
            }
          >
            {saveQualificationsMutation.isPending
              ? t("users.qualifications.saving")
              : t("users.qualifications.save")}
          </Button>
        </div>
      </div>
    </>
  )
}
