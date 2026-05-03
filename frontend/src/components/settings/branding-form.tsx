import { useEffect, useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"

import { updateBrandingMutation } from "@/components/settings/settings-api"
import {
  createBrandingSchema,
  type BrandingFormValues,
} from "@/components/settings/settings-schemas"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useToast } from "@/components/ui/toast"
import { getApiErrorDetails, getTranslatedApiError } from "@/lib/api-error"
import { brandingFallback, brandingQueryOptions } from "@/lib/queries"

export function BrandingForm() {
  const { t } = useTranslation()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const [submitError, setSubmitError] = useState<unknown>(null)
  const { data: branding = brandingFallback } = useQuery(brandingQueryOptions)
  const schema = useMemo(() => createBrandingSchema(t), [t])

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<BrandingFormValues>({
    resolver: zodResolver(schema),
    defaultValues: branding,
  })

  const mutation = useMutation({
    ...updateBrandingMutation,
    onMutate: () => {
      setSubmitError(null)
    },
    onSuccess: (updatedBranding) => {
      queryClient.setQueryData(["branding"], updatedBranding)
      reset(updatedBranding)
      toast({
        variant: "default",
        description: t("settings.branding.saved"),
      })
    },
    onError: (error) => {
      setSubmitError(error)
      if (getApiErrorDetails(error)?.code === "VERSION_CONFLICT") {
        void queryClient.invalidateQueries({ queryKey: ["branding"] })
      }
    },
  })

  useEffect(() => {
    reset(branding)
  }, [branding, reset])

  const errorMessage = submitError
    ? getTranslatedApiError(
        t,
        submitError,
        "settings.branding.errors",
        "settings.branding.errors.default",
      )
    : null

  return (
    <form
      className="grid gap-4"
      onSubmit={handleSubmit((values) =>
        mutation.mutate({
          product_name: values.product_name.trim(),
          organization_name: values.organization_name.trim(),
          version: values.version,
        }),
      )}
    >
      <input type="hidden" {...register("version", { valueAsNumber: true })} />
      <div className="grid max-w-sm gap-2">
        <Label htmlFor="settings-product-name">
          {t("settings.branding.productName")}
        </Label>
        <Input
          id="settings-product-name"
          aria-invalid={Boolean(errors.product_name)}
          {...register("product_name")}
        />
        {errors.product_name && (
          <p className="text-sm text-destructive">
            {errors.product_name.message}
          </p>
        )}
      </div>
      <div className="grid max-w-sm gap-2">
        <Label htmlFor="settings-organization-name">
          {t("settings.branding.organizationName")}
        </Label>
        <Input
          id="settings-organization-name"
          aria-invalid={Boolean(errors.organization_name)}
          placeholder={t("settings.branding.organizationNamePlaceholder")}
          {...register("organization_name")}
        />
        {errors.organization_name && (
          <p className="text-sm text-destructive">
            {errors.organization_name.message}
          </p>
        )}
      </div>
      {errorMessage && (
        <p className="text-sm text-destructive">{errorMessage}</p>
      )}
      <div>
        <Button type="submit" disabled={mutation.isPending}>
          {mutation.isPending
            ? t("settings.common.saving")
            : t("settings.common.save")}
        </Button>
      </div>
    </form>
  )
}
