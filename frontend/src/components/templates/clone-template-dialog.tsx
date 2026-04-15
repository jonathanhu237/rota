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
import type { Template } from "@/lib/types"

type CloneTemplateDialogProps = {
  open: boolean
  template?: Template | null
  isPending: boolean
  onConfirm: () => void
  onOpenChange: (open: boolean) => void
}

export function CloneTemplateDialog({
  open,
  template,
  isPending,
  onConfirm,
  onOpenChange,
}: CloneTemplateDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("templates.cloneDialog.title")}</DialogTitle>
          <DialogDescription>
            {t("templates.cloneDialog.description", {
              name: template?.name ?? "",
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
          <Button type="button" onClick={onConfirm} disabled={isPending}>
            {isPending
              ? t("templates.cloneDialog.submitting")
              : t("templates.cloneDialog.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
