import { useEffect, useMemo } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"

import { updateOwnProfileMutation } from "@/components/settings/settings-api"
import {
  createProfileSchema,
  type ProfileFormValues,
} from "@/components/settings/settings-schemas"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useToast } from "@/components/ui/toast"
import type { User } from "@/lib/types"

export function ProfileForm({ user }: { user: User }) {
  const { t } = useTranslation()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const schema = useMemo(() => createProfileSchema(t), [t])
  const mutation = useMutation({
    ...updateOwnProfileMutation,
    onSuccess: (updatedUser) => {
      queryClient.setQueryData(["auth", "me"], updatedUser)
      toast({
        variant: "default",
        description: t("settings.profile.saved"),
      })
    },
  })

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<ProfileFormValues>({
    resolver: zodResolver(schema),
    defaultValues: { name: user.name },
  })

  useEffect(() => {
    reset({ name: user.name })
  }, [reset, user.name])

  return (
    <form
      className="grid gap-4"
      onSubmit={handleSubmit((values) =>
        mutation.mutate({ name: values.name.trim() }),
      )}
    >
      <div className="grid gap-2">
        <Label htmlFor="settings-name">{t("settings.profile.name")}</Label>
        <Input id="settings-name" {...register("name")} />
        {errors.name && (
          <p className="text-sm text-destructive">{errors.name.message}</p>
        )}
      </div>
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
