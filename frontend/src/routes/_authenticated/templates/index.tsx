import { useEffect, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router"
import { CopyPlus } from "lucide-react"
import { useTranslation } from "react-i18next"

import { TemplateFormDialog } from "@/components/templates/template-form-dialog"
import { TemplatesTable } from "@/components/templates/templates-table"
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
  createTemplate,
  currentUserQueryOptions,
  templatesQueryOptions,
} from "@/lib/queries"
import type { Template } from "@/lib/types"

const pageSize = 10

export const Route = createFileRoute("/_authenticated/templates/")({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(currentUserQueryOptions)
    if (!user.is_admin) {
      throw redirect({ to: "/" })
    }
  },
  component: TemplatesPage,
})

function TemplatesPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { toast } = useToast()

  const [page, setPage] = useState(1)
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)

  const { data: currentUser } = useQuery(currentUserQueryOptions)
  const templatesQuery = useQuery(templatesQueryOptions(page, pageSize))

  useEffect(() => {
    if (currentUser && !currentUser.is_admin) {
      navigate({ to: "/", replace: true })
    }
  }, [currentUser, navigate])

  const createTemplateMutation = useMutation({
    mutationFn: createTemplate,
    onSuccess: async (template) => {
      setIsCreateDialogOpen(false)
      await queryClient.invalidateQueries({ queryKey: ["templates", "list"] })
      toast({
        variant: "default",
        description: t("templates.success.created"),
      })
      navigate({
        to: "/templates/$templateId",
        params: { templateId: String(template.id) },
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

  const openTemplate = (template: Template) => {
    navigate({
      to: "/templates/$templateId",
      params: { templateId: String(template.id) },
    })
  }

  return (
    <>
      <Card>
        <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <CardTitle>{t("templates.title")}</CardTitle>
            <CardDescription>{t("templates.description")}</CardDescription>
          </div>
          <Button onClick={() => setIsCreateDialogOpen(true)}>
            <CopyPlus />
            {t("templates.createTemplate")}
          </Button>
        </CardHeader>
        <CardContent>
          <TemplatesTable
            templates={templatesQuery.data?.templates ?? []}
            pagination={templatesQuery.data?.pagination}
            isLoading={templatesQuery.isLoading}
            isFetching={templatesQuery.isFetching}
            onOpen={openTemplate}
            onPageChange={setPage}
          />
        </CardContent>
      </Card>
      <TemplateFormDialog
        open={isCreateDialogOpen}
        isPending={createTemplateMutation.isPending}
        onOpenChange={setIsCreateDialogOpen}
        onSubmit={(values) => createTemplateMutation.mutate(values)}
      />
    </>
  )
}
