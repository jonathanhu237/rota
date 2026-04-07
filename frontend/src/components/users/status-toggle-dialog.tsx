import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import type { User } from "@/lib/types"

type StatusToggleDialogProps = {
  open: boolean
  user?: User | null
  isPending: boolean
  onConfirm: () => void
  onOpenChange: (open: boolean) => void
}

export function StatusToggleDialog({
  open,
  user,
  isPending,
  onConfirm,
  onOpenChange,
}: StatusToggleDialogProps) {
  const { t } = useTranslation()

  const nextStatus = user?.status === "active" ? "disabled" : "active"

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {nextStatus === "disabled"
              ? t("users.statusDialog.disableTitle")
              : t("users.statusDialog.enableTitle")}
          </DialogTitle>
          <DialogDescription>
            {nextStatus === "disabled"
              ? t("users.statusDialog.disableDescription", {
                  name: user?.name ?? "",
                })
              : t("users.statusDialog.enableDescription", {
                  name: user?.name ?? "",
                })}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            {t("common.cancel")}
          </Button>
          <Button
            type="button"
            variant={nextStatus === "disabled" ? "destructive" : "default"}
            onClick={onConfirm}
            disabled={isPending}
          >
            {isPending
              ? t("users.statusDialog.submitting")
              : nextStatus === "disabled"
                ? t("users.statusDialog.confirmDisable")
                : t("users.statusDialog.confirmEnable")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
