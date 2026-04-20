import { useEffect, useEffectEvent } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod/v3"

import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
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
import { UserQualifications } from "./user-qualifications"

export type UserFormValues = {
  email: string
  name: string
  is_admin: boolean
}

type UserFormDialogProps = {
  mode: "create" | "edit"
  open: boolean
  user?: User | null
  isPending: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (values: UserFormValues) => void
}

export function UserFormDialog({
  mode,
  open,
  user,
  isPending,
  onOpenChange,
  onSubmit,
}: UserFormDialogProps) {
  const { t, i18n } = useTranslation()

  const formSchema = z.object({
    name: z.string().trim().min(1, t("users.validation.nameRequired")),
    email: z
      .string()
      .trim()
      .min(1, t("users.validation.emailRequired"))
      .email(t("users.validation.emailInvalid")),
    is_admin: z.boolean(),
  })

  const {
    register,
    handleSubmit,
    reset,
    trigger,
    formState: { errors },
  } = useForm<UserFormValues>({
    resolver: zodResolver(formSchema),
    shouldUnregister: true,
    defaultValues: {
      email: user?.email ?? "",
      name: user?.name ?? "",
      is_admin: user?.is_admin ?? false,
    },
  })

  useEffect(() => {
    reset({
      email: user?.email ?? "",
      name: user?.name ?? "",
      is_admin: user?.is_admin ?? false,
    })
  }, [mode, open, reset, user])

  const revalidateVisibleErrors = useEffectEvent(() => {
    const errorFields = Object.keys(errors) as (keyof UserFormValues)[]
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
          <DialogTitle>
            {mode === "create"
              ? t("users.form.createTitle")
              : t("users.form.editTitle")}
          </DialogTitle>
          <DialogDescription>
            {mode === "create"
              ? t("users.form.createDescription")
              : t("users.form.editDescription")}
          </DialogDescription>
        </DialogHeader>
        <form
          className="grid gap-4"
          onSubmit={handleSubmit((values) => onSubmit(values))}
        >
          <div className="grid gap-2">
            <Label htmlFor="user-name">{t("users.name")}</Label>
            <Input id="user-name" {...register("name")} />
            {errors.name && (
              <p className="text-sm text-destructive">{errors.name.message}</p>
            )}
          </div>
          <div className="grid gap-2">
            <Label htmlFor="user-email">{t("users.email")}</Label>
            <Input id="user-email" type="email" {...register("email")} />
            {errors.email && (
              <p className="text-sm text-destructive">{errors.email.message}</p>
            )}
          </div>
          {mode === "create" && (
            <p className="text-sm text-muted-foreground">
              {t("users.form.invitationHint")}
            </p>
          )}
          <label className="flex items-center gap-3 text-sm font-medium">
            <Checkbox {...register("is_admin")} />
            <span>{t("users.isAdmin")}</span>
          </label>
          {mode === "edit" && user && (
            <UserQualifications
              key={`${user.id}-${open ? "open" : "closed"}`}
              open={open}
              user={user}
            />
          )}
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
                ? mode === "create"
                  ? t("users.form.submittingCreate")
                  : t("users.form.submittingEdit")
                : mode === "create"
                  ? t("users.form.submitCreate")
                  : t("users.form.submitEdit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
