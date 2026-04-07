import { useEffect, useEffectEvent } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod/v3"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import type { User } from "@/lib/types"

type PasswordResetValues = {
  password: string
  confirmPassword: string
}

type PasswordResetDialogProps = {
  open: boolean
  user?: User | null
  isPending: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (values: PasswordResetValues) => void
}

export function PasswordResetDialog({
  open,
  user,
  isPending,
  onOpenChange,
  onSubmit,
}: PasswordResetDialogProps) {
  const { t, i18n } = useTranslation()

  const formSchema = z
    .object({
      password: z
        .string()
        .min(1, t("users.validation.passwordRequired"))
        .min(8, t("users.validation.passwordMin")),
      confirmPassword: z
        .string()
        .min(1, t("users.validation.confirmPasswordRequired")),
    })
    .refine((values) => values.password === values.confirmPassword, {
      path: ["confirmPassword"],
      message: t("users.validation.passwordMismatch"),
    })

  const {
    register,
    handleSubmit,
    reset,
    trigger,
    formState: { errors },
  } = useForm<PasswordResetValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      password: "",
      confirmPassword: "",
    },
  })

  useEffect(() => {
    reset({
      password: "",
      confirmPassword: "",
    })
  }, [open, reset, user?.id])

  const revalidateVisibleErrors = useEffectEvent(() => {
    const errorFields = Object.keys(errors) as (keyof PasswordResetValues)[]
    if (errorFields.length > 0) {
      void trigger(errorFields)
    }
  })

  useEffect(() => {
    revalidateVisibleErrors()
  }, [i18n.language])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("users.passwordDialog.title")}</DialogTitle>
          <DialogDescription>
            {t("users.passwordDialog.description", {
              name: user?.name ?? "",
            })}
          </DialogDescription>
        </DialogHeader>
        <form
          className="grid gap-4"
          onSubmit={handleSubmit((values) => onSubmit(values))}
        >
          <div className="grid gap-2">
            <Label htmlFor="reset-password">{t("users.password")}</Label>
            <Input
              id="reset-password"
              type="password"
              {...register("password")}
            />
            {errors.password && (
              <p className="text-sm text-destructive">
                {errors.password.message}
              </p>
            )}
          </div>
          <div className="grid gap-2">
            <Label htmlFor="reset-confirm-password">
              {t("users.confirmPassword")}
            </Label>
            <Input
              id="reset-confirm-password"
              type="password"
              {...register("confirmPassword")}
            />
            {errors.confirmPassword && (
              <p className="text-sm text-destructive">
                {errors.confirmPassword.message}
              </p>
            )}
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              {t("common.cancel")}
            </Button>
            <Button type="submit" disabled={isPending}>
              {isPending
                ? t("users.passwordDialog.submitting")
                : t("users.passwordDialog.submit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
