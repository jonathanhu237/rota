import { useMemo } from "react"
import { useMutation } from "@tanstack/react-query"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"

import { changeOwnPasswordMutation } from "@/components/settings/settings-api"
import {
  createPasswordSchema,
  type PasswordFormValues,
} from "@/components/settings/settings-schemas"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useToast } from "@/components/ui/toast"
import { getTranslatedApiError } from "@/lib/api-error"

export function PasswordForm() {
  const { t } = useTranslation()
  const { toast } = useToast()
  const schema = useMemo(() => createPasswordSchema(t), [t])
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<PasswordFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      current_password: "",
      new_password: "",
      confirm_password: "",
    },
  })

  const mutation = useMutation({
    ...changeOwnPasswordMutation,
    onSuccess: () => {
      reset()
      toast({
        variant: "default",
        description: t("settings.password.saved"),
      })
    },
    onError: (error) => {
      toast({
        variant: "destructive",
        description: getTranslatedApiError(
          t,
          error,
          "settings.password.errors",
          "settings.password.errors.default",
        ),
      })
    },
  })

  return (
    <form
      className="grid gap-4"
      onSubmit={handleSubmit((values) =>
        mutation.mutate({
          current_password: values.current_password,
          new_password: values.new_password,
        }),
      )}
    >
      <div className="grid gap-2">
        <Label htmlFor="settings-current-password">
          {t("settings.password.current")}
        </Label>
        <Input
          id="settings-current-password"
          type="password"
          {...register("current_password")}
        />
        {errors.current_password && (
          <p className="text-sm text-destructive">
            {errors.current_password.message}
          </p>
        )}
      </div>
      <div className="grid gap-2">
        <Label htmlFor="settings-new-password">
          {t("settings.password.new")}
        </Label>
        <Input
          id="settings-new-password"
          type="password"
          {...register("new_password")}
        />
        {errors.new_password && (
          <p className="text-sm text-destructive">
            {errors.new_password.message}
          </p>
        )}
      </div>
      <div className="grid gap-2">
        <Label htmlFor="settings-confirm-password">
          {t("settings.password.confirm")}
        </Label>
        <Input
          id="settings-confirm-password"
          type="password"
          {...register("confirm_password")}
        />
        {errors.confirm_password && (
          <p className="text-sm text-destructive">
            {errors.confirm_password.message}
          </p>
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
