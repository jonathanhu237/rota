import { useCallback, useEffect, useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation } from "@tanstack/react-query"
import { Mail, Send } from "lucide-react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"

import { requestEmailChangeMutation } from "@/components/settings/settings-api"
import {
  createEmailChangeSchema,
  type EmailChangeFormValues,
} from "@/components/settings/settings-schemas"
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
import { getApiErrorDetails, getTranslatedApiError } from "@/lib/api-error"
import type { User } from "@/lib/types"

export function EmailForm({ user }: { user: User }) {
  const { t } = useTranslation()
  const schema = useMemo(() => createEmailChangeSchema(t), [t])
  const [open, setOpen] = useState(false)
  const [sentEmail, setSentEmail] = useState<string | null>(null)
  const [formError, setFormError] = useState<string | null>(null)
  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors },
  } = useForm<EmailChangeFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      new_email: "",
      current_password: "",
    },
  })

  const mutation = useMutation({
    ...requestEmailChangeMutation,
    onSuccess: (_result, values) => {
      setSentEmail(values.new_email.trim())
      setFormError(null)
      reset({
        new_email: "",
        current_password: "",
      })
    },
    onError: (error) => {
      const code = getApiErrorDetails(error)?.code
      const message = getTranslatedApiError(
        t,
        error,
        "settings.email.errors",
        "settings.email.errors.default",
      )

      if (code === "EMAIL_ALREADY_EXISTS" || code === "INVALID_REQUEST") {
        setError("new_email", { message })
        return
      }

      if (code === "INVALID_CURRENT_PASSWORD") {
        setError("current_password", { message })
        return
      }

      setFormError(message)
    },
  })

  const resetDialogState = useCallback(() => {
    setSentEmail(null)
    setFormError(null)
    reset({
      new_email: "",
      current_password: "",
    })
  }, [reset])

  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      setOpen(nextOpen)
      if (!nextOpen) {
        resetDialogState()
      }
    },
    [resetDialogState],
  )

  useEffect(() => {
    if (!sentEmail) {
      return
    }

    const timeout = window.setTimeout(() => {
      handleOpenChange(false)
    }, 1600)

    return () => {
      window.clearTimeout(timeout)
    }
  }, [handleOpenChange, sentEmail])

  return (
    <div className="grid gap-4">
      <div className="grid gap-2">
        <Label htmlFor="settings-current-email">
          {t("settings.email.current")}
        </Label>
        <Input id="settings-current-email" value={user.email} readOnly disabled />
      </div>
      <div>
        <Button type="button" variant="outline" onClick={() => setOpen(true)}>
          <Mail data-icon="inline-start" />
          {t("settings.email.changeButton")}
        </Button>
      </div>

      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("settings.email.dialog.title")}</DialogTitle>
            <DialogDescription>
              {t("settings.email.dialog.description")}
            </DialogDescription>
          </DialogHeader>
          {sentEmail ? (
            <div className="grid gap-2 text-sm">
              <p className="font-medium">
                {t("settings.email.dialog.sent", { email: sentEmail })}
              </p>
              <p className="text-muted-foreground">
                {t("settings.email.dialog.sentDescription")}
              </p>
            </div>
          ) : (
            <form
              className="grid gap-4"
              noValidate
              onSubmit={handleSubmit((values) => {
                setFormError(null)
                mutation.mutate(values)
              })}
            >
              <div className="grid gap-2">
                <Label htmlFor="settings-new-email">
                  {t("settings.email.dialog.newEmail")}
                </Label>
                <Input
                  id="settings-new-email"
                  type="email"
                  placeholder={t("settings.email.dialog.newEmailPlaceholder")}
                  {...register("new_email")}
                />
                {errors.new_email && (
                  <p className="text-sm text-destructive">
                    {errors.new_email.message}
                  </p>
                )}
              </div>
              <div className="grid gap-2">
                <Label htmlFor="settings-email-current-password">
                  {t("settings.email.dialog.currentPassword")}
                </Label>
                <Input
                  id="settings-email-current-password"
                  type="password"
                  {...register("current_password")}
                />
                {errors.current_password && (
                  <p className="text-sm text-destructive">
                    {errors.current_password.message}
                  </p>
                )}
              </div>
              {formError && (
                <p className="text-sm text-destructive">{formError}</p>
              )}
              <DialogFooter>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => handleOpenChange(false)}
                >
                  {t("common.cancel")}
                </Button>
                <Button type="submit" disabled={mutation.isPending}>
                  <Send data-icon="inline-start" />
                  {mutation.isPending
                    ? t("settings.email.dialog.submitting")
                    : t("settings.email.dialog.submit")}
                </Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
