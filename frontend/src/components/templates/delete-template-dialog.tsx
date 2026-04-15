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

type DeleteTemplateDialogProps = {
  open: boolean
  template?: Template | null
  isPending: boolean
  onConfirm: () => void
  onOpenChange: (open: boolean) => void
}

export function DeleteTemplateDialog({
  open,
  template,
  isPending,
  onConfirm,
  onOpenChange,
}: DeleteTemplateDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("templates.deleteDialog.title")}</DialogTitle>
          <DialogDescription>
            {t("templates.deleteDialog.description", {
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
          <Button
            type="button"
            variant="destructive"
            onClick={onConfirm}
            disabled={isPending}
          >
            {isPending
              ? t("templates.deleteDialog.submitting")
              : t("templates.deleteDialog.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
