import { useEffect, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router"
import { UserPlus } from "lucide-react"
import { useTranslation } from "react-i18next"

import { StatusToggleDialog } from "@/components/users/status-toggle-dialog"
import {
  UserFormDialog,
  type UserFormValues,
} from "@/components/users/user-form-dialog"
import { UsersTable } from "@/components/users/users-table"
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
  createUser,
  currentUserQueryOptions,
  resendInvitation,
  updateUser,
  updateUserStatus,
  usersQueryOptions,
} from "@/lib/queries"
import type { User } from "@/lib/types"

const pageSize = 10

export const Route = createFileRoute("/_authenticated/users/")({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(currentUserQueryOptions)
    if (!user.is_admin) {
      throw redirect({ to: "/" })
    }
  },
  component: UsersPage,
})

function UsersPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { toast } = useToast()

  const [page, setPage] = useState(1)
  const [formMode, setFormMode] = useState<"create" | "edit">("create")
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [isFormOpen, setIsFormOpen] = useState(false)
  const [isStatusDialogOpen, setIsStatusDialogOpen] = useState(false)

  const { data: currentUser } = useQuery(currentUserQueryOptions)
  const usersQuery = useQuery(usersQueryOptions(page, pageSize))

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
        "users.errors",
        "users.errors.INTERNAL_ERROR",
      ),
    })
  }

  const invalidateUsers = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["users"] }),
      queryClient.invalidateQueries({ queryKey: ["auth"] }),
    ])
  }

  const createUserMutation = useMutation({
    mutationFn: createUser,
    onSuccess: async () => {
      const total = usersQuery.data?.pagination.total ?? 0
      const nextPage = Math.max(1, Math.ceil((total + 1) / pageSize))

      setIsFormOpen(false)
      setSelectedUser(null)
      setPage(nextPage)
      await invalidateUsers()
      toast({
        variant: "default",
        description: t("users.success.created"),
      })
    },
    onError: handleMutationError,
  })

  const resendInvitationMutation = useMutation({
    mutationFn: (userID: number) => resendInvitation(userID),
    onSuccess: async () => {
      await invalidateUsers()
      toast({
        variant: "default",
        description: t("users.success.invitationResent"),
      })
    },
    onError: handleMutationError,
  })

  const updateUserMutation = useMutation({
    mutationFn: ({
      userID,
      version,
      values,
    }: {
      userID: number
      version: number
      values: UserFormValues
    }) =>
      updateUser(userID, {
        email: values.email,
        name: values.name,
        is_admin: values.is_admin,
        version,
      }),
    onSuccess: async () => {
      setIsFormOpen(false)
      setSelectedUser(null)
      await invalidateUsers()
      toast({
        variant: "default",
        description: t("users.success.updated"),
      })
    },
    onError: handleMutationError,
  })

  const updateStatusMutation = useMutation({
    mutationFn: ({
      userID,
      version,
      status,
    }: {
      userID: number
      version: number
      status: User["status"]
    }) =>
      updateUserStatus(userID, {
        status,
        version,
      }),
    onSuccess: async (_, variables) => {
      const nextStatus = variables.status

      setIsStatusDialogOpen(false)
      setSelectedUser(null)
      await invalidateUsers()
      toast({
        variant: "default",
        description:
          nextStatus === "disabled"
            ? t("users.success.disabled")
            : t("users.success.enabled"),
      })
    },
    onError: handleMutationError,
  })

  const openCreateDialog = () => {
    setFormMode("create")
    setSelectedUser(null)
    setIsFormOpen(true)
  }

  const openEditDialog = (user: User) => {
    setFormMode("edit")
    setSelectedUser(user)
    setIsFormOpen(true)
  }

  const openStatusDialog = (user: User) => {
    setSelectedUser(user)
    setIsStatusDialogOpen(true)
  }

  const handleUserFormSubmit = (values: UserFormValues) => {
    if (formMode === "create") {
      createUserMutation.mutate({
        email: values.email,
        name: values.name,
        is_admin: values.is_admin,
      })
      return
    }

    if (!selectedUser) {
      return
    }

    updateUserMutation.mutate({
      userID: selectedUser.id,
      version: selectedUser.version,
      values,
    })
  }

  return (
    <>
      <Card>
        <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <CardTitle>{t("users.title")}</CardTitle>
            <CardDescription>{t("users.description")}</CardDescription>
          </div>
          <Button onClick={openCreateDialog}>
            <UserPlus />
            {t("users.createUser")}
          </Button>
        </CardHeader>
        <CardContent>
          <UsersTable
            users={usersQuery.data?.users ?? []}
            pagination={usersQuery.data?.pagination}
            isLoading={usersQuery.isLoading}
            isFetching={usersQuery.isFetching}
            onPageChange={setPage}
            onEdit={openEditDialog}
            onResendInvitation={(user) => {
              resendInvitationMutation.mutate(user.id)
            }}
            onToggleStatus={openStatusDialog}
          />
        </CardContent>
      </Card>
      <UserFormDialog
        mode={formMode}
        open={isFormOpen}
        user={selectedUser}
        isPending={
          formMode === "create"
            ? createUserMutation.isPending
            : updateUserMutation.isPending
        }
        onOpenChange={(open) => {
          setIsFormOpen(open)
          if (!open) {
            setSelectedUser(null)
          }
        }}
        onSubmit={handleUserFormSubmit}
      />
      <StatusToggleDialog
        open={isStatusDialogOpen}
        user={selectedUser}
        isPending={updateStatusMutation.isPending}
        onOpenChange={(open) => {
          setIsStatusDialogOpen(open)
          if (!open) {
            setSelectedUser(null)
          }
        }}
        onConfirm={() => {
          if (!selectedUser) {
            return
          }

          updateStatusMutation.mutate({
            userID: selectedUser.id,
            version: selectedUser.version,
            status: selectedUser.status === "disabled" ? "active" : "disabled",
          })
        }}
      />
    </>
  )
}
